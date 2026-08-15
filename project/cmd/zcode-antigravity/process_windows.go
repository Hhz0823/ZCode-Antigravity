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
)

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
	if !strings.EqualFold(filepath.Clean(actualAbs), filepath.Clean(expectedAbs)) {
		return fmt.Errorf("PID %d 指向 %s，不是本程序的 %s；已拒绝停止", pid, actualAbs, expectedAbs)
	}
	cmd := exec.Command("taskkill.exe", "/PID", strconv.Itoa(pid), "/T", "/F")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("taskkill 失败: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func windowsProcessPath(pid int) (string, error) {
	script := fmt.Sprintf("$p = Get-CimInstance Win32_Process -Filter 'ProcessId = %d' -ErrorAction Stop; if ($null -eq $p -or [string]::IsNullOrWhiteSpace($p.ExecutablePath)) { exit 3 }; [Console]::Out.Write($p.ExecutablePath)", pid)
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
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
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
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
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
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
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	name := strings.TrimSpace(string(output))
	return name, err == nil && name != ""
}
