//go:build windows

package main

import (
	"os/exec"
	"testing"
)

func TestPrepareChildProcessPreventsConsoleWindows(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "exit", "0")
	prepareChildProcess(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("prepareChildProcess did not configure Windows process attributes")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("child process window is not hidden")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("CreationFlags = %#x, want CREATE_NO_WINDOW", cmd.SysProcAttr.CreationFlags)
	}
}
