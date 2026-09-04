//go:build darwin

package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractMacUpdateArchiveRejectsTraversal(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "update.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../escaped")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("no"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	err = extractMacUpdateArchive(archive, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "不安全路径") {
		t.Fatalf("traversal archive error = %v", err)
	}
}
