// Package gittest создаёт во временных каталогах Git-репозитории в нужных
// состояниях для тестов ядра. Глобальная и системная конфигурация Git
// пользователя не используется, поэтому результат не зависит от машины.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// env изолирует git от ~/.gitconfig и /etc/gitconfig (подпись коммитов,
// хуки, алиасы) и задаёт автора коммитов.
var env = []string{
	"GIT_CONFIG_GLOBAL=" + os.DevNull,
	"GIT_CONFIG_NOSYSTEM=1",
	"GIT_AUTHOR_NAME=Forktopus Test",
	"GIT_AUTHOR_EMAIL=test@example.invalid",
	"GIT_COMMITTER_NAME=Forktopus Test",
	"GIT_COMMITTER_EMAIL=test@example.invalid",
	"GIT_TERMINAL_PROMPT=0",
	"LC_ALL=C",
}

// Git выполняет git в каталоге dir и возвращает stdout без завершающего
// перевода строки. Ошибка завершает тест.
func Git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	out, err := run(dir, args...)
	if err != nil {
		t.Fatalf("git %s (в %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimRight(out, "\n")
}

// TryGit выполняет git и возвращает вывод и ошибку, не завершая тест.
// Нужен для команд, которые завершаются с ошибкой намеренно (конфликт слияния).
func TryGit(dir string, args ...string) (string, error) {
	return run(dir, args...)
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// WriteFile записывает файл относительно корня репозитория.
func WriteFile(t testing.TB, repo, name, content string) {
	t.Helper()
	path := filepath.Join(repo, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Commit записывает файл, индексирует и коммитит его.
func Commit(t testing.TB, repo, name, content, message string) {
	t.Helper()
	WriteFile(t, repo, name, content)
	Git(t, repo, "add", "--", name)
	Git(t, repo, "commit", "-q", "-m", message)
}

// Init создаёт репозиторий в dir с веткой main и одним коммитом.
func Init(t testing.TB, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	Git(t, dir, "init", "-q", "-b", "main")
	Commit(t, dir, "README.md", "fixture\n", "initial commit")
	return dir
}

// Clean — чистый репозиторий без remote.
func Clean(t testing.TB) string {
	t.Helper()
	return Init(t, filepath.Join(t.TempDir(), "clean"))
}

// Dirty — репозиторий с одним staged, одним unstaged и одним untracked файлом.
func Dirty(t testing.TB) string {
	t.Helper()
	repo := Init(t, filepath.Join(t.TempDir(), "dirty"))
	WriteFile(t, repo, "staged.txt", "staged\n")
	Git(t, repo, "add", "--", "staged.txt")
	WriteFile(t, repo, "README.md", "fixture\nunstaged edit\n")
	WriteFile(t, repo, "untracked.txt", "untracked\n")
	return repo
}

// Upstream — клон bare-репозитория с настроенным upstream origin/main.
// Возвращает путь клона и bare-репозитория.
func Upstream(t testing.TB) (repo, bare string) {
	t.Helper()
	base := t.TempDir()
	bare = filepath.Join(base, "remote.git")
	Git(t, base, "init", "-q", "--bare", "-b", "main", bare)
	repo = Init(t, filepath.Join(base, "local"))
	Git(t, repo, "remote", "add", "origin", bare)
	Git(t, repo, "push", "-q", "-u", "origin", "main")
	return repo, bare
}

// Ahead — ветка main на один коммит впереди origin/main.
func Ahead(t testing.TB) string {
	t.Helper()
	repo, _ := Upstream(t)
	Commit(t, repo, "local.txt", "local\n", "local commit")
	return repo
}

// Behind — ветка main на один коммит позади origin/main (fetch уже выполнен,
// рабочая копия не обновлена). Возвращает также bare-репозиторий.
func Behind(t testing.TB) (repo, bare string) {
	t.Helper()
	repo, bare = Upstream(t)
	PushFromOtherClone(t, bare, "remote.txt", "remote\n", "remote commit")
	Git(t, repo, "fetch", "-q", "origin")
	return repo, bare
}

// PushFromOtherClone добавляет коммит в main bare-репозитория через
// отдельный временный клон, не трогая тестируемую рабочую копию.
func PushFromOtherClone(t testing.TB, bare, name, content, message string) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	Git(t, filepath.Dir(other), "clone", "-q", bare, other)
	Commit(t, other, name, content, message)
	Git(t, other, "push", "-q", "origin", "main")
}

// Detached — репозиторий в состоянии detached HEAD.
func Detached(t testing.TB) string {
	t.Helper()
	repo := Init(t, filepath.Join(t.TempDir(), "detached"))
	Git(t, repo, "checkout", "-q", "--detach", "HEAD")
	return repo
}

// Worktree — основной репозиторий и дополнительное рабочее дерево на ветке
// feature. В дополнительном дереве .git — файл, а не каталог.
func Worktree(t testing.TB) (main, worktree string) {
	t.Helper()
	main = Init(t, filepath.Join(t.TempDir(), "main"))
	worktree = filepath.Join(t.TempDir(), "worktree")
	Git(t, main, "worktree", "add", "-q", "-b", "feature", worktree)
	return main, worktree
}

// MergeConflict — репозиторий с незавершённым слиянием и конфликтом в README.md.
func MergeConflict(t testing.TB) string {
	t.Helper()
	repo := Init(t, filepath.Join(t.TempDir(), "conflict"))
	Git(t, repo, "checkout", "-q", "-b", "other")
	Commit(t, repo, "README.md", "other side\n", "other change")
	Git(t, repo, "checkout", "-q", "main")
	Commit(t, repo, "README.md", "main side\n", "main change")
	if out, err := TryGit(repo, "merge", "other"); err == nil {
		t.Fatalf("ожидался конфликт слияния, но merge прошёл:\n%s", out)
	}
	return repo
}
