//go:build !darwin

package main

import "fmt"

func runUpdateHelper(_ []string) error {
	return fmt.Errorf("当前平台不使用内置更新辅助程序")
}
