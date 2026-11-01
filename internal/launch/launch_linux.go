package launch

import (
	"errors"
	"os/exec"
)

func folderCmd(dir string) *exec.Cmd { return exec.Command("xdg-open", dir) }

// terminalCmd — терминал по умолчанию (x-terminal-emulator в Debian и Ubuntu),
// иначе первый известный. Каталог — рабочий каталог процесса: его берут все.
func terminalCmd(dir string) (*exec.Cmd, error) {
	bin, ok := firstFound("x-terminal-emulator", "gnome-terminal", "konsole", "xfce4-terminal", "kitty", "alacritty", "xterm")
	if !ok {
		return nil, errors.New("не найден терминал: x-terminal-emulator, gnome-terminal, konsole, xterm")
	}
	cmd := exec.Command(bin)
	cmd.Dir = dir
	return cmd, nil
}
