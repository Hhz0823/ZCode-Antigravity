package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func rotateManagedFile(path string, maxBytes int64, keep int) error {
	if maxBytes <= 0 || keep <= 0 {
		return nil
	}
	info, errStat := os.Stat(path)
	if errors.Is(errStat, fs.ErrNotExist) {
		return nil
	}
	if errStat != nil {
		return errStat
	}
	if info.Size() < maxBytes {
		return nil
	}
	oldest := fmt.Sprintf("%s.%d", path, keep)
	if errRemove := os.Remove(oldest); errRemove != nil && !errors.Is(errRemove, fs.ErrNotExist) {
		return errRemove
	}
	for index := keep - 1; index >= 1; index-- {
		from := fmt.Sprintf("%s.%d", path, index)
		to := fmt.Sprintf("%s.%d", path, index+1)
		if errRename := os.Rename(from, to); errRename != nil && !errors.Is(errRename, fs.ErrNotExist) {
			return errRename
		}
	}
	return os.Rename(path, path+".1")
}

func pruneManagedBackups(dir string, keep int) error {
	if keep <= 0 {
		return nil
	}
	entries, errRead := os.ReadDir(dir)
	if errRead != nil {
		return errRead
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !isManagedBackupName(entry.Name()) {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	if len(paths) <= keep {
		return nil
	}
	for _, path := range paths[:len(paths)-keep] {
		if errRemove := os.Remove(path); errRemove != nil {
			return errRemove
		}
	}
	return nil
}

func isManagedBackupName(name string) bool {
	matched, errMatch := filepath.Match("config-*-before-*.json", name)
	return errMatch == nil && matched
}
