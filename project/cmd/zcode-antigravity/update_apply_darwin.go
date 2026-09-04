//go:build darwin

package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxUpdateArchiveFiles = 5000
	maxExpandedUpdateSize = 1 << 30
)

func runUpdateHelper(args []string) error {
	if executable, err := os.Executable(); err == nil && strings.HasPrefix(filepath.Base(executable), "ZCode-Antigravity-Updater-") {
		defer os.Remove(executable)
	}
	flags := flag.NewFlagSet("apply-update", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	archivePath := flags.String("archive", "", "verified update archive")
	targetPath := flags.String("target", "", "installed application")
	expectedVersion := flags.String("version", "", "expected version")
	expectedDigest := flags.String("sha256", "", "expected archive digest")
	parentPID := flags.Int("parent-pid", 0, "application process to wait for")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return fmt.Errorf("更新辅助程序参数无效")
	}
	versionValue, err := normalizedReleaseVersion(*expectedVersion)
	if err != nil {
		return err
	}
	wantedAsset, _ := expectedUpdateAsset("darwin", versionValue)
	archive, err := filepath.Abs(strings.TrimSpace(*archivePath))
	if err != nil || filepath.Base(archive) != wantedAsset {
		return fmt.Errorf("更新压缩包名称与版本不匹配")
	}
	target, err := filepath.Abs(strings.TrimSpace(*targetPath))
	if err != nil || filepath.Base(target) != "ZCode Antigravity.app" {
		return fmt.Errorf("只允许更新 ZCode Antigravity.app")
	}
	if info, errStat := os.Stat(target); errStat != nil || !info.IsDir() {
		return fmt.Errorf("当前 App 不存在")
	}
	digest, err := parseSHA256Digest("sha256:" + strings.TrimSpace(*expectedDigest))
	if err != nil {
		return err
	}
	info, err := os.Stat(archive)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > updateMaxAssetBytes {
		return fmt.Errorf("更新压缩包无效")
	}
	if matches, errHash := fileMatchesUpdate(archive, digest, info.Size()); errHash != nil || !matches {
		return fmt.Errorf("更新压缩包 SHA-256 校验失败")
	}
	if *parentPID > 0 {
		deadline := time.Now().Add(60 * time.Second)
		for processExists(*parentPID) && time.Now().Before(deadline) {
			time.Sleep(200 * time.Millisecond)
		}
		if processExists(*parentPID) {
			return fmt.Errorf("等待旧版本退出超时")
		}
	}
	if isZCodeRunning() {
		return fmt.Errorf("请先完全退出 ZCode，再安装更新")
	}
	if err := stopGatewayBeforeMacUpdate(); err != nil {
		return err
	}
	extractRoot, err := os.MkdirTemp(filepath.Dir(archive), ".macos-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(extractRoot)
	if err := extractMacUpdateArchive(archive, extractRoot); err != nil {
		return err
	}
	appSource, err := findSingleUpdateApp(extractRoot)
	if err != nil {
		return err
	}
	if err := verifyMacUpdateApp(appSource, versionValue); err != nil {
		return err
	}
	staging := filepath.Join(filepath.Dir(target), fmt.Sprintf(".ZCode Antigravity %s update-%d.app", versionValue, os.Getpid()))
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		return fmt.Errorf("更新暂存 App 已存在，请重试")
	}
	defer os.RemoveAll(staging)
	if output, err := exec.Command("/usr/bin/ditto", appSource, staging).CombinedOutput(); err != nil {
		return fmt.Errorf("准备新 App: %w (%s)", err, compactText(output, 300))
	}
	if err := verifyMacUpdateApp(staging, versionValue); err != nil {
		return err
	}
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	backupDir := filepath.Join(configRoot, "ZCodeAntigravity", "App Backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return err
	}
	currentVersion := macBundleShortVersion(target)
	backup := filepath.Join(backupDir, fmt.Sprintf("ZCode Antigravity %s auto-update %s.app-backup", currentVersion, time.Now().Format("20060102-150405")))
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		return fmt.Errorf("更新备份路径已存在")
	}
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("备份当前 App: %w", err)
	}
	if err := os.Rename(staging, target); err != nil {
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return fmt.Errorf("安装新 App 失败: %v；恢复旧 App 也失败: %v", err, restoreErr)
		}
		return fmt.Errorf("安装新 App 失败，已恢复旧版本: %w", err)
	}
	if err := verifyMacUpdateApp(target, versionValue); err != nil {
		failed := filepath.Join(extractRoot, "failed-new-app")
		_ = os.Rename(target, failed)
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return fmt.Errorf("新 App 校验失败: %v；恢复旧 App 也失败: %v", err, restoreErr)
		}
		return fmt.Errorf("新 App 校验失败，已恢复旧版本: %w", err)
	}
	postUpdateMarker := filepath.Join(configRoot, "ZCodeAntigravity", "post-update")
	if err := os.WriteFile(postUpdateMarker, []byte(versionValue+"\n"), 0o600); err != nil {
		fmt.Printf("警告：无法写入更新后同步标记: %v\n", err)
	}
	command := exec.Command("/usr/bin/open", "-n", "-a", target, "--args", "--post-update")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("更新已安装，但无法自动重启: %w (%s)", err, compactText(output, 300))
	}
	fmt.Printf("自动更新完成: %s；旧版备份: %s\n", versionValue, backup)
	return nil
}

func stopGatewayBeforeMacUpdate() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	app, err := newApp(filepath.Dir(executable))
	if err != nil {
		return fmt.Errorf("更新前读取本机状态: %w", err)
	}
	current, err := app.loadState()
	if err != nil || current.PID <= 0 || current.Port <= 0 || app.probeGateway(current.Port) != nil {
		return nil
	}
	if err := app.stop(); err != nil {
		return fmt.Errorf("更新前安全停止旧网关: %w", err)
	}
	return nil
}

func extractMacUpdateArchive(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("打开更新压缩包: %w", err)
	}
	defer reader.Close()
	if len(reader.File) == 0 || len(reader.File) > maxUpdateArchiveFiles {
		return fmt.Errorf("更新压缩包文件数异常")
	}
	root, _ := filepath.Abs(destination)
	var expanded uint64
	for _, entry := range reader.File {
		clean := filepath.Clean(filepath.FromSlash(entry.Name))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("更新压缩包包含不安全路径")
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("更新压缩包不允许符号链接")
		}
		expanded += entry.UncompressedSize64
		if expanded > maxExpandedUpdateSize {
			return fmt.Errorf("更新压缩包解压后过大")
		}
		target := filepath.Join(root, clean)
		relative, err := filepath.Rel(root, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("更新压缩包路径越界")
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		mode := entry.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeOutputErr := output.Close()
		closeInputErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutputErr != nil {
			return closeOutputErr
		}
		if closeInputErr != nil {
			return closeInputErr
		}
	}
	return nil
}

func findSingleUpdateApp(root string) (string, error) {
	apps := make([]string, 0, 1)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && entry.Name() == "ZCode Antigravity.app" {
			apps = append(apps, path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(apps) != 1 {
		return "", fmt.Errorf("更新压缩包必须只包含一个 ZCode Antigravity.app")
	}
	return apps[0], nil
}

func verifyMacUpdateApp(appPath, expectedVersion string) error {
	shortVersion := strings.SplitN(expectedVersion, "-", 2)[0]
	if actual := macBundleShortVersion(appPath); actual != shortVersion {
		return fmt.Errorf("更新 App 版本为 %q，期望 %q", actual, shortVersion)
	}
	for _, relative := range []string{
		"Contents/MacOS/ZCode-Antigravity",
		"Contents/MacOS/ZCode-Antigravity-Core",
		"Contents/MacOS/backend/cli-proxy-api",
	} {
		if info, err := os.Stat(filepath.Join(appPath, relative)); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("更新 App 缺少可执行文件 %s", relative)
		}
	}
	core := filepath.Join(appPath, "Contents/MacOS/ZCode-Antigravity-Core")
	if output, err := exec.Command(core, "version").CombinedOutput(); err != nil || !strings.Contains(string(output), expectedVersion) {
		return fmt.Errorf("更新 App 核心版本校验失败")
	}
	if output, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", appPath).CombinedOutput(); err != nil {
		return fmt.Errorf("更新 App 签名校验失败: %s", compactText(output, 300))
	}
	return nil
}

func macBundleShortVersion(appPath string) string {
	output, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-", filepath.Join(appPath, "Contents/Info.plist")).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}
