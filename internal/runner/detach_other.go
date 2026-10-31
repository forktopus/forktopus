//go:build !unix

package runner

import "os/exec"

// detach: на Windows консольный ввод и так недоступен процессу без консоли.
func detach(*exec.Cmd) {}
