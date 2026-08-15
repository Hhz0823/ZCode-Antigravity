//go:build windows

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
	replaceFileWriteThrough = 0x2
)

var (
	kernel32FileProcedures = syscall.NewLazyDLL("kernel32.dll")
	moveFileExWProcedure   = kernel32FileProcedures.NewProc("MoveFileExW")
	replaceFileWProcedure  = kernel32FileProcedures.NewProc("ReplaceFileW")
)

func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".zcode-antigravity-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	from, err := syscall.UTF16PtrFromString(tmpName)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var result uintptr
	var callErr error
	if _, statErr := os.Stat(path); statErr == nil {
		result, _, callErr = replaceFileWProcedure.Call(
			uintptr(unsafe.Pointer(to)),
			uintptr(unsafe.Pointer(from)),
			0,
			uintptr(replaceFileWriteThrough),
			0,
			0,
		)
	} else if os.IsNotExist(statErr) {
		result, _, callErr = moveFileExWProcedure.Call(
			uintptr(unsafe.Pointer(from)),
			uintptr(unsafe.Pointer(to)),
			uintptr(moveFileReplaceExisting|moveFileWriteThrough),
		)
	} else {
		return statErr
	}
	if result == 0 {
		return callErr
	}
	return nil
}
