package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Путь задаётся пользователем, поэтому его метасимволы не должны расширять
// разрешение на другие файлы или добавлять произвольные правила AppArmor.
func TestProfileExactPath(t *testing.T) {
	path := "/tmp/тест [*?]/{a,b}/@{HOME}/quote\"\\\n.AppImage"
	name, body := profile(path)
	if !strings.HasPrefix(name, "forktopus-appimage-") || len(name) != len("forktopus-appimage-")+64 {
		t.Fatalf("неверное имя: %s", name)
	}
	for _, unwanted := range []string{"@{HOME}", "[*?]", "{a,b}", "quote\"", "\n.AppImage"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("не экранирован путь: %q", body)
		}
	}
	other, _ := profile(path + "2")
	if name == other {
		t.Fatal("разные пути получили одно имя профиля")
	}
	if again, _ := profile(path); again != name {
		t.Fatal("имя профиля нестабильно")
	}
	parser, err := exec.LookPath("apparmor_parser")
	if err != nil {
		parser = "/sbin/apparmor_parser"
	}
	if _, err = os.Stat(parser); err != nil {
		t.Skip("нет парсера AppArmor для проверки синтаксиса")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "profile")
	if err = os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(parser, "-Q", "-T", "-o", filepath.Join(dir, "compiled"), file).CombinedOutput(); err != nil {
		t.Fatalf("профиль не компилируется: %v\n%s", err, out)
	}
}

func TestImagePath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Forktopus.AppImage")
	data := []byte("\x7fELF\x02\x01\x01\x00AI\x02")
	if err := os.WriteFile(file, data, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if got, err := imagePath(link); err != nil || got != file {
		t.Fatalf("не разрешён путь ссылки: %q, %v", got, err)
	}
	if err := os.WriteFile(file, []byte("не AppImage"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"", file, file + "-missing"} {
		if _, err := imagePath(raw); err == nil {
			t.Fatalf("принят неверный файл %q", raw)
		}
	}
}

// Отмена, отказ pkexec и успешная настройка проверяются без root и без
// изменения системы; аргументы sh отделены от пути и текста профиля.
func TestSetupOutcomes(t *testing.T) {
	dir := t.TempDir()
	image := filepath.Join(dir, "image ' [*].AppImage")
	if err := os.WriteFile(image, []byte("\x7fELF\x02\x01\x01\x00AI\x02"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPIMAGE", image)
	t.Setenv("PATH", dir)
	pkexec := filepath.Join(dir, "pkexec")
	for _, tt := range []struct {
		name, script string
		want         int
	}{
		{"success", "exit 0", 0},
		{"cancel", "exit 126", 1},
		{"failure", "exit 1", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(pkexec, []byte("#!/bin/sh\n"+tt.script+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if got := run([]string{setupFlag}); got != tt.want {
				t.Fatalf("получен код %d, ожидался %d", got, tt.want)
			}
		})
	}
	if got := run([]string{"--version"}); got != 0 {
		t.Fatal("--version требует настройку системы")
	}
}

func TestInstallerRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{"../../bad", "forktopus-appimage-../bad", "forktopus-appimage-a", "forktopus-appimage-" + strings.Repeat("z", 64)} {
		if err := exec.Command("/bin/sh", "-c", installScript, "test", name, "body").Run(); err == nil {
			t.Fatalf("принято неверное имя %q", name)
		}
	}
}

func TestNormalStartupDecisions(t *testing.T) {
	dir := t.TempDir()
	image := filepath.Join(dir, "Forktopus.AppImage")
	if err := os.WriteFile(image, []byte("\x7fELF\x02\x01\x01\x00AI\x02"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPIMAGE", image)
	t.Setenv("PATH", dir)
	marker := filepath.Join(dir, "installed")
	t.Setenv("CAPTURE", marker)
	if err := os.WriteFile(filepath.Join(dir, "pkexec"), []byte("#!/bin/sh\n[ \"$1\" = /bin/sh ] && [ \"$2\" = -c ] && [ \"$4\" = forktopus-apparmor ] || exit 1\nprintf '%s' \"$5\" > \"$CAPTURE\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name                         string
		args                         []string
		restricted, allowed, consent bool
		want                         int
	}{
		{"unrestricted", nil, false, false, true, 0},
		{"already-configured", nil, true, true, true, 0},
		{"cancel", nil, true, false, false, 1},
		{"consent-restarts", []string{"argument with spaces"}, true, false, true, restart},
		{"autostart-does-not-prompt", []string{"--hidden"}, true, false, true, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Remove(marker)
			status := "1"
			if tt.consent {
				status = "0"
			}
			if err := os.WriteFile(filepath.Join(dir, "zenity"), []byte("#!/bin/sh\nexit "+status+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			got := prepare(tt.args, func() bool { return tt.restricted }, func() error {
				if tt.allowed {
					return nil
				}
				return errors.New("userns запрещены")
			})
			if got != tt.want {
				t.Fatalf("код %d, ожидался %d", got, tt.want)
			}
			body, err := os.ReadFile(marker)
			if tt.want == restart {
				name, _ := profile(image)
				if err != nil || string(body) != name {
					t.Fatalf("не передан точный профиль: %q %v", body, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatal("вызван pkexec без необходимости или согласия")
			}
		})
	}
}

// После настройки нужен exec исходного AppImage для вступления профиля в
// силу. Обычный запуск и повторный запуск обязаны сохранить аргументы.
func TestAppRunForwarding(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "usr", "libexec"), 0700); err != nil {
		t.Fatal(err)
	}
	wrapper, err := os.ReadFile("../../build/linux/appimage/AppRun")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"AppRun":                         wrapper,
		"usr/libexec/forktopus-appimage": []byte("#!/bin/sh\nexit \"$HELPER_STATUS\"\n"),
		"AppRun.wails":                   []byte("#!/bin/sh\nprintf '%s\\n' inner \"$@\" > \"$CAPTURE\"\n"),
		"image":                          []byte("#!/bin/sh\nprintf '%s\\n' \"$FORKTOPUS_APPIMAGE_SETUP_RETRY\" \"$@\" > \"$CAPTURE\"\n"),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name, status string
		args         []string
		want         string
		fails        bool
	}{
		{"normal", "0", []string{"a b", "--hidden"}, "inner\na b\n--hidden\n", false},
		{"restart", "20", []string{"a b", "--hidden"}, "1\na b\n--hidden\n", false},
		{"explicit", "0", []string{setupFlag}, "", false},
		{"cancel", "1", nil, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			capture := filepath.Join(dir, "capture")
			_ = os.Remove(capture)
			cmd := exec.Command(filepath.Join(dir, "AppRun"), tt.args...)
			cmd.Env = append(os.Environ(), "HELPER_STATUS="+tt.status, "APPIMAGE="+filepath.Join(dir, "image"), "CAPTURE="+capture)
			err := cmd.Run()
			if (err != nil) != tt.fails {
				t.Fatalf("неверный результат: %v", err)
			}
			body, _ := os.ReadFile(capture)
			if string(body) != tt.want {
				t.Fatalf("получено %q, ожидалось %q", body, tt.want)
			}
		})
	}
}
