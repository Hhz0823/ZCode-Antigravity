//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
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
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("无法验证 PID %d: %w", pid, err)
	}
	actual := strings.TrimSpace(string(output))
	expectedBase := filepath.Base(expectedPath)
	if actual == "" || !strings.Contains(actual, expectedBase) {
		return fmt.Errorf("PID %d 不是预期的 %s；已拒绝停止", pid, expectedBase)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	time.Sleep(250 * time.Millisecond)
	return nil
}

func describePortOwner(port int) string {
	cmd := exec.Command("lsof", "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-Fpct")
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

func isZCodeRunning() bool { return false }

func preferredBrowserExecutable() string { return "" }

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
	cmd := exec.Command("xdg-open", rawURL)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func openZCodeApplication() error {
	return fmt.Errorf("自动打开 ZCode 仅支持 Windows")
}

func detectTunAdapter() (string, bool) {
	return "", false
}
