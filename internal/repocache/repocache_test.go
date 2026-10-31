package repocache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/model"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache", "repositories.json")
	if _, ok := Load(path); ok {
		t.Fatal("кэша ещё нет")
	}
	at := time.Date(2026, 10, 31, 10, 42, 0, 0, time.UTC)
	ahead := 2
	in := model.ScanResult{
		Repositories: []model.Repository{{ID: "a", Path: "/srv/repos/a", Remotes: []model.RemoteInfo{}, Branch: &model.BranchStatus{Name: "main", Ahead: &ahead}}},
		Issues:       []model.ScanIssue{{RootID: "r", Path: "/srv/repos/locked", Error: &model.Error{Code: model.CodeOther, Message: "permission denied"}}},
		ScannedAt:    at,
	}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	out, ok := Load(path)
	if !ok || !out.ScannedAt.Equal(at) || len(out.Repositories) != 1 || *out.Repositories[0].Branch.Ahead != 2 || len(out.Issues) != 1 {
		t.Fatalf("после чтения: %+v ok=%v", out, ok)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("права файла: %v %v", info.Mode(), err)
	}
}

func TestBrokenOrOldCacheIgnored(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"broken.json": "{",
		"old.json":    `{"version": 999, "repositories": []}`,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, ok := Load(path); ok {
			t.Errorf("%s: кэш должен игнорироваться", name)
		}
	}
}
