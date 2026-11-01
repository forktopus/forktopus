package launch

import (
	"os/exec"
	"syscall"
)

func folderCmd(dir string) *exec.Cmd { return exec.Command("explorer.exe", dir) }

// terminalCmd — Windows Terminal, иначе cmd в новой консоли.
func terminalCmd(dir string) (*exec.Cmd, error) {
	if wt, ok := firstFound("wt.exe"); ok {
		return exec.Command(wt, "-d", dir), nil
	}
	cmd := exec.Command("cmd.exe")
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010} // CREATE_NEW_CONSOLE
	return cmd, nil
}
