package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "forktopus", "config.json"))
	cfg, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != FormatVersion || cfg.Roots == nil || len(cfg.Roots) != 0 {
		t.Fatalf("неожиданные значения по умолчанию: %+v", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "forktopus")
	s := NewStore(filepath.Join(dir, "config.json"))
	want := Default()
	want.Roots = []model.Root{{ID: "a1", Path: "/srv/repos/github.com"}}
	want.Remotes = map[string]string{"repo1": "upstream"}
	want.HostProviders = map[string]model.Provider{"git.example.org:2222": model.ProviderGitea}
	want.UI = UI{Theme: "dark", Mode: "radar", Filters: map[string]string{"host": "github.com"}}

	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := NewStore(s.Path()).Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Roots[0] != want.Roots[0] || got.Remotes["repo1"] != "upstream" ||
		got.HostProviders["git.example.org:2222"] != model.ProviderGitea ||
		got.UI.Mode != "radar" || got.UI.Filters["host"] != "github.com" {
		t.Fatalf("после чтения: %+v", got)
	}

	assertPerm(t, s.Path(), 0o600)
	assertPerm(t, dir, 0o700)
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("в каталоге остались временные файлы: %v", entries)
	}
}

func TestCorruptFileIsNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	corrupt := []byte(`{"roots": [`)
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path)
	cfg, err := s.Load()
	if err == nil {
		t.Fatal("ожидалась ошибка разбора")
	}
	if cfg.Roots == nil {
		t.Error("при ошибке должны вернуться значения по умолчанию")
	}
	if err := s.Save(Default()); err == nil {
		t.Fatal("повреждённый файл не должен перезаписываться")
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(corrupt) {
		t.Fatalf("файл изменён: %q", data)
	}

	// Пользователь исправил файл — запись снова разрешена.
	if err := os.WriteFile(path, []byte(`{"version":1,"roots":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(Default()); err != nil {
		t.Fatalf("после исправления запись отклонена: %v", err)
	}
}

func assertPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s: права %o, ожидалось %o", path, got, want)
	}
}

func TestZoomLevel(t *testing.T) { // D-061
	for _, c := range []struct{ zoom, want float64 }{{0, 1}, {0.5, 1}, {1.3, 1.3}, {2, 2}, {5, 2}} {
		if got := (UI{Zoom: c.zoom}).ZoomLevel(); got != c.want {
			t.Errorf("Zoom %v: %v, ожидалось %v", c.zoom, got, c.want)
		}
	}
}
