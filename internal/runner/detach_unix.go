//go:build unix

package runner

import (
	"os/exec"
	"syscall"
)

// detach запускает процесс в новой сессии без управляющего терминала: ssh и
// git не смогут спросить пароль или passphrase через /dev/tty и завершатся
// ошибкой сразу, а не по таймауту (idea 04-GIT-AND-PROVIDERS §6).
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
