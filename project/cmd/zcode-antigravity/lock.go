package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

func (a *app) acquireRunLock() (func(), error) {
	pidText := strconv.Itoa(os.Getpid())
	for attempt := 0; attempt < 2; attempt++ {
		file, errOpen := os.OpenFile(a.paths.Lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errOpen == nil {
			if _, errWrite := file.WriteString(pidText + "\n"); errWrite != nil {
				_ = file.Close()
				_ = os.Remove(a.paths.Lock)
				return nil, errWrite
			}
			if errSync := file.Sync(); errSync != nil {
				_ = file.Close()
				_ = os.Remove(a.paths.Lock)
				return nil, errSync
			}
			_ = file.Close()
			return func() {
				raw, errRead := os.ReadFile(a.paths.Lock)
				if errRead == nil && strings.TrimSpace(string(raw)) == pidText {
					_ = os.Remove(a.paths.Lock)
				}
			}, nil
		}
		if !errors.Is(errOpen, fs.ErrExist) {
			return nil, fmt.Errorf("创建单实例锁: %w", errOpen)
		}
		raw, errRead := os.ReadFile(a.paths.Lock)
		if errRead != nil {
			return nil, fmt.Errorf("读取单实例锁: %w", errRead)
		}
		ownerPID, errPID := strconv.Atoi(strings.TrimSpace(string(raw)))
		if errPID == nil && ownerPID > 0 && processExists(ownerPID) {
			return nil, fmt.Errorf("另一个 ZCode Antigravity 管理器正在运行 (PID %d)", ownerPID)
		}
		if errRemove := os.Remove(a.paths.Lock); errRemove != nil && !errors.Is(errRemove, fs.ErrNotExist) {
			return nil, fmt.Errorf("清理失效单实例锁: %w", errRemove)
		}
	}
	return nil, fmt.Errorf("无法获取单实例锁")
}
