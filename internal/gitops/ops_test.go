package gitops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/gittest"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

var r = runner.Exec{}

// snapshot — состояние рабочей копии, которое операции не должны трогать.
func snapshot(t *testing.T, repo string) string {
	t.Helper()
	return gittest.Git(t, repo, "rev-parse", "HEAD") + "\n" + gittest.Git(t, repo, "status", "--porcelain=v1", "--untracked-files=all")
}

func TestFetchUpdatesTrackingRefsOnly(t *testing.T) { // A11
	repo, bare := gittest.Upstream(t)
	gittest.WriteFile(t, repo, "untracked.txt", "local\n")
	before := snapshot(t, repo)
	gittest.PushFromOtherClone(t, bare, "remote.txt", "remote\n", "remote commit")
	remoteHead := gittest.Git(t, bare, "rev-parse", "main")

	if err := Fetch(context.Background(), r, repo, "origin", false); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got := gittest.Git(t, repo, "rev-parse", "origin/main"); got != remoteHead {
		t.Errorf("origin/main = %s, ожидалось %s", got, remoteHead)
	}
	if after := snapshot(t, repo); after != before {
		t.Errorf("fetch изменил рабочую копию:\nдо:\n%s\nпосле:\n%s", before, after)
	}
}

func TestFailedFetchKeepsLastFetchTime(t *testing.T) {
	repo, _ := gittest.Upstream(t)
	gittest.Git(t, repo, "fetch", "-q", "origin")
	head := filepath.Join(repo, ".git", "FETCH_HEAD")
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(head, old, old); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, repo, "remote", "add", "offline", "http://127.0.0.1:1/repo.git")
	if err := Fetch(context.Background(), r, repo, "offline", false); err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if fi, err := os.Stat(head); err != nil || !fi.ModTime().Equal(old) {
		t.Errorf("время последнего fetch изменилось после сбоя: %v %v", fi.ModTime(), err)
	}
}

func TestFetchKeepsStaleTrackingRefs(t *testing.T) { // D-033
	repo, bare := gittest.Upstream(t)
	gittest.Git(t, repo, "push", "-q", "origin", "main:gone")
	gittest.Git(t, repo, "fetch", "-q", "origin")
	gittest.Git(t, bare, "branch", "-D", "gone")
	gittest.Git(t, repo, "config", "fetch.prune", "true")

	if err := Fetch(context.Background(), r, repo, "origin", false); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if _, err := gittest.TryGit(repo, "rev-parse", "--verify", "-q", "refs/remotes/origin/gone"); err != nil {
		t.Error("fetch подрезал origin/gone, хотя prune выключен")
	}
}

func TestFetchPrune(t *testing.T) { // D-057
	repo, bare := gittest.Upstream(t)
	gittest.Git(t, repo, "push", "-q", "origin", "main:gone")
	gittest.Git(t, repo, "branch", "-q", "gone", "origin/gone")
	gittest.Git(t, bare, "branch", "-D", "gone")

	if err := Fetch(context.Background(), r, repo, "origin", true); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if _, err := gittest.TryGit(repo, "rev-parse", "--verify", "-q", "refs/remotes/origin/gone"); err == nil {
		t.Error("origin/gone остался после fetch --prune")
	}
	if _, err := gittest.TryGit(repo, "rev-parse", "--verify", "-q", "refs/heads/gone"); err != nil {
		t.Error("prune удалил локальную ветку gone")
	}
}

func TestFetchErrors(t *testing.T) {
	repo := gittest.Clean(t)
	gittest.Git(t, repo, "remote", "add", "missing", filepath.Join(t.TempDir(), "no-such.git"))
	err := Fetch(context.Background(), r, repo, "missing", false)
	if code := model.CodeOf(err); code != model.CodeOther {
		t.Errorf("отсутствующий репозиторий: %v", err)
	}
	if model.AsError(err).Message == "" {
		t.Error("нет причины")
	}

	// Порт 1 на localhost закрыт: соединение отклоняется сразу.
	gittest.Git(t, repo, "remote", "add", "offline", "http://127.0.0.1:1/repo.git")
	if code := model.CodeOf(Fetch(context.Background(), r, repo, "offline", false)); code != model.CodeNetworkError {
		t.Errorf("сервер недоступен: ожидался NETWORK_ERROR, получено %s", code)
	}

	// Неудачный fetch не выглядит свежим: FETCH_HEAD не появился (06.4).
	if _, err := os.Stat(filepath.Join(repo, ".git", "FETCH_HEAD")); !os.IsNotExist(err) {
		t.Errorf("после неудачного fetch остался FETCH_HEAD: %v", err)
	}

	if code := model.CodeOf(Fetch(context.Background(), r, repo, "--upload-pack=touch", false)); code != model.CodeInvalidInput {
		t.Errorf("имя remote как опция: ожидался INVALID_INPUT, получено %s", code)
	}
}

func TestPullFastForward(t *testing.T) { // A12
	repo, bare := gittest.Upstream(t)
	gittest.PushFromOtherClone(t, bare, "remote.txt", "remote\n", "remote commit")
	remoteHead := gittest.Git(t, bare, "rev-parse", "main")
	// Конфиг пользователя не должен превращать ff-pull в rebase или merge.
	gittest.Git(t, repo, "config", "pull.rebase", "true")
	gittest.Git(t, repo, "config", "pull.ff", "false")

	if err := Pull(context.Background(), r, repo); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if head := gittest.Git(t, repo, "rev-parse", "HEAD"); head != remoteHead {
		t.Errorf("HEAD = %s, ожидалось %s", head, remoteHead)
	}
	if merges := gittest.Git(t, repo, "rev-list", "--merges", "--count", "HEAD"); merges != "0" {
		t.Errorf("появился merge-коммит: %s", merges)
	}
}

func TestPullDivergedFails(t *testing.T) {
	repo, bare := gittest.Upstream(t)
	gittest.PushFromOtherClone(t, bare, "remote.txt", "remote\n", "remote commit")
	gittest.Commit(t, repo, "local.txt", "local\n", "local commit")
	before := gittest.Git(t, repo, "rev-parse", "HEAD")

	err := Pull(context.Background(), r, repo)
	if code := model.CodeOf(err); code != model.CodeDiverged {
		t.Fatalf("ожидался DIVERGED, получено %v", err)
	}
	if after := gittest.Git(t, repo, "rev-parse", "HEAD"); after != before {
		t.Error("HEAD изменился после неудачного pull")
	}
	if out := gittest.Git(t, repo, "status", "--porcelain"); out != "" {
		t.Errorf("рабочая копия изменилась:\n%s", out)
	}
}

func TestSwitch(t *testing.T) {
	repo := gittest.Clean(t)
	gittest.Git(t, repo, "branch", "feature")
	if err := Switch(context.Background(), r, repo, "feature"); err != nil {
		t.Fatalf("switch: %v", err)
	}
	if b := gittest.Git(t, repo, "branch", "--show-current"); b != "feature" {
		t.Errorf("текущая ветка %s", b)
	}

	// Одноимённая remote-tracking ветка не превращается в новую локальную.
	up, _ := gittest.Upstream(t)
	gittest.Git(t, up, "push", "-q", "origin", "main:remote-only")
	gittest.Git(t, up, "fetch", "-q", "origin")
	if err := Switch(context.Background(), r, up, "remote-only"); err == nil {
		t.Error("switch создал ветку из origin/remote-only")
	}
	if _, err := gittest.TryGit(up, "rev-parse", "--verify", "-q", "refs/heads/remote-only"); err == nil {
		t.Error("появилась локальная ветка remote-only")
	}
}

func TestSwitchRejectsBadNames(t *testing.T) {
	repo := gittest.Clean(t)
	for _, name := range []string{"", "-f", "--force", "@{-1}", "a..b", "bad name", "ветка~1"} {
		if code := model.CodeOf(Switch(context.Background(), r, repo, name)); code != model.CodeInvalidInput {
			t.Errorf("%q: ожидался INVALID_INPUT, получено %s", name, code)
		}
	}
}

func TestSwitchDoesNotOverwriteUntracked(t *testing.T) {
	// Даже если предусловия пропустили бы, Git сам не перезапишет файлы.
	repo := gittest.Clean(t)
	gittest.Git(t, repo, "checkout", "-q", "-b", "feature")
	gittest.Commit(t, repo, "file.txt", "feature\n", "feature file")
	gittest.Git(t, repo, "checkout", "-q", "main")
	gittest.WriteFile(t, repo, "file.txt", "local\n")

	err := Switch(context.Background(), r, repo, "feature")
	if code := model.CodeOf(err); code != model.CodeDirtyWorktree {
		t.Fatalf("ожидался DIRTY_WORKTREE, получено %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(repo, "file.txt")); string(b) != "local\n" {
		t.Errorf("локальный файл изменён: %q", b)
	}
}

func TestHeadReachable(t *testing.T) {
	repo := gittest.Detached(t)
	ok, err := HeadReachable(context.Background(), r, repo)
	if err != nil || !ok {
		t.Fatalf("коммит из main: ok=%v err=%v", ok, err)
	}
	gittest.Commit(t, repo, "orphan.txt", "x\n", "detached commit")
	ok, err = HeadReachable(context.Background(), r, repo)
	if err != nil || ok {
		t.Fatalf("коммит вне веток: ok=%v err=%v", ok, err)
	}
}

func TestGitError(t *testing.T) {
	cases := []struct {
		stderr string
		want   model.ErrorCode
	}{
		{"fatal: could not read Username for 'https://github.com': terminal prompts disabled", model.CodeAuthRequired},
		{"git@github.com: Permission denied (publickey).\r\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.", model.CodeAuthRequired},
		{"remote: HTTP Basic: Access denied\nfatal: Authentication failed for 'https://gitlab.com/x/y.git/'", model.CodeAuthRequired},
		{"fatal: unable to access 'https://example.invalid/x.git/': Could not resolve host: example.invalid", model.CodeNetworkError},
		{"ssh: connect to host example.com port 22: Connection timed out\r\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights", model.CodeNetworkError},
		{"hint: Diverging branches can't be fast-forwarded\nfatal: Not possible to fast-forward, aborting.", model.CodeDiverged},
		{"error: Your local changes to the following files would be overwritten by checkout:\n\tREADME.md", model.CodeDirtyWorktree},
		{"fatal: 'nope' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights", model.CodeOther},
		{"fatal: something else", model.CodeOther},
	}
	for _, c := range cases {
		e := gitError(runner.Result{Stderr: []byte(c.stderr), ExitCode: 128})
		if e.Code != c.want || e.Message == "" || e.ExitCode != 128 {
			t.Errorf("%q: %+v, ожидался %s", c.stderr, e, c.want)
		}
	}
	e := gitError(runner.Result{Stderr: []byte("fatal: unable to access 'https://user:secret@host/x.git/': SSL certificate problem"), ExitCode: 128})
	if e.Code != model.CodeNetworkError || strings.Contains(e.Message, "secret") {
		t.Errorf("токен в сообщении или неверный код: %+v", e)
	}
}

func TestLocks(t *testing.T) {
	var l Locks
	unlock, err := l.TryLock("a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.TryLock("a"); model.CodeOf(err) != model.CodeRepoBusy {
		t.Fatalf("вторая операция: ожидался REPO_BUSY, получено %v", err)
	}
	if u, err := l.TryLock("b"); err != nil {
		t.Fatalf("другой репозиторий заблокирован: %v", err)
	} else {
		u()
	}

	// Чтение статуса ждёт окончания операции.
	got := make(chan time.Time, 1)
	go func() {
		u, err := l.Lock(context.Background(), "a")
		if err == nil {
			got <- time.Now()
			u()
		}
	}()
	time.Sleep(50 * time.Millisecond)
	released := time.Now()
	unlock()
	select {
	case at := <-got:
		if at.Before(released) {
			t.Error("Lock не дождался освобождения")
		}
	case <-time.After(time.Second):
		t.Fatal("Lock не получил блокировку после освобождения")
	}

	// Отмена ожидания.
	unlock, _ = l.TryLock("a")
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := l.Lock(ctx, "a"); model.CodeOf(err) != model.CodeOpCancelled {
		t.Fatalf("ожидался OP_CANCELLED, получено %v", err)
	}
}

func TestGitErrorNamesTheCause(t *testing.T) {
	e := gitError(runner.Result{Stderr: []byte("ssh: Could not resolve hostname gitea.local: Name or service not known\r\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists."), ExitCode: 128})
	if e.Code != model.CodeNetworkError || !strings.Contains(e.Message, "Could not resolve hostname gitea.local") {
		t.Errorf("причина потерялась: %+v", e)
	}
}

// Пути с пробелами и кириллицей и кириллическое имя ветки передаются Git
// аргументами без shell (A29).
func TestOpsInPathWithSpacesAndCyrillic(t *testing.T) {
	ctx := context.Background()
	base := filepath.Join(t.TempDir(), "Мои проекты $HOME; 'кавычки'")
	bare := filepath.Join(base, "удалённый репо.git")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, base, "init", "-q", "--bare", "-b", "main", bare)
	repo := gittest.Init(t, filepath.Join(base, "локальный репо"))
	gittest.Git(t, repo, "remote", "add", "origin", bare)
	gittest.Git(t, repo, "push", "-q", "-u", "origin", "main")
	gittest.PushFromOtherClone(t, bare, "новый файл.txt", "x\n", "коммит")
	remoteHead := gittest.Git(t, bare, "rev-parse", "main")

	if err := Fetch(ctx, r, repo, "origin", false); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if err := Pull(ctx, r, repo); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if head := gittest.Git(t, repo, "rev-parse", "HEAD"); head != remoteHead {
		t.Errorf("HEAD = %s, ожидалось %s", head, remoteHead)
	}
	if err := Switch(ctx, r, repo, "фича с пробелом"); model.CodeOf(err) != model.CodeInvalidInput {
		t.Errorf("имя с пробелом: %v", err)
	}
	gittest.Git(t, repo, "branch", "фича")
	if err := Switch(ctx, r, repo, "фича"); err != nil {
		t.Fatalf("switch: %v", err)
	}
	if cur := gittest.Git(t, repo, "branch", "--show-current"); cur != "фича" {
		t.Errorf("текущая ветка %q", cur)
	}
}
