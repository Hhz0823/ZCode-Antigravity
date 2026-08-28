package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteWindowsPowerShellScriptAddsUTF8BOM(t *testing.T) {
	wantScript := []byte("throw '拒绝把安装根目录设置为磁盘根目录。'\r\n")
	path := filepath.Join(t.TempDir(), "install.ps1")
	if err := writeWindowsPowerShellScript(path, wantScript); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := append(append([]byte{}, windowsPowerShellUTF8BOM...), wantScript...)
	if !bytes.Equal(got, want) {
		t.Fatalf("written script did not contain exactly one UTF-8 BOM")
	}
}

func TestWriteWindowsPowerShellScriptKeepsSingleUTF8BOM(t *testing.T) {
	want := append(append([]byte{}, windowsPowerShellUTF8BOM...), []byte("Write-Output 'ok'\r\n")...)
	path := filepath.Join(t.TempDir(), "install.ps1")
	if err := writeWindowsPowerShellScript(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("existing UTF-8 BOM was duplicated or changed")
	}
}

func TestWriteWindowsPowerShellScriptRejectsInvalidUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install.ps1")
	if err := writeWindowsPowerShellScript(path, []byte{0xff}); err == nil {
		t.Fatal("expected invalid UTF-8 to be rejected")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid script should not be written; stat error = %v", err)
	}
}

func TestInstallScriptsDefaultToDirectNetworking(t *testing.T) {
	for _, path := range []string{"Install-From-Package.ps1", "../OneClick-Installer.ps1"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(raw, []byte("$proxyURL = ''")) || bytes.Contains(raw, []byte("Test-V2rayNTunUp")) {
			t.Fatalf("%s still forces a TUN/proxy dependency", path)
		}
	}
}
