//go:build unix

package runner

import (
	"os/exec"
	"syscall"
)

// detach запускает процесс в новой сессии без управляющего терминала: ssh и
// git не смогут спросить пароль или passphrase через /dev/tty и завершатся
// ошибкой сразу, а не по таймауту (idea 04-GIT-AND-PROVIDERS §6).
//
// Процесс становится лидером своей группы, поэтому при отмене и таймауте
// завершается вся группа: потомки (ssh от git fetch, helper'ы) не переживают
// отмену и закрытие приложения (11.4).
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
