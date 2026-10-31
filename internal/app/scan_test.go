package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/forktopus/forktopus/internal/gittest"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
	"github.com/forktopus/forktopus/internal/settings"
)

func TestScanRootsAndCache(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cachePath := filepath.Join(dir, "cache", "repositories.json")
	root := t.TempDir()
	keep := gittest.Init(t, filepath.Join(root, "github.com", "org", "keep"))
	gone := gittest.Init(t, filepath.Join(root, "github.com", "org", "gone"))
	gittest.Git(t, keep, "remote", "add", "origin", "git@github.com:org/keep.git")
	gittest.Git(t, keep, "remote", "add", "fork", "git@github.com:me/keep.git")

	s, err := NewAppService(settings.NewStore(cfgPath), runner.Exec{}, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if snap := s.ListRepositories(); !snap.Stale || snap.ScannedAt != nil || len(snap.Repositories) != 0 {
		t.Fatalf("до сканирования: %+v", snap)
	}
	if _, err := s.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	res, err := s.ScanRoots(ctx)
	if err != nil || len(res.Repositories) != 2 {
		t.Fatalf("ScanRoots: %+v err=%v", res, err)
	}
	if snap := s.ListRepositories(); snap.Stale || snap.ScannedAt == nil || len(snap.Repositories) != 2 {
		t.Fatalf("после сканирования: %+v", snap)
	}

	// Выбор remote сохраняется в настройках и переживает пересканирование.
	keepID := model.PathID(mustEval(t, keep))
	if err := s.SelectRemote(keepID, "fork"); err != nil {
		t.Fatal(err)
	}
	if err := s.SelectRemote(keepID, "missing"); model.CodeOf(err) != model.CodeInvalidInput {
		t.Errorf("несуществующий remote: %v", err)
	}

	// Перезапуск: список сразу из кэша, помечен как устаревший.
	restarted, err := NewAppService(settings.NewStore(cfgPath), runner.Exec{}, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	snap := restarted.ListRepositories()
	if !snap.Stale || snap.ScannedAt == nil || len(snap.Repositories) != 2 {
		t.Fatalf("кэш после перезапуска: %+v", snap)
	}

	// Удалённый с диска репозиторий исчезает после фонового сканирования.
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	res, err = restarted.ScanRoots(ctx)
	if err != nil || len(res.Repositories) != 1 || res.Repositories[0].Remote != "fork" {
		t.Fatalf("пересканирование: %+v err=%v", res.Repositories, err)
	}
	if snap := restarted.ListRepositories(); snap.Stale || len(snap.Repositories) != 1 {
		t.Fatalf("после пересканирования: %+v", snap)
	}

	// Удаление корня убирает его репозитории, файлы на диске остаются.
	if err := restarted.RemoveRoot(model.PathID(mustEval(t, root))); err != nil {
		t.Fatal(err)
	}
	if snap := restarted.ListRepositories(); len(snap.Repositories) != 0 {
		t.Fatalf("после удаления корня: %+v", snap)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal(err)
	}
}

func TestBrokenCacheIgnored(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "repositories.json")
	if err := os.WriteFile(cachePath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewAppService(settings.NewStore(filepath.Join(dir, "config.json")), runner.Exec{}, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if snap := s.ListRepositories(); len(snap.Repositories) != 0 || snap.ScannedAt != nil {
		t.Fatalf("повреждённый кэш: %+v", snap)
	}
}

func TestPickFolderWithoutWindow(t *testing.T) {
	s, _ := newService(t)
	if _, err := s.PickFolder(); model.CodeOf(err) != model.CodeUnsupported {
		t.Fatalf("PickFolder без окна: %v", err)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	c, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
