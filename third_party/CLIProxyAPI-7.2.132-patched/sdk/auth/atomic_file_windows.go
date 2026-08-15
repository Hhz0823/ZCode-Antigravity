//go:build windows

package auth

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	authKernel32     = syscall.NewLazyDLL("kernel32.dll")
	authReplaceFileW = authKernel32.NewProc("ReplaceFileW")
	authMoveFileExW  = authKernel32.NewProc("MoveFileExW")
)

const (
	authReplaceWriteThrough = 0x00000001
	authMoveReplaceExisting = 0x00000001
	authMoveWriteThrough    = 0x00000008
)

func writeAuthFileAtomic(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".auth-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err = tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if _, statErr := os.Stat(path); statErr == nil {
		err = replaceAuthFile(path, tmpPath)
	} else if errors.Is(statErr, fs.ErrNotExist) {
		err = moveAuthFile(tmpPath, path, false)
	} else {
		return statErr
	}
	if err != nil {
		return err
	}
	cleanup = false
	return nil
}

func replaceAuthFile(target, replacement string) error {
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	replacementPtr, err := syscall.UTF16PtrFromString(replacement)
	if err != nil {
		return err
	}
	r1, _, callErr := authReplaceFileW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(unsafe.Pointer(replacementPtr)),
		0,
		authReplaceWriteThrough,
		0,
		0,
	)
	if r1 == 0 {
		return fmt.Errorf("ReplaceFileW: %w", callErr)
	}
	return nil
}

func moveAuthFile(source, target string, replace bool) error {
	sourcePtr, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	flags := uintptr(authMoveWriteThrough)
	if replace {
		flags |= authMoveReplaceExisting
	}
	r1, _, callErr := authMoveFileExW.Call(
		uintptr(unsafe.Pointer(sourcePtr)),
		uintptr(unsafe.Pointer(targetPtr)),
		flags,
	)
	if r1 == 0 {
		return fmt.Errorf("MoveFileExW: %w", callErr)
	}
	return nil
}
