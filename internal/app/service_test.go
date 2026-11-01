package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
	"github.com/forktopus/forktopus/internal/settings"
)

func newService(t *testing.T) (*AppService, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config", "config.json")
	s, err := NewAppService(settings.NewStore(path), runner.Exec{}, "")
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func TestRootsLifecycle(t *testing.T) {
	s, cfgPath := newService(t)
	dir := t.TempDir()

	root, err := s.AddRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root.ID == "" || len(s.ListRoots()) != 1 {
		t.Fatalf("корень не добавлен: %+v, %+v", root, s.ListRoots())
	}

	// Тот же каталог через симлинк — та же запись.
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	again, err := s.AddRoot(link)
	if err != nil || again.ID != root.ID || len(s.ListRoots()) != 1 {
		t.Fatalf("дубликат через симлинк: %+v err=%v roots=%+v", again, err, s.ListRoots())
	}

	// Перезапуск: список читается из файла.
	restarted, err := NewAppService(settings.NewStore(cfgPath), runner.Exec{}, "")
	if err != nil || len(restarted.ListRoots()) != 1 {
		t.Fatalf("после перезапуска: %+v err=%v", restarted.ListRoots(), err)
	}

	if err := s.RemoveRoot(root.ID); err != nil {
		t.Fatal(err)
	}
	if len(s.ListRoots()) != 0 {
		t.Fatal("корень не удалён из списка")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("удаление корня затронуло файлы: %v", err)
	}
}

func TestAddRootRejectsBadInput(t *testing.T) {
	s, _ := newService(t)
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"relative/path", filepath.Join(t.TempDir(), "missing"), file} {
		if _, err := s.AddRoot(p); model.CodeOf(err) != model.CodeInvalidInput {
			t.Errorf("AddRoot(%q): ожидался INVALID_INPUT, получено %v", p, err)
		}
	}
	if err := s.RemoveRoot("unknown"); model.CodeOf(err) != model.CodeInvalidInput {
		t.Errorf("RemoveRoot(unknown): %v", err)
	}
}

func TestAddRootFailsOnCorruptSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewAppService(settings.NewStore(path), runner.Exec{}, "")
	if err == nil {
		t.Fatal("ожидалась ошибка чтения настроек")
	}
	if _, err := s.AddRoot(t.TempDir()); err == nil {
		t.Fatal("запись в повреждённые настройки должна отклоняться")
	}
	if len(s.ListRoots()) != 0 {
		t.Fatal("корень добавлен в память, хотя не сохранён")
	}
}

func TestUIStatePersists(t *testing.T) {
	s, cfgPath := newService(t)
	ui := settings.UI{Theme: "light", Mode: "explorer", Filters: map[string]string{"tab": "dirty"}}
	if err := s.SaveUI(ui); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewAppService(settings.NewStore(cfgPath), runner.Exec{}, "")
	if err != nil {
		t.Fatal(err)
	}
	got := restarted.GetUI()
	if got.Theme != "light" || got.Mode != "explorer" || got.Filters["tab"] != "dirty" {
		t.Fatalf("после перезапуска: %+v", got)
	}
}

func TestCheckTools(t *testing.T) {
	s, _ := newService(t)
	diags := s.CheckTools(context.Background())
	// Репозиториев с серверными remote нет — CLI провайдеров проверяются без хоста.
	// Установлены ли они в окружении теста, неважно: важно, что не «ок» без версии.
	// tea — запасной CLI Gitea (D-074): не установлен — в списке его нет.
	names := []string{"gh", "glab", "gt"}
	if len(diags) == 5 {
		names = append(names, "tea")
	}
	if len(diags) != len(names)+1 || diags[0].Name != "git" || diags[0].Availability != model.AvailabilityOK {
		t.Fatalf("CheckTools: %+v", diags)
	}
	for i, name := range names {
		d := diags[i+1]
		if d.Name != name || d.Host != "" || (d.Availability == model.AvailabilityOK) == (d.Version == "") {
			t.Errorf("%s: %+v", name, d)
		}
	}
}

// blockingRunner ждёт отмены контекста — как долгий fetch или зависший CLI.
type blockingRunner struct{ started chan struct{} }

func (b blockingRunner) Run(ctx context.Context, _ runner.Command) (runner.Result, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return runner.Result{}, model.Errorf(model.CodeOpCancelled, "отменено")
}

// Завершение приложения отменяет идущие команды (11.4).
func TestShutdownCancelsCommands(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "repo", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := blockingRunner{started: make(chan struct{}, 1)}
	s, err := NewAppService(settings.NewStore(filepath.Join(t.TempDir(), "config.json")), r, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRoot(root); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := s.ScanRoots(context.Background())
		done <- err
	}()
	select {
	case <-r.started:
	case <-time.After(2 * time.Second):
		t.Fatal("скан не начался")
	}
	if err := s.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if model.CodeOf(err) != model.CodeOpCancelled {
			t.Fatalf("ожидался OP_CANCELLED, получено %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("скан не отменился при завершении")
	}
	// После завершения новые вызовы сразу отменены.
	if _, err := s.ScanRoots(context.Background()); model.CodeOf(err) != model.CodeOpCancelled {
		t.Fatalf("после завершения: %v", err)
	}
}
