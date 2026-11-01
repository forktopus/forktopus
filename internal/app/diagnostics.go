package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/forktopus/forktopus/internal/applog"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// reportOps — сколько последних операций попадает в отчёт.
const reportOps = 10

// AttachLog подключает технический лог к отчёту диагностики. Функция, а не
// метод: экспортированные методы сервиса Wails отдаёт в UI.
func AttachLog(s *AppService, path string, recent *applog.Recent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logPath, s.recent = path, recent
}

// GetDiagnostics собирает отчёт диагностики: ОС, пути настроек и лога,
// утилиты (из последней проверки, если её не было — проверяет сейчас),
// корни, последние ошибки лога и операции. Домашний каталог заменяется на
// «~», весь текст проходит маскировку секретов — отчёт можно отправить.
func (s *AppService) GetDiagnostics(ctx context.Context) model.Diagnostics {
	s.mu.Lock()
	toolsKnown := s.lastTools != nil
	s.mu.Unlock()
	if !toolsKnown {
		s.CheckTools(ctx)
	}

	s.mu.Lock()
	diagTools, toolsAt := slices.Clone(s.lastTools), s.toolsAt
	roots, repos := len(s.cfg.Roots), slices.Clone(s.snapshot.Repositories)
	ops := slices.Clone(s.opLog)
	logPath, recent := s.logPath, s.recent
	s.mu.Unlock()

	clean := cleaner()
	d := model.Diagnostics{OS: osName(), ConfigPath: clean(s.store.Path()), LogPath: clean(logPath), Errors: []model.LogEntry{}}
	if recent != nil {
		for _, e := range recent.Entries() {
			d.Errors = append(d.Errors, model.LogEntry{Time: e.Time, Level: e.Level, Message: clean(e.Message)})
		}
	}

	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("Forktopus — отчёт диагностики, %s", time.Now().Format("2006-01-02 15:04:05 -0700"))
	line("ОС: %s", d.OS)
	line("Настройки: %s", d.ConfigPath)
	if d.LogPath != "" {
		line("Лог: %s", d.LogPath)
	}
	failed := 0
	for _, r := range repos {
		if r.Error != nil {
			failed++
		}
	}
	line("Корневых папок: %d, репозиториев: %d, с ошибкой чтения: %d", roots, len(repos), failed)

	line("\nУтилиты (проверены %s):", toolsAt.Format("15:04:05"))
	for _, t := range diagTools {
		parts := []string{t.Name}
		if t.Host != "" {
			parts = append(parts, t.Host)
		}
		if t.Version != "" {
			parts = append(parts, t.Version)
		}
		parts = append(parts, string(t.Availability))
		if t.Path != "" {
			parts = append(parts, t.Path)
		}
		if t.Error != nil {
			parts = append(parts, string(t.Error.Code)+": "+t.Error.Message)
		}
		line("- %s", strings.Join(parts, " · "))
	}

	line("\nПоследние ошибки:")
	if len(d.Errors) == 0 {
		line("- нет")
	}
	for _, e := range d.Errors {
		line("- %s %s %s", e.Time.Format("15:04:05"), e.Level, e.Message)
	}

	line("\nПоследние операции:")
	if len(ops) == 0 {
		line("- нет")
	}
	names := map[string]string{}
	for _, r := range repos {
		names[r.ID] = filepath.Base(r.Path)
	}
	slices.Reverse(ops)
	for _, op := range ops[:min(len(ops), reportOps)] {
		result := "готово"
		if op.Error != nil {
			result = string(op.Error.Code) + ": " + op.Error.Message
		}
		line("- %s %s %s %s (%d мс) — %s", op.At.Format("15:04:05"), op.Op, names[op.RepoID], op.Target, op.DurationMs, result)
	}
	d.Report = clean(b.String())
	return d
}

// cleaner — маскировка секретов и «~» вместо домашнего каталога.
func cleaner() func(string) string {
	home, _ := os.UserHomeDir()
	return func(s string) string {
		s = runner.Redact(s)
		if home != "" && home != "/" {
			s = strings.ReplaceAll(s, home+string(filepath.Separator), "~"+string(filepath.Separator))
			if strings.HasSuffix(s, home) {
				s = strings.TrimSuffix(s, home) + "~"
			}
		}
		return s
	}
}
