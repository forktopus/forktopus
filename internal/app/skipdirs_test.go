package app

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/forktopus/forktopus/internal/gittest"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
	"github.com/forktopus/forktopus/internal/scan"
	"github.com/forktopus/forktopus/internal/settings"
)

// Список пропусков сохраняется, проверяется и действует со следующего скана (11.6).
func TestSkipDirs(t *testing.T) {
	ctx := context.Background()
	s, cfgPath := newService(t)
	if got := s.GetSkipDirs(); !slices.Equal(got.Dirs, scan.DefaultSkipDirs) || !slices.Equal(got.Defaults, scan.DefaultSkipDirs) {
		t.Fatalf("по умолчанию: %+v", got)
	}

	root := t.TempDir()
	gittest.Init(t, filepath.Join(root, "app"))
	gittest.Init(t, filepath.Join(root, "build", "dep"))
	gittest.Init(t, filepath.Join(root, "archive", "old"))
	if _, err := s.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	names := func() []string {
		res, err := s.ScanRoots(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, r := range res.Repositories {
			out = append(out, filepath.Base(r.Path))
		}
		slices.Sort(out)
		return out
	}
	if got := names(); !slices.Equal(got, []string{"app", "old"}) {
		t.Fatalf("скан по умолчанию: %v", got)
	}

	got, err := s.SetSkipDirs([]string{" archive ", "", "node_modules", "archive"})
	if err != nil || !slices.Equal(got.Dirs, []string{"archive", "node_modules"}) {
		t.Fatalf("SetSkipDirs: %+v err=%v", got, err)
	}
	if got := names(); !slices.Equal(got, []string{"app", "dep"}) {
		t.Fatalf("скан со своим списком: %v", got)
	}

	// Переживает перезапуск.
	restarted, err := NewAppService(settings.NewStore(cfgPath), runner.Exec{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := restarted.GetSkipDirs(); !slices.Equal(got.Dirs, []string{"archive", "node_modules"}) {
		t.Fatalf("после перезапуска: %+v", got)
	}

	for _, bad := range []string{"a/b", `a\b`, "..", "."} {
		if _, err := s.SetSkipDirs([]string{"x", bad}); model.CodeOf(err) != model.CodeInvalidInput {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if got := s.GetSkipDirs(); !slices.Equal(got.Dirs, []string{"archive", "node_modules"}) {
		t.Fatalf("ошибка ввода не должна менять список: %+v", got)
	}

	// Пустой и совпадающий с умолчанием (в другом порядке) — сброс: в файле поля нет.
	reversed := slices.Clone(scan.DefaultSkipDirs)
	slices.Reverse(reversed)
	for _, reset := range [][]string{nil, reversed} {
		if _, err := s.SetSkipDirs([]string{"archive"}); err != nil {
			t.Fatal(err)
		}
		got, err := s.SetSkipDirs(reset)
		if err != nil || !slices.Equal(got.Dirs, scan.DefaultSkipDirs) {
			t.Fatalf("сброс %v: %+v err=%v", reset, got, err)
		}
		if cfg, _ := settings.NewStore(cfgPath).Load(); cfg.ScanSkipDirs != nil {
			t.Fatalf("сброс %v: в настройках %v", reset, cfg.ScanSkipDirs)
		}
	}
	if got := names(); !slices.Equal(got, []string{"app", "old"}) {
		t.Fatalf("скан после сброса: %v", got)
	}
}

// Исключённый путь пропадает из проблем сразу, сканер в него не заходит, список можно вернуть (D-067).
func TestSkipPaths(t *testing.T) {
	ctx := context.Background()
	s, cfgPath := newService(t)
	root := t.TempDir()
	missing := filepath.Join(root, "gone")
	if _, err := s.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.snapshot.Issues = []model.ScanIssue{{RootID: "x", Path: filepath.Join(missing, "deep")}, {RootID: "x", Path: filepath.Join(root, "gone2")}}
	s.mu.Unlock()

	issues, err := s.SetSkipPaths([]string{missing + "/", " ", missing})
	if err != nil || len(issues) != 1 || issues[0].Path != filepath.Join(root, "gone2") {
		t.Fatalf("SetSkipPaths: %+v err=%v", issues, err)
	}
	if got := s.GetSkipPaths(); !slices.Equal(got, []string{missing}) {
		t.Fatalf("GetSkipPaths: %v", got)
	}
	if cfg, _ := settings.NewStore(cfgPath).Load(); !slices.Equal(cfg.ScanSkipPaths, []string{missing}) {
		t.Fatalf("в настройках: %v", cfg.ScanSkipPaths)
	}
	if _, err := s.SetSkipPaths([]string{"relative/path"}); model.CodeOf(err) != model.CodeInvalidInput {
		t.Errorf("относительный путь: %v", err)
	}

	gittest.Init(t, filepath.Join(root, "skipped", "repo"))
	if _, err := s.SetSkipPaths([]string{filepath.Join(root, "skipped")}); err != nil {
		t.Fatal(err)
	}
	if res, err := s.ScanRoots(ctx); err != nil || len(res.Repositories) != 0 {
		t.Fatalf("скан с пропуском: %+v err=%v", res.Repositories, err)
	}
	if _, err := s.SetSkipPaths(nil); err != nil {
		t.Fatal(err)
	}
	if res, err := s.ScanRoots(ctx); err != nil || len(res.Repositories) != 1 {
		t.Fatalf("скан после возврата: %+v err=%v", res.Repositories, err)
	}
}
