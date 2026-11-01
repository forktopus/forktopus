//go:build !linux

package main

// trayAvailable: в Windows и macOS область уведомлений есть всегда.
func trayAvailable() bool { return true }
