package gitlocal

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/forktopus/forktopus/internal/gittest"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

func canonical(t *testing.T, p string) string {
	t.Helper()
	c, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInspect(t *testing.T) {
	ctx := context.Background()
	repo := gittest.Clean(t)
	sub := filepath.Join(repo, "sub dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	info, err := Inspect(ctx, runner.Exec{}, sub)
	if err != nil {
		t.Fatal(err)
	}
	if info.TopLevel != canonical(t, repo) || info.GitDir != filepath.Join(canonical(t, repo), ".git") || info.Superproject != "" {
		t.Errorf("Inspect: %+v", info)
	}

	main, wt := gittest.Worktree(t)
	info, err = Inspect(ctx, runner.Exec{}, wt)
	if err != nil {
		t.Fatal(err)
	}
	if info.TopLevel != canonical(t, wt) || filepath.Dir(filepath.Dir(info.GitDir)) != filepath.Join(canonical(t, main), ".git") {
		t.Errorf("Inspect worktree: %+v", info)
	}

	if _, err := Inspect(ctx, runner.Exec{}, t.TempDir()); model.CodeOf(err) != model.CodeOther {
		t.Errorf("обычный каталог: %v", err)
	}
}

func TestInspectSubmodule(t *testing.T) {
	parent := gittest.Clean(t)
	child := gittest.Clean(t)
	gittest.Git(t, parent, "-c", "protocol.file.allow=always", "submodule", "add", "-q", child, "libs/child")
	info, err := Inspect(context.Background(), runner.Exec{}, filepath.Join(parent, "libs", "child"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Superproject != canonical(t, parent) {
		t.Errorf("Superproject = %q, want %q", info.Superproject, parent)
	}
}

func TestRemotes(t *testing.T) {
	repo := gittest.Clean(t)
	gittest.Git(t, repo, "remote", "add", "origin", "https://user:token@github.com/org/repo.git")
	gittest.Git(t, repo, "remote", "set-url", "--push", "origin", "git@github.com:org/repo.git")
	gittest.Git(t, repo, "remote", "add", "local", "/srv/repos/Мои проекты/demo.git")
	remotes, err := Remotes(context.Background(), runner.Exec{}, repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.RemoteInfo{
		{Name: "local", FetchURL: "/srv/repos/Мои проекты/demo.git", Provider: model.ProviderOther},
		{Name: "origin", FetchURL: "https://github.com/org/repo.git", PushURL: "git@github.com:org/repo.git", Host: "github.com", Path: "org/repo", Provider: model.ProviderGitHub},
	}
	if len(remotes) != len(want) {
		t.Fatalf("Remotes: %+v", remotes)
	}
	for i := range want {
		if remotes[i] != want[i] {
			t.Errorf("remote %d:\n got %+v\nwant %+v", i, remotes[i], want[i])
		}
	}
}

func TestSelectRemote(t *testing.T) {
	r := func(names ...string) []model.RemoteInfo {
		out := []model.RemoteInfo{}
		for _, n := range names {
			out = append(out, model.RemoteInfo{Name: n})
		}
		return out
	}
	cases := []struct {
		remotes   []model.RemoteInfo
		preferred string
		want      string
	}{
		{r("origin", "upstream"), "", "origin"},
		{r("origin", "upstream"), "upstream", "upstream"},
		{r("origin"), "gone", "origin"},
		{r("fork"), "", "fork"},
		{r("fork", "upstream"), "", ""},
		{r(), "", ""},
	}
	for _, c := range cases {
		if got := SelectRemote(c.remotes, c.preferred); got != c.want {
			t.Errorf("SelectRemote(%v, %q) = %q, want %q", c.remotes, c.preferred, got, c.want)
		}
	}
}
