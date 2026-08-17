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

func TestSameManagedBackendRootAllowsHistoricalVersion(t *testing.T) {
	expected := `C:\Users\Tester\AppData\Local\ZCodeAntigravity\app-0.4.5-test\backend\cli-proxy-api.exe`
	actual := `C:\Users\Tester\AppData\Local\ZCodeAntigravity\app-0.4.4-test\backend\cli-proxy-api.exe`
	if !sameManagedBackendRoot(expected, actual) {
		t.Fatal("historical backend in the same managed root should be accepted")
	}
}

func TestSameManagedBackendRootRejectsUnrelatedProcess(t *testing.T) {
	expected := `C:\Users\Tester\AppData\Local\ZCodeAntigravity\app-0.4.5-test\backend\cli-proxy-api.exe`
	cases := []string{
		`C:\Windows\System32\cli-proxy-api.exe`,
		`C:\Users\Tester\AppData\Local\ZCodeAntigravity\app-evil\backend\not-the-backend.exe`,
		`D:\ZCodeAntigravity\app-0.4.4-test\backend\cli-proxy-api.exe`,
	}
	for _, actual := range cases {
		if sameManagedBackendRoot(expected, actual) {
			t.Fatalf("unrelated process path was accepted: %s", actual)
		}
	}
}
