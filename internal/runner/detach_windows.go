//go:build windows

package runner

import (
	"os/exec"
	"syscall"
)

// createNoWindow — CREATE_NO_WINDOW из WinAPI (в syscall нет константы).
const createNoWindow = 0x08000000

// detach: приложение собрано без консоли (-H windowsgui), и консольный
// git.exe иначе открывал бы своё окно на каждый вызов. Без консоли ssh и git
// тоже не могут спросить пароль в терминале.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
