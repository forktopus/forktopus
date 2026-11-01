package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/forktopus/forktopus/internal/app"
)

// NewAutostart — автозапуск по XDG: forktopus.desktop в ~/.config/autostart.
// Не Wails: он пишет в Exec путь os.Executable, а у AppImage это точка
// монтирования /tmp/.mount_*, которая меняется при каждом запуске; путь к
// самому файлу AppImage — в переменной APPIMAGE.
func NewAutostart(*application.App) app.Autostart {
	dir, err := os.UserConfigDir() // учитывает XDG_CONFIG_HOME
	if err != nil {
		return nil
	}
	exe := os.Getenv("APPIMAGE")
	if exe == "" {
		if exe, err = os.Executable(); err != nil {
			return nil
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
	}
	return xdgAutostart{path: filepath.Join(dir, "autostart", "forktopus.desktop"), exe: exe}
}

type xdgAutostart struct {
	path string // файл .desktop
	exe  string // что запускать
}

// Enabled — файл есть и не отключён (Hidden=true по спецификации — «удалён»).
func (x xdgAutostart) Enabled() (bool, error) {
	data, err := os.ReadFile(x.path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !strings.Contains(string(data), "\nHidden=true"), nil
}

func (x xdgAutostart) SetEnabled(on bool) error {
	if !on {
		if err := os.Remove(x.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(x.path), 0o755); err != nil {
		return err
	}
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Forktopus
Comment=Все ваши Git-репозитории на одном экране
Exec=%s %s
Icon=forktopus
Terminal=false
X-GNOME-Autostart-enabled=true
`, quoteExec(x.exe), HiddenFlag)
	tmp := x.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(entry), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, x.path)
}

// quoteExec — аргумент строки Exec: с пробелами или спецсимволами — в
// кавычках, а " ` $ \ экранируются (Desktop Entry Specification).
func quoteExec(s string) string {
	if !strings.ContainsAny(s, " \t\"`$\\'><~|&;*?#()") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`)
	// Обратная косая черта — ещё и экранирование самого значения ключа.
	return `"` + strings.ReplaceAll(r.Replace(s), `\`, `\\`) + `"`
}
