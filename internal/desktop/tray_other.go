//go:build !linux

package desktop

// trayAvailable: в Windows и macOS область уведомлений есть всегда.
func trayAvailable() bool { return true }
