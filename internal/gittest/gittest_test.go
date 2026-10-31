package gittest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// branchHeader возвращает строку "## ..." из git status -sb.
func branchHeader(t *testing.T, repo string) string {
	t.Helper()
	return strings.SplitN(Git(t, repo, "status", "-sb", "--porcelain=v1"), "\n", 2)[0]
}

func TestFixtures(t *testing.T) {
	if got := Git(t, Clean(t), "status", "--porcelain"); got != "" {
		t.Errorf("clean: есть изменения %q", got)
	}

	dirty := Git(t, Dirty(t), "status", "--porcelain", "--untracked-files=all")
	for _, want := range []string{"A  staged.txt", " M README.md", "?? untracked.txt"} {
		if !strings.Contains(dirty, want) {
			t.Errorf("dirty: нет %q в\n%s", want, dirty)
		}
	}

	if got := branchHeader(t, Ahead(t)); got != "## main...origin/main [ahead 1]" {
		t.Errorf("ahead: %q", got)
	}

	behind, _ := Behind(t)
	if got := branchHeader(t, behind); got != "## main...origin/main [behind 1]" {
		t.Errorf("behind: %q", got)
	}

	if got := branchHeader(t, Detached(t)); got != "## HEAD (no branch)" {
		t.Errorf("detached: %q", got)
	}

	_, wt := Worktree(t)
	info, err := os.Stat(filepath.Join(wt, ".git"))
	if err != nil || info.IsDir() {
		t.Errorf("worktree: .git должен быть файлом: %v", err)
	}
	if got := branchHeader(t, wt); got != "## feature" {
		t.Errorf("worktree: %q", got)
	}

	conflict := MergeConflict(t)
	if got := Git(t, conflict, "status", "--porcelain"); got != "UU README.md" {
		t.Errorf("conflict: %q", got)
	}
	if _, err := os.Stat(filepath.Join(conflict, ".git", "MERGE_HEAD")); err != nil {
		t.Errorf("conflict: нет MERGE_HEAD: %v", err)
	}
}
