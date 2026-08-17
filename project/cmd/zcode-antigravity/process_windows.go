//go:build windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
	createNoWindow        = 0x08000000
)

func hiddenWindowsProcessAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

func prepareChildProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = hiddenWindowsProcessAttributes()
}

func launchDetached(binary string, args []string, workDir, logPath string) (int, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()
	cmd := exec.Command(binary, args...)
	cmd.Dir = workDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | detachedProcess,
		HideWindow:    true,
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		return 0, err
	}
	return pid, nil
}

func terminateOwnedProcess(pid int, expectedPath string) error {
	actualPath, err := windowsProcessPath(pid)
	if err != nil {
		return fmt.Errorf("无法验证 PID %d 的程序路径，为避免误杀已拒绝停止: %w", pid, err)
	}
	expectedAbs, err := filepath.Abs(expectedPath)
	if err != nil {
		return err
	}
	actualAbs, err := filepath.Abs(actualPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(actualAbs), filepath.Clean(expectedAbs)) &&
		!sameManagedBackendRoot(expectedAbs, actualAbs) {
		return fmt.Errorf("PID %d 指向 %s，不是本程序的 %s；已拒绝停止", pid, actualAbs, expectedAbs)
	}
	cmd := exec.Command("taskkill.exe", "/PID", strconv.Itoa(pid), "/T", "/F")
	prepareChildProcess(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("taskkill 失败: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// sameManagedBackendRoot accepts a running backend from an older installed app
// directory while still refusing to terminate arbitrary processes. This is
// needed when a newer manager reused a healthy older gateway and an old release
// recorded its own backend path in state.json.
func sameManagedBackendRoot(expectedPath, actualPath string) bool {
	expectedRoot, expectedOK := managedBackendRoot(expectedPath)
	actualRoot, actualOK := managedBackendRoot(actualPath)
	return expectedOK && actualOK && strings.EqualFold(expectedRoot, actualRoot)
}

func managedBackendRoot(path string) (string, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	backendDir := filepath.Dir(filepath.Clean(abs))
	appDir := filepath.Dir(backendDir)
	root := filepath.Dir(appDir)
	if !strings.EqualFold(filepath.Base(abs), "cli-proxy-api.exe") ||
		!strings.EqualFold(filepath.Base(backendDir), "backend") ||
		!strings.HasPrefix(strings.ToLower(filepath.Base(appDir)), "app-") ||
		len(filepath.Base(appDir)) <= len("app-") {
		return "", false
	}
	return filepath.Clean(root), true
}

func windowsProcessPath(pid int) (string, error) {
	script := fmt.Sprintf("$p = Get-CimInstance Win32_Process -Filter 'ProcessId = %d' -ErrorAction Stop; if ($null -eq $p -or [string]::IsNullOrWhiteSpace($p.ExecutablePath)) { exit 3 }; [Console]::Out.Write($p.ExecutablePath)", pid)
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
	prepareChildProcess(cmd)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", fmt.Errorf("进程不存在或路径不可见")
	}
	return path, nil
}

func describePortOwner(port int) string {
	cmd := exec.Command("netstat.exe", "-ano", "-p", "tcp")
	prepareChildProcess(cmd)
	output, err := cmd.Output()
	if err != nil {
		return "占用进程未知"
	}
	suffix := ":" + strconv.Itoa(port)
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		fields := strings.Fields(string(line))
		if len(fields) < 5 || !strings.HasSuffix(fields[1], suffix) {
			continue
		}
		pid := fields[len(fields)-1]
		name := windowsTaskName(pid)
		if name != "" {
			return name + ", PID " + pid
		}
		return "PID " + pid
	}
	return "占用进程未知"
}

func windowsTaskName(pid string) string {
	cmd := exec.Command("tasklist.exe", "/FI", "PID eq "+pid, "/FO", "CSV", "/NH")
	prepareChildProcess(cmd)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(output))
	if !strings.HasPrefix(line, "\"") {
		return ""
	}
	line = strings.TrimPrefix(line, "\"")
	if index := strings.Index(line, "\""); index >= 0 {
		return line[:index]
	}
	return ""
}

func isZCodeRunning() bool {
	cmd := exec.Command("tasklist.exe", "/FI", "IMAGENAME eq ZCode.exe", "/FO", "CSV", "/NH")
	prepareChildProcess(cmd)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(output)), "\"zcode.exe\"")
}

func preferredBrowserExecutable() string {
	candidates := []string{
		filepath.Join(os.Getenv("PROGRAMFILES"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := windowsProcessPath(pid)
	return err == nil
}

func launchDashboardWindow(rawURL string) error {
	browser := preferredBrowserExecutable()
	var cmd *exec.Cmd
	if browser != "" {
		cmd = exec.Command(browser, "--app="+rawURL, "--new-window")
	} else {
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", rawURL)
	}
	prepareChildProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func openZCodeApplication() error {
	candidates := []string{
		filepath.Join(os.Getenv("PROGRAMFILES"), "ZCode", "ZCode.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "ZCode", "ZCode.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ZCode", "ZCode.exe"),
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if info, err := os.Stat(candidate); err != nil || info.IsDir() {
			continue
		}
		cmd := exec.Command(candidate)
		prepareChildProcess(cmd)
		if err := cmd.Start(); err != nil {
			return err
		}
		return cmd.Process.Release()
	}
	return fmt.Errorf("未找到 ZCode.exe，请先安装 ZCode")
}

func detectTunAdapter() (string, bool) {
	script := `$pattern='(?i)tun|wintun|v2ray|xray|sing.?box'; $adapter=Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq 'Up' -and (($_.Name+' '+$_.InterfaceDescription) -match $pattern) } | Select-Object -First 1; if($null -eq $adapter){exit 3}; [Console]::Out.Write($adapter.Name)`
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
	prepareChildProcess(cmd)
	output, err := cmd.Output()
	name := strings.TrimSpace(string(output))
	return name, err == nil && name != ""
}
