package app

import (
	"context"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/gittest"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/remote"
	"github.com/forktopus/forktopus/internal/runner"
	"github.com/forktopus/forktopus/internal/settings"
)

// ghRunner: git — настоящий, gh отвечает заготовками, glab и tea «не установлены».
type ghRunner struct {
	prCalls atomic.Int32
	offline atomic.Bool
}

func (g *ghRunner) Run(ctx context.Context, c runner.Command) (runner.Result, error) {
	switch c.Name {
	case "git":
		return runner.Exec{}.Run(ctx, c)
	case "gh":
		args := strings.Join(c.Args, " ")
		if g.offline.Load() && !strings.HasPrefix(args, "--version") {
			return runner.Result{ExitCode: 1, Stderr: []byte("error connecting to api.github.com\n")}, nil
		}
		switch {
		case strings.HasPrefix(args, "auth status"):
			return runner.Result{Stdout: []byte(`{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com"}]}}`)}, nil
		case strings.HasPrefix(args, "pr list"):
			g.prCalls.Add(1)
			// PR есть только у ветки main.
			if strings.Contains(args, "--head main ") {
				return runner.Result{Stdout: []byte(`[{"isDraft":true,"url":"https://github.com/o/r/pull/1","isCrossRepository":false,"headRefName":"main"}]`)}, nil
			}
			return runner.Result{Stdout: []byte(`[]`)}, nil
		case strings.HasPrefix(args, "run list"):
			return runner.Result{Stdout: []byte(`[]`)}, nil
		case strings.HasPrefix(args, "--version"):
			return runner.Result{Stdout: []byte("gh version 2.103.3 (2026-10-26)\n")}, nil
		}
	}
	return runner.Result{}, model.Errorf(model.CodeCLIMissing, "%s не найден в PATH", c.Name)
}

// eventually ждёт фонового обновления.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("не дождались: %s", what)
}

func TestRemoteStatusFlow(t *testing.T) {
	ctx := context.Background()
	repo := gittest.Clean(t)
	gittest.Git(t, repo, "remote", "add", "origin", "git@github.com:o/r.git")
	gittest.Git(t, repo, "branch", "feature")
	gh := &ghRunner{}
	s, err := NewAppService(settings.NewStore(filepath.Join(t.TempDir(), "config.json")), gh, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRoot(filepath.Dir(repo)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ScanRoots(ctx); err != nil {
		t.Fatal(err)
	}
	id := model.PathID(mustEval(t, repo))

	// После сканирования PR/CI приходят в фоне.
	eventually(t, "PR после сканирования", func() bool {
		r, _ := s.GetRepo(id)
		return r.PR.Availability == model.AvailabilityOK
	})
	r, _ := s.GetRepo(id)
	if *r.PR.Open != 1 || *r.PR.Draft != 1 || *r.CI.Active != 0 || r.Remotes[0].Provider != model.ProviderGitHub {
		t.Errorf("PR/CI в списке: %+v %+v", r.PR, r.CI)
	}

	// Повторный запрос — из кэша, пересканирование не теряет данные.
	st, err := s.GetRemoteStatus(ctx, id)
	if err != nil || st.Branch != "main" || *st.PR.Open != 1 || gh.prCalls.Load() != 1 {
		t.Errorf("из кэша: %+v, вызовов %d", st, gh.prCalls.Load())
	}
	if _, err := s.ScanRoots(ctx); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.GetRepo(id); r.PR.Open == nil || *r.PR.Open != 1 {
		t.Errorf("пересканирование стёрло PR: %+v", r.PR)
	}

	// Ручное обновление идёт мимо кэша.
	if _, err := s.RefreshRemote(ctx, id); err != nil || gh.prCalls.Load() < 2 {
		t.Errorf("RefreshRemote: %v, вызовов %d", err, gh.prCalls.Load())
	}

	// Смена ветки: данные main не показываются для feature, новые приходят в фоне.
	res, _ := s.SwitchBranch(ctx, id, "feature", shown(t, s, id))
	if res.Error != nil {
		t.Fatalf("switch: %+v", res.Error)
	}
	if res.Repo.PR.Open != nil && *res.Repo.PR.Open == 1 {
		t.Error("PR ветки main показан для feature")
	}
	eventually(t, "PR новой ветки", func() bool {
		r, _ := s.GetRepo(id)
		return r.PR.Availability == model.AvailabilityOK && *r.PR.Open == 0
	})

	// CheckTools: gh проверен для github.com, glab и gt — без хоста; tea не установлен — его нет.
	diags := s.CheckTools(ctx)
	if len(diags) != 4 || diags[1].Name != "gh" || diags[1].Host != "github.com" || diags[1].Availability != model.AvailabilityOK ||
		diags[2].Name != "glab" || diags[2].Availability != model.AvailabilityUnavailable {
		t.Errorf("CheckTools: %+v", diags)
	}
}

// После перезапуска без сети время последнего успеха берётся из кэша списка (A24).
func TestRemoteLastSuccessAfterRestart(t *testing.T) {
	ctx := context.Background()
	repo := gittest.Clean(t)
	gittest.Git(t, repo, "remote", "add", "origin", "git@github.com:o/r.git")
	gh := &ghRunner{}
	s, err := NewAppService(settings.NewStore(filepath.Join(t.TempDir(), "config.json")), gh, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRoot(filepath.Dir(repo)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ScanRoots(ctx); err != nil {
		t.Fatal(err)
	}
	id := model.PathID(mustEval(t, repo))
	ok, err := s.RefreshRemote(ctx, id)
	if err != nil || ok.PR.LastSuccessAt == nil {
		t.Fatalf("онлайн: %v %+v", err, ok.PR)
	}

	// «Перезапуск»: новый Fetcher и пустая память статусов, сети нет.
	s.mu.Lock()
	s.remote, s.remoteByRepo = remote.New(gh, remote.Options{}), map[string]model.RemoteStatus{}
	s.mu.Unlock()
	gh.offline.Store(true)
	st, err := s.GetRemoteStatus(ctx, id)
	if err != nil || model.CodeOf(st.PR.Error) != model.CodeNetworkError || st.PR.Open != nil {
		t.Fatalf("без сети: %v %+v", err, st.PR)
	}
	if st.PR.LastSuccessAt == nil || !st.PR.LastSuccessAt.Equal(*ok.PR.LastSuccessAt) || st.CI.LastSuccessAt == nil {
		t.Errorf("время последнего успеха потеряно: %+v %+v", st.PR, st.CI)
	}
}

func TestRemoteRef(t *testing.T) { // D-059
	cases := []struct {
		name   string
		branch *model.BranchStatus
		want   string
	}{
		{"ветка", &model.BranchStatus{Name: "main", HeadSHA: "abc"}, "main"},
		{"detached HEAD — коммит", &model.BranchStatus{Detached: true, HeadSHA: "abc"}, "@abc"},
		{"detached без коммита", &model.BranchStatus{Detached: true}, ""},
		{"статус не прочитан", nil, ""},
	}
	for _, c := range cases {
		if got := remoteRef(model.Repository{Branch: c.branch}); got != c.want {
			t.Errorf("%s: %q, ожидалось %q", c.name, got, c.want)
		}
	}
}
