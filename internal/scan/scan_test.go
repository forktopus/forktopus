package scan

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/forktopus/forktopus/internal/gittest"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

func tempRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func paths(t *testing.T, root string, res model.ScanResult) []string {
	t.Helper()
	out := []string{}
	for _, r := range res.Repositories {
		rel, err := filepath.Rel(root, r.Path)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, rel)
	}
	return out
}

func byPath(res model.ScanResult, path string) model.Repository {
	for _, r := range res.Repositories {
		if r.Path == path {
			return r
		}
	}
	return model.Repository{}
}

func run(t *testing.T, roots []model.Root, opts Options) model.ScanResult {
	t.Helper()
	res, err := Run(context.Background(), runner.Exec{}, roots, opts)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestScanFindsRepositories(t *testing.T) {
	root := tempRoot(t)
	gittest.Init(t, mkdir(t, root, "github.com", "org", "shallow"))
	gittest.Init(t, mkdir(t, root, "git.company.local", "group", "sub", "deeper", "Мой проект с пробелом"))
	gittest.Init(t, mkdir(t, root, ".hidden", "repo"))
	mkdir(t, root, "plain", "dir")
	// Нет прав на каталог — ошибка в Issues, остальное сканируется (A04).
	locked := mkdir(t, root, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	res := run(t, []model.Root{{ID: "r", Path: root}, {ID: "missing", Path: filepath.Join(root, "missing")}}, Options{})

	want := []string{".hidden/repo", "git.company.local/group/sub/deeper/Мой проект с пробелом", "github.com/org/shallow"}
	if got := paths(t, root, res); !slices.Equal(got, want) {
		t.Fatalf("найдено %v, want %v", got, want)
	}
	reasons := map[string]model.IssueReason{}
	for _, is := range res.Issues {
		reasons[is.Path] = is.Reason
	}
	if os.Geteuid() != 0 && reasons[locked] != model.IssueNoAccess {
		t.Errorf("нет диагностики для каталога без прав: %+v", res.Issues)
	}
	if reasons[filepath.Join(root, "missing")] != model.IssueNotFound {
		t.Errorf("нет диагностики для отсутствующего корня: %+v", res.Issues)
	}

	// Каталог из SkipPaths не читается — и проблемы с ним нет (D-067).
	res = run(t, []model.Root{{ID: "r", Path: root + "/"}}, Options{SkipPaths: []string{locked, filepath.Join(root, "github.com", "org")}})
	if len(res.Issues) != 0 {
		t.Errorf("пропущенный путь в диагностике: %+v", res.Issues)
	}
	if got := paths(t, root, res); !slices.Equal(got, want[:2]) {
		t.Errorf("с пропуском путей найдено %v", got)
	}
	for _, r := range res.Repositories {
		if r.ID != model.PathID(r.Path) || r.RootID != "r" || r.Error != nil || r.ParentID != "" {
			t.Errorf("запись %+v", r)
		}
		// Статус читается вместе со сканированием.
		if r.Branch == nil || r.Branch.Name != "main" || r.Worktree == nil || r.UpdatedAt.IsZero() {
			t.Errorf("статус не прочитан: %+v", r)
		}
	}
}

func TestRefresh(t *testing.T) {
	root := tempRoot(t)
	repo := gittest.Init(t, mkdir(t, root, "repo"))
	res := run(t, []model.Root{{ID: "r", Path: root}}, Options{})
	found := res.Repositories[0]
	found.ParentID, found.Nesting = "parent", model.NestingNested

	gittest.WriteFile(t, repo, "new.txt", "x\n")
	gittest.Git(t, repo, "remote", "add", "origin", "https://gitlab.com/g/repo.git")
	got := Refresh(context.Background(), runner.Exec{}, found, Options{})
	if got.Worktree == nil || got.Worktree.Untracked != 1 || got.Remote != "origin" || !got.UpdatedAt.After(found.UpdatedAt) {
		t.Errorf("после обновления: %+v", got)
	}
	if got.ID != found.ID || got.ParentID != "parent" || got.Nesting != model.NestingNested {
		t.Errorf("расположение и связи должны сохраниться: %+v", got)
	}

	// Каталог исчез — ошибка у записи, статус сброшен, а не оставлен старым.
	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}
	gone := Refresh(context.Background(), runner.Exec{}, got, Options{})
	if gone.Error == nil || gone.Branch != nil || gone.Worktree != nil {
		t.Errorf("исчезнувший репозиторий: %+v", gone)
	}
}

func TestScanSkipsAndSymlinks(t *testing.T) {
	root := tempRoot(t)
	outside := tempRoot(t)
	gittest.Init(t, mkdir(t, outside, "external"))
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	app := gittest.Init(t, mkdir(t, root, "app"))
	gittest.Init(t, mkdir(t, app, "node_modules", "dep"))
	gittest.Init(t, mkdir(t, root, "custom-skip", "repo"))
	// Bare-репозиторий пропускается молча (D-026).
	gittest.Git(t, root, "init", "-q", "--bare", "remotes/demo.git")

	res := run(t, []model.Root{{ID: "r", Path: root}}, Options{})
	if got := paths(t, root, res); !slices.Equal(got, []string{"app", "custom-skip/repo"}) {
		t.Errorf("по умолчанию: %v", got)
	}
	if len(res.Issues) != 0 {
		t.Errorf("лишняя диагностика: %+v", res.Issues)
	}

	res = run(t, []model.Root{{ID: "r", Path: root}}, Options{SkipDirs: []string{"custom-skip"}})
	if got := paths(t, root, res); !slices.Equal(got, []string{"app", "app/node_modules/dep"}) {
		t.Errorf("со своим списком пропусков: %v", got)
	}
}

func TestScanNestedAndWorktrees(t *testing.T) {
	root := tempRoot(t)
	parent := gittest.Init(t, mkdir(t, root, "parent"))
	nested := gittest.Init(t, mkdir(t, parent, "tools", "nested"))
	inner := gittest.Init(t, mkdir(t, nested, "inner"))
	child := gittest.Clean(t)
	gittest.Git(t, parent, "-c", "protocol.file.allow=always", "submodule", "add", "-q", child, "libs/child")
	gittest.Commit(t, parent, "a.txt", "a", "with submodule")
	// Worktree — .git-файл, отдельная запись со своим путём (A05, A22).
	gittest.Git(t, parent, "worktree", "add", "-q", "-b", "wt", filepath.Join(root, "parent-wt"))

	res := run(t, []model.Root{{ID: "r", Path: root}}, Options{})
	want := []string{"parent", "parent-wt", "parent/libs/child", "parent/tools/nested", "parent/tools/nested/inner"}
	if got := paths(t, root, res); !slices.Equal(got, want) {
		t.Fatalf("найдено %v, want %v", got, want)
	}
	p := byPath(res, parent)
	if sub := byPath(res, filepath.Join(parent, "libs", "child")); sub.ParentID != p.ID || sub.Nesting != model.NestingSubmodule || sub.GitDir == "" {
		t.Errorf("сабмодуль: %+v", sub)
	}
	n := byPath(res, nested)
	if n.ParentID != p.ID || n.Nesting != model.NestingNested {
		t.Errorf("вложенный клон: %+v", n)
	}
	if in := byPath(res, inner); in.ParentID != n.ID || in.Nesting != model.NestingNested {
		t.Errorf("вложенный во вложенный: %+v", in)
	}
	if wt := byPath(res, filepath.Join(root, "parent-wt")); wt.ParentID != "" || wt.ID == p.ID {
		t.Errorf("worktree: %+v", wt)
	}
}

func TestScanIdentityAcrossRoots(t *testing.T) {
	root := tempRoot(t)
	repo := gittest.Init(t, mkdir(t, root, "group", "repo"))
	gittest.Git(t, repo, "remote", "add", "origin", "git@git.home.test:group/repo.git")
	gittest.Git(t, repo, "remote", "add", "upstream", "https://github.com/up/repo.git")

	res := run(t, []model.Root{{ID: "outer", Path: root}, {ID: "inner", Path: filepath.Join(root, "group")}}, Options{
		HostProviders: map[string]model.Provider{"git.home.test": model.ProviderGitea},
		Remotes:       map[string]string{model.PathID(repo): "upstream"},
	})
	if len(res.Repositories) != 1 {
		t.Fatalf("дубликаты: %+v", res.Repositories)
	}
	r := res.Repositories[0]
	if r.RootID != "inner" {
		t.Errorf("корень должен быть самым конкретным: %q", r.RootID)
	}
	if r.Remote != "upstream" || len(r.Remotes) != 2 || r.Remotes[0].Provider != model.ProviderGitea {
		t.Errorf("remote: %q %+v", r.Remote, r.Remotes)
	}
}

func TestScanCancel(t *testing.T) {
	root := tempRoot(t)
	for _, name := range []string{"a", "b", "c", "d"} {
		gittest.Init(t, mkdir(t, root, name))
	}
	ctx, cancel := context.WithCancel(context.Background())
	var seen atomic.Int32
	res, err := Run(ctx, runner.Exec{}, []model.Root{{ID: "r", Path: root}}, Options{
		OnRepository: func(model.Repository) {
			if seen.Add(1) == 1 {
				cancel()
			}
		},
	})
	if model.CodeOf(err) != model.CodeOpCancelled {
		t.Fatalf("ожидалась отмена, получено %v", err)
	}
	// Найденное до отмены не теряется.
	if len(res.Repositories) == 0 {
		t.Error("частичный результат пуст")
	}
}

// Сломанный .git-файл — запись с ошибкой, обход продолжается.
func TestScanBrokenRepositoryDoesNotStopScan(t *testing.T) {
	root := tempRoot(t)
	broken := mkdir(t, root, "broken")
	if err := os.WriteFile(filepath.Join(broken, ".git"), []byte("gitdir: /nonexistent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Init(t, mkdir(t, root, "ok"))
	res := run(t, []model.Root{{ID: "r", Path: root}}, Options{})
	if len(res.Repositories) != 2 {
		t.Fatalf("найдено: %+v", res.Repositories)
	}
	if b := byPath(res, broken); b.Error == nil {
		t.Errorf("ожидалась ошибка у сломанного репозитория: %+v", b)
	}
	if ok := byPath(res, filepath.Join(root, "ok")); ok.Error != nil {
		t.Errorf("исправный репозиторий с ошибкой: %+v", ok)
	}
}

func TestScanTakesRepositoryLock(t *testing.T) {
	root := tempRoot(t)
	gittest.Init(t, mkdir(t, root, "free"))
	gittest.Init(t, mkdir(t, root, "busy"))
	var mu sync.Mutex
	locked := map[string]int{}
	opts := Options{Lock: func(_ context.Context, id string) (func(), error) {
		mu.Lock()
		defer mu.Unlock()
		locked[id]++
		return func() {}, nil
	}}
	res := run(t, []model.Root{{ID: "r", Path: root}}, opts)
	if len(res.Repositories) != 2 || len(locked) != 2 {
		t.Fatalf("блокировка берётся для каждого репозитория: %v, %+v", locked, res.Repositories)
	}

	// Не дождались блокировки — статус не читается, причина в Error.
	busyID := ""
	for _, r := range res.Repositories {
		if filepath.Base(r.Path) == "busy" {
			busyID = r.ID
		}
	}
	opts.Lock = func(_ context.Context, id string) (func(), error) {
		if id == busyID {
			return nil, model.Errorf(model.CodeOpCancelled, "ожидание отменено")
		}
		return func() {}, nil
	}
	res = run(t, []model.Root{{ID: "r", Path: root}}, opts)
	for _, r := range res.Repositories {
		busy := r.ID == busyID
		if busy != (r.Error != nil) || busy != (r.Branch == nil) {
			t.Errorf("%s: error=%v branch=%v", r.Path, r.Error, r.Branch)
		}
	}
}
