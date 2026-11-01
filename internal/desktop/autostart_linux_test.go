package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXDGAutostart(t *testing.T) { // 18.4
	x := xdgAutostart{path: filepath.Join(t.TempDir(), "autostart", "forktopus.desktop"), exe: "/opt/My Apps/forktopus"}
	if on, err := x.Enabled(); err != nil || on {
		t.Fatalf("до включения: %v, %v", on, err)
	}
	if err := x.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(x.path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\nExec=\"/opt/My Apps/forktopus\" --hidden\n") {
		t.Fatalf("строка Exec:\n%s", data)
	}
	if on, _ := x.Enabled(); !on {
		t.Fatal("после включения автозапуск выключен")
	}

	// Пользователь отключил запись сам (Hidden=true) — считается выключенным.
	if err := os.WriteFile(x.path, append(data, "Hidden=true\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if on, _ := x.Enabled(); on {
		t.Fatal("Hidden=true, а автозапуск включён")
	}

	if err := x.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(x.path); !os.IsNotExist(err) {
		t.Fatalf("файл не удалён: %v", err)
	}
	if err := x.SetEnabled(false); err != nil {
		t.Fatalf("повторное выключение: %v", err)
	}
}

func TestQuoteExec(t *testing.T) {
	for in, want := range map[string]string{
		"/usr/bin/forktopus":  "/usr/bin/forktopus",
		"/home/a b/Forktopus": `"/home/a b/Forktopus"`,
		`/x/$HOME"q`:          `"/x/\\$HOME\\"q"`,
	} {
		if got := quoteExec(in); got != want {
			t.Errorf("%q: %s, ожидалось %s", in, got, want)
		}
	}
}
