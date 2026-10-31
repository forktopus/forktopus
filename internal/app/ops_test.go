package app

import (
	"context"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/gittest"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
	"github.com/forktopus/forktopus/internal/settings"
)

// recorder запоминает подкоманды git, чтобы проверить, что блокировка
// сработала до вызова Git (A13).
type recorder struct {
	mu   sync.Mutex
	subs []string
}

func (r *recorder) Run(ctx context.Context, c runner.Command) (runner.Result, error) {
	r.mu.Lock()
	if len(c.Args) > 0 {
		r.subs = append(r.subs, c.Args[0])
	}
	r.mu.Unlock()
	return runner.Exec{}.Run(ctx, c)
}

func (r *recorder) ran(sub string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Contains(r.subs, sub)
}

func (r *recorder) reset() {
	r.mu.Lock()
	r.subs = nil
	r.mu.Unlock()
}

// opService — сервис с одним репозиторием: корень — каталог над repo.
func opService(t *testing.T, repo string) (*AppService, *recorder, string) {
	t.Helper()
	rec := &recorder{}
	s, err := NewAppService(settings.NewStore(filepath.Join(t.TempDir(), "config.json")), rec, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRoot(filepath.Dir(repo)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ScanRoots(context.Background()); err != nil {
		t.Fatal(err)
	}
	id := model.PathID(mustEval(t, repo))
	if _, err := s.GetRepo(id); err != nil {
		t.Fatal(err)
	}
	rec.reset()
	return s, rec, id
}

// shown — отпечаток состояния, которое сейчас видит UI.
func shown(t *testing.T, s *AppService, id string) model.OpExpect {
	t.Helper()
	r, err := s.GetRepo(id)
	if err != nil || r.Branch == nil {
		t.Fatalf("GetRepo: %+v err=%v", r, err)
	}
	return model.OpExpect{Branch: r.Branch.Name, HeadSHA: r.Branch.HeadSHA}
}

func TestFetchAndPull(t *testing.T) { // A11, A12
	ctx := context.Background()
	repo, bare := gittest.Behind(t)
	s, _, id := opService(t, repo)
	gittest.PushFromOtherClone(t, bare, "second.txt", "2\n", "second remote commit")

	res, err := s.FetchRepo(ctx, id, shown(t, s, id))
	if err != nil || res.Error != nil {
		t.Fatalf("fetch: %+v err=%v", res, err)
	}
	if res.Target != "origin" || res.Repo.Branch == nil || *res.Repo.Branch.Behind != 2 || res.Repo.Branch.LastFetchAt == nil {
		t.Errorf("после fetch: %+v", res.Repo.Branch)
	}

	res, err = s.FastForwardPull(ctx, id, shown(t, s, id))
	if err != nil || res.Error != nil {
		t.Fatalf("pull: %+v err=%v", res, err)
	}
	if b := res.Repo.Branch; *b.Behind != 0 || *b.Ahead != 0 || b.HeadSHA != gittest.Git(t, bare, "rev-parse", "main") {
		t.Errorf("после pull: %+v", b)
	}
	if got, _ := s.GetRepo(id); got.Branch.HeadSHA != res.Repo.Branch.HeadSHA {
		t.Error("свежий статус не попал в список")
	}

	ops := s.ListOperations()
	if len(ops) != 2 || ops[0].Op != model.OperationPull || ops[1].Op != model.OperationFetch || ops[0].Error != nil {
		t.Errorf("журнал: %+v", ops)
	}
}

func TestPullBlockedBeforeGit(t *testing.T) { // A13
	ctx := context.Background()
	repo, _ := gittest.Behind(t)
	gittest.WriteFile(t, repo, "README.md", "local edit\n")
	s, rec, id := opService(t, repo)
	head := gittest.Git(t, repo, "rev-parse", "HEAD")

	res, err := s.FastForwardPull(ctx, id, shown(t, s, id))
	if err != nil || res.Error == nil || res.Error.Code != model.CodeDirtyWorktree {
		t.Fatalf("ожидался DIRTY_WORKTREE: %+v err=%v", res, err)
	}
	if rec.ran("pull") || rec.ran("fetch") || rec.ran("merge") {
		t.Errorf("git вызван несмотря на блокировку: %v", rec.subs)
	}
	if gittest.Git(t, repo, "rev-parse", "HEAD") != head || res.Repo.Worktree.Unstaged != 1 {
		t.Error("рабочая копия изменилась")
	}
	if ops := s.ListOperations(); len(ops) != 1 || ops[0].Error == nil || ops[0].Error.Code != model.CodeDirtyWorktree {
		t.Errorf("блокировка не попала в журнал: %+v", ops)
	}
}

func TestSwitchBranch(t *testing.T) { // A14, A15
	ctx := context.Background()
	main, worktree := gittest.Worktree(t)
	gittest.Git(t, main, "branch", "free")
	s, rec, id := opService(t, main)

	// Ветка занята другой рабочей копией.
	res, _ := s.SwitchBranch(ctx, id, "feature", shown(t, s, id))
	if res.Error == nil || res.Error.Code != model.CodeBranchInWorktree || rec.ran("switch") {
		t.Errorf("ветка в другом worktree: %+v, git: %v", res.Error, rec.subs)
	}
	if b := gittest.Git(t, worktree, "branch", "--show-current"); b != "feature" {
		t.Errorf("второе рабочее дерево тронуто: %s", b)
	}

	// Локальные изменения блокируют switch и сохраняются.
	gittest.WriteFile(t, main, "README.md", "local edit\n")
	res, _ = s.SwitchBranch(ctx, id, "free", shown(t, s, id))
	if res.Error == nil || res.Error.Code != model.CodeDirtyWorktree || rec.ran("switch") {
		t.Errorf("dirty: %+v", res.Error)
	}
	gittest.Git(t, main, "checkout", "--", "README.md")

	// Несуществующая ветка и имя-опция — до вызова Git.
	for _, name := range []string{"missing", "--force", ""} {
		res, _ = s.SwitchBranch(ctx, id, name, shown(t, s, id))
		if res.Error == nil || res.Error.Code != model.CodeInvalidInput || rec.ran("switch") {
			t.Errorf("%q: %+v", name, res.Error)
		}
	}

	res, _ = s.SwitchBranch(ctx, id, "free", shown(t, s, id))
	if res.Error != nil || res.Repo.Branch.Name != "free" {
		t.Fatalf("switch на чистом дереве: %+v", res)
	}
}

func TestSwitchFromDetached(t *testing.T) { // D-034
	ctx := context.Background()
	repo := gittest.Detached(t)
	s, _, id := opService(t, repo)

	res, _ := s.SwitchBranch(ctx, id, "main", shown(t, s, id))
	if res.Error != nil || res.Repo.Branch.Name != "main" {
		t.Fatalf("коммит из main: %+v", res)
	}

	gittest.Git(t, repo, "checkout", "-q", "--detach", "HEAD")
	gittest.Commit(t, repo, "orphan.txt", "x\n", "detached commit")
	if _, err := s.RefreshRepo(ctx, id); err != nil {
		t.Fatal(err)
	}
	head := gittest.Git(t, repo, "rev-parse", "HEAD")
	res, _ = s.SwitchBranch(ctx, id, "main", shown(t, s, id))
	if res.Error == nil || res.Error.Code != model.CodeDetachedUnreachable || gittest.Git(t, repo, "rev-parse", "HEAD") != head {
		t.Errorf("коммит вне веток: %+v", res.Error)
	}
}

func TestOpStateChanged(t *testing.T) { // D-032
	ctx := context.Background()
	repo, _ := gittest.Behind(t)
	s, rec, id := opService(t, repo)
	seen := shown(t, s, id)
	gittest.Commit(t, repo, "terminal.txt", "x\n", "commit from terminal")

	res, err := s.FastForwardPull(ctx, id, seen)
	if err != nil || res.Error == nil || res.Error.Code != model.CodeStateChanged || rec.ran("pull") {
		t.Fatalf("ожидался STATE_CHANGED: %+v err=%v", res.Error, err)
	}
	if res.Repo.Branch.HeadSHA != gittest.Git(t, repo, "rev-parse", "HEAD") {
		t.Error("UI не получил свежий статус")
	}
	if _, err := s.FetchRepo(ctx, "nope", seen); model.CodeOf(err) != model.CodeInvalidInput {
		t.Errorf("неизвестный репозиторий: %v", err)
	}
}

func TestOpBusy(t *testing.T) { // D-031
	ctx := context.Background()
	repo, _ := gittest.Upstream(t)
	s, rec, id := opService(t, repo)

	unlock, err := s.locks.TryLock(id) // «идёт другая операция»
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	res, _ := s.FetchRepo(ctx, id, shown(t, s, id))
	if res.Error == nil || res.Error.Code != model.CodeRepoBusy || rec.ran("fetch") || time.Since(start) > time.Second {
		t.Errorf("ожидался мгновенный REPO_BUSY: %+v", res.Error)
	}

	// Обновление статуса ждёт конца операции.
	done := make(chan error, 1)
	go func() {
		_, err := s.RefreshRepo(ctx, id)
		done <- err
	}()
	select {
	case <-done:
		t.Fatal("RefreshRepo не дождался операции")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RefreshRepo завис после окончания операции")
	}
}
