//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestActiveDarwinTUN(t *testing.T) {
	input := `lo0: flags=8049<UP,LOOPBACK,RUNNING,MULTICAST> mtu 16384
	inet 127.0.0.1 netmask 0xff000000
utun4: flags=8051<UP,POINTOPOINT,RUNNING,MULTICAST> mtu 1380
	inet6 fe80::1%utun4 prefixlen 64 scopeid 0x1a
`
	name, ok := activeDarwinTUN(input)
	if !ok || name != "utun4" {
		t.Fatalf("activeDarwinTUN = %q, %v; want utun4, true", name, ok)
	}
}

func TestActiveDarwinTUNRejectsDownInterface(t *testing.T) {
	name, ok := activeDarwinTUN("utun2: flags=8050<POINTOPOINT,RUNNING,MULTICAST> mtu 1380\n")
	if ok || name != "" {
		t.Fatalf("activeDarwinTUN = %q, %v; want empty, false", name, ok)
	}
}

func TestDarwinProcessPathResolvesCurrentExecutable(t *testing.T) {
	actual, errPath := darwinProcessPath(os.Getpid())
	if errPath != nil {
		t.Fatalf("darwinProcessPath: %v", errPath)
	}
	expected, errExecutable := os.Executable()
	if errExecutable != nil {
		t.Fatal(errExecutable)
	}
	if evaluated, errEval := filepath.EvalSymlinks(actual); errEval == nil {
		actual = evaluated
	}
	if evaluated, errEval := filepath.EvalSymlinks(expected); errEval == nil {
		expected = evaluated
	}
	if filepath.Clean(actual) != filepath.Clean(expected) {
		t.Fatalf("darwinProcessPath = %q, want %q", actual, expected)
	}
}
