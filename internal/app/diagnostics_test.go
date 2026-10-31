package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/applog"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/settings"
)

// Отчёт содержит ОС, пути, утилиты, ошибки и операции — без секретов и домашнего каталога (11.5).
func TestDiagnosticsReport(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "/" {
		t.Skip("нет домашнего каталога")
	}
	ctx := context.Background()
	// Настройки «в домашнем каталоге» — путь нужен только для отчёта, файл в TempDir.
	s, err := NewAppService(settings.NewStore(filepath.Join(t.TempDir(), "config.json")), &ghRunner{}, "")
	if err != nil {
		t.Fatal(err)
	}
	s.store = settings.NewStore(filepath.Join(home, ".config", "forktopus", "config.json"))
	log, recent := applog.New(&strings.Builder{})
	AttachLog(s, filepath.Join(home, ".cache", "forktopus", "forktopus.log"), recent)
	log.Warn("команда завершилась с ошибкой", "bin", "gh", "error", "401: token ghp_abcdef123 invalid in "+home+"/work")

	s.mu.Lock()
	s.opLog = append(s.opLog, model.OpRecord{RepoID: "x", Op: model.OperationFetch, Target: "origin", At: time.Now(), DurationMs: 12,
		Error: model.Errorf(model.CodeNetworkError, "fatal: unable to access 'https://user:s3cret@github.com/o/r.git/'")})
	s.mu.Unlock()

	d := s.GetDiagnostics(ctx)
	for _, secret := range []string{"s3cret", "ghp_abcdef123", home + "/"} {
		if strings.Contains(d.Report, secret) || strings.Contains(d.ConfigPath+d.LogPath, secret) || strings.Contains(d.Errors[0].Message, secret) {
			t.Errorf("в отчёте %q:\n%s", secret, d.Report)
		}
	}
	for _, want := range []string{
		"ОС: ", "Настройки: ~/.config/forktopus/config.json", "Лог: ~/.cache/forktopus/forktopus.log",
		"- git ", "- gh · 2.103.3", "ghp_***", "~/work", "fetch  origin (12 мс) — NETWORK_ERROR", "https://***@github.com",
	} {
		if !strings.Contains(d.Report, want) {
			t.Errorf("в отчёте нет %q:\n%s", want, d.Report)
		}
	}
	if d.ConfigPath != "~/.config/forktopus/config.json" || len(d.Errors) != 1 || d.OS == "" {
		t.Errorf("поля: %+v", d)
	}
}
