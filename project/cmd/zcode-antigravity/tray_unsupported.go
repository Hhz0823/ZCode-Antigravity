//go:build !windows && !darwin

package main

func platformTraySupported() bool { return false }

func runPlatformTray(_ <-chan struct{}, _ trayHooks) error { return nil }
