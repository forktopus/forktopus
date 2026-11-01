// Package launch открывает каталог в файловом менеджере системы и запускает
// в нём терминал (D-066). Процессы отпускаются: приложение их не ждёт.
package launch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// NearestDir — сам каталог или ближайший существующий выше по пути: у
// проблемного каталога его может уже не быть. Пустой путь — домашний каталог.
func NearestDir(path string) (string, error) {
	if path == "" {
		return os.UserHomeDir()
	}
	dir := filepath.Clean(path)
	for {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir, nil
		}
		up := filepath.Dir(dir)
		if up == dir {
			return "", errors.New("каталог не найден: " + path)
		}
		dir = up
	}
}

// Folder открывает ближайший к path существующий каталог в файловом менеджере.
func Folder(path string) error {
	dir, err := NearestDir(path)
	if err != nil {
		return err
	}
	return start(folderCmd(dir))
}

// Terminal запускает терминал системы с рабочим каталогом — ближайшим к path.
func Terminal(path string) error {
	dir, err := NearestDir(path)
	if err != nil {
		return err
	}
	cmd, err := terminalCmd(dir)
	if err != nil {
		return err
	}
	return start(cmd)
}

func start(cmd *exec.Cmd) error {
	cmd.Env = cleanEnv(os.Environ())
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }() // забрать процесс, когда завершится
	return nil
}

// firstFound — первая из программ, что есть в PATH.
func firstFound(names ...string) (string, bool) {
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p, true
		}
	}
	return "", false
}

// appImageVars — переменные, которые задают AppRun и Wails для самого
// приложения; в запущенных программах (терминал, шелл в нём) им не место.
var appImageVars = []string{
	"APPDIR", "APPIMAGE", "ARGV0", "OWD",
	"GSETTINGS_SCHEMA_DIR", "WEBKIT_INJECTED_BUNDLE_PATH", "WEBKIT_DISABLE_DMABUF_RENDERER",
	"GTK_DATA_PREFIX", "GTK_EXE_PREFIX", "GTK_PATH", "GTK_IM_MODULE_FILE", "GTK_THEME",
	"GDK_PIXBUF_MODULE_FILE", "GI_TYPELIB_PATH", "GIO_MODULE_DIR",
}

// cleanEnv — окружение без следов AppImage: из списков путей убираются
// каталоги внутри $APPDIR, переменные из appImageVars — целиком. Вне AppImage
// окружение не меняется.
func cleanEnv(env []string) []string {
	appdir := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "APPDIR="); ok {
			appdir = v
		}
	}
	if appdir == "" {
		return env
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		switch {
		case slices.Contains(appImageVars, k):
			continue
		case k == "PATH" || k == "LD_LIBRARY_PATH" || k == "XDG_DATA_DIRS":
			parts := slices.DeleteFunc(filepath.SplitList(v), func(p string) bool {
				return p == "" || p == appdir || strings.HasPrefix(p, appdir+string(filepath.Separator))
			})
			if len(parts) == 0 {
				continue
			}
			kv = k + "=" + strings.Join(parts, string(filepath.ListSeparator))
		}
		out = append(out, kv)
	}
	return out
}
