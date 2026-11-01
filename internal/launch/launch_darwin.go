package launch

import "os/exec"

func folderCmd(dir string) *exec.Cmd { return exec.Command("open", dir) }

func terminalCmd(dir string) (*exec.Cmd, error) {
	return exec.Command("open", "-a", "Terminal", dir), nil
}
