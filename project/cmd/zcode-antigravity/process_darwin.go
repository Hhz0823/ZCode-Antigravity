//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func prepareChildProcess(_ *exec.Cmd) {}

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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
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
	actualPath, err := darwinProcessPath(pid)
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
	if evaluated, evalErr := filepath.EvalSymlinks(expectedAbs); evalErr == nil {
		expectedAbs = evaluated
	}
	if evaluated, evalErr := filepath.EvalSymlinks(actualAbs); evalErr == nil {
		actualAbs = evaluated
	}
	if filepath.Clean(actualAbs) != filepath.Clean(expectedAbs) {
		return fmt.Errorf("PID %d 指向 %s，不是本程序的 %s；已拒绝停止", pid, actualAbs, expectedAbs)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	return nil
}

func darwinProcessPath(pid int) (string, error) {
	cmd := exec.Command("/usr/sbin/lsof", "-a", "-p", strconv.Itoa(pid), "-d", "txt", "-Fn")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "n/") {
			return strings.TrimSpace(strings.TrimPrefix(line, "n")), nil
		}
	}
	return "", fmt.Errorf("进程不存在或可执行文件路径不可见")
}

func describePortOwner(port int) string {
	cmd := exec.Command("/usr/sbin/lsof", "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-Fpct")
	output, err := cmd.Output()
	if err != nil {
		return "占用进程未知"
	}
	value := strings.Join(strings.Fields(string(output)), " ")
	if value == "" {
		return "占用进程未知"
	}
	return value
}

func isZCodeRunning() bool {
	return exec.Command("/usr/bin/pgrep", "-x", "ZCode").Run() == nil
}

func preferredBrowserExecutable() string {
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
	}
	for _, candidate := range candidates {
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
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func launchDashboardWindow(rawURL string) error {
	if browser := preferredBrowserExecutable(); browser != "" {
		cmd := exec.Command(browser, "--app="+rawURL, "--new-window")
		if err := cmd.Start(); err == nil {
			return cmd.Process.Release()
		}
	}
	cmd := exec.Command("/usr/bin/open", rawURL)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func openZCodeApplication() error {
	cmd := exec.Command("/usr/bin/open", "-a", "ZCode")
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("未找到 ZCode.app，请先安装 ZCode: %w", err)
	}
	return nil
}

func detectTunAdapter() (string, bool) {
	output, err := exec.Command("/sbin/ifconfig").Output()
	if err != nil {
		return "", false
	}
	return activeDarwinTUN(string(output))
}

func activeDarwinTUN(output string) (string, bool) {
	current := ""
	up := false
	flush := func() (string, bool) {
		if strings.HasPrefix(current, "utun") && up {
			return current, true
		}
		return "", false
	}
	for _, line := range strings.Split(output, "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		if name, ok := flush(); ok {
			return name, true
		}
		colon := strings.IndexByte(line, ':')
		if colon < 1 {
			current = ""
			up = false
			continue
		}
		current = strings.TrimSpace(line[:colon])
		flags := line[colon+1:]
		up = strings.Contains(flags, "<UP") || strings.Contains(flags, ",UP")
	}
	return flush()
}
