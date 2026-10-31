package gitlocal

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/gittest"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

func z(records ...string) []byte { return []byte(strings.Join(records, "\x00") + "\x00") }

func TestParseStatus(t *testing.T) {
	const sha = "1111111111111111111111111111111111111111"
	tests := []struct {
		name   string
		out    []byte
		branch model.BranchStatus
		wt     model.WorktreeStatus
		ab     *[2]int // ahead, behind
	}{
		{
			name:   "чисто, с upstream",
			out:    z("# branch.oid "+sha, "# branch.head main", "# branch.upstream origin/main", "# branch.ab +0 -0"),
			branch: model.BranchStatus{Name: "main", HeadSHA: sha, Upstream: "origin/main"},
			ab:     &[2]int{0, 0},
		},
		{
			name:   "ahead и behind",
			out:    z("# branch.oid "+sha, "# branch.head feature/x", "# branch.upstream origin/feature/x", "# branch.ab +2 -3"),
			branch: model.BranchStatus{Name: "feature/x", HeadSHA: sha, Upstream: "origin/feature/x"},
			ab:     &[2]int{2, 3},
		},
		{
			name:   "upstream пропал",
			out:    z("# branch.oid "+sha, "# branch.head main", "# branch.upstream origin/main"),
			branch: model.BranchStatus{Name: "main", HeadSHA: sha, Upstream: "origin/main", UpstreamGone: true},
		},
		{
			name:   "detached",
			out:    z("# branch.oid "+sha, "# branch.head (detached)"),
			branch: model.BranchStatus{Detached: true, HeadSHA: sha},
		},
		{
			name:   "без коммитов",
			out:    z("# branch.oid (initial)", "# branch.head main", "? new file.txt"),
			branch: model.BranchStatus{Name: "main", Unborn: true},
			wt:     model.WorktreeStatus{Untracked: 1},
		},
		{
			name: "staged, unstaged, оба сразу, untracked",
			out: z("# branch.oid "+sha, "# branch.head main",
				"1 A. N... 000000 100644 100644 0000000000000000000000000000000000000000 "+sha+" staged.txt",
				"1 .M N... 100644 100644 100644 "+sha+" "+sha+" README.md",
				"1 MM N... 100644 100644 100644 "+sha+" "+sha+" both.txt",
				"? untracked.txt", "? dir/с пробелом.txt"),
			branch: model.BranchStatus{Name: "main", HeadSHA: sha},
			wt:     model.WorktreeStatus{Staged: 2, Unstaged: 2, Untracked: 2},
		},
		{
			// Исходный путь переименования идёт отдельной записью и может выглядеть как «? …».
			name: "переименование: второй путь после NUL не считается",
			out: z("# branch.oid "+sha, "# branch.head main",
				"2 R. N... 100644 100644 100644 "+sha+" "+sha+" R100 new name.txt", "? старое имя.txt",
				"2 C. N... 100644 100644 100644 "+sha+" "+sha+" C75 copy.txt", "1 fake"),
			branch: model.BranchStatus{Name: "main", HeadSHA: sha},
			wt:     model.WorktreeStatus{Staged: 2},
		},
		{
			name: "все коды unmerged",
			out: z("# branch.oid "+sha, "# branch.head main",
				"u DD N... 100644 000000 000000 000000 "+sha+" "+sha+" "+sha+" a",
				"u AU N... 000000 100644 000000 100644 "+sha+" "+sha+" "+sha+" b",
				"u UD N... 100644 100644 000000 000000 "+sha+" "+sha+" "+sha+" c",
				"u UA N... 000000 000000 100644 100644 "+sha+" "+sha+" "+sha+" d",
				"u DU N... 100644 000000 100644 100644 "+sha+" "+sha+" "+sha+" e",
				"u AA N... 000000 100644 100644 100644 "+sha+" "+sha+" "+sha+" f",
				"u UU N... 100644 100644 100644 100644 "+sha+" "+sha+" "+sha+" g"),
			branch: model.BranchStatus{Name: "main", HeadSHA: sha},
			wt:     model.WorktreeStatus{Conflicts: 7},
		},
		{
			name: "сабмодуль с новыми коммитами и изменениями",
			out: z("# branch.oid "+sha, "# branch.head main",
				"1 .M SCM. 160000 160000 160000 "+sha+" "+sha+" libs/sub"),
			branch: model.BranchStatus{Name: "main", HeadSHA: sha},
			wt:     model.WorktreeStatus{Unstaged: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := parseStatus(tt.out)
			got := st.Branch
			if tt.ab == nil {
				if got.Ahead != nil || got.Behind != nil {
					t.Fatalf("ahead/behind должны отсутствовать, получено %v/%v", got.Ahead, got.Behind)
				}
			} else if got.Ahead == nil || got.Behind == nil || *got.Ahead != tt.ab[0] || *got.Behind != tt.ab[1] {
				t.Fatalf("ahead/behind: ожидалось %v, получено %v/%v", *tt.ab, got.Ahead, got.Behind)
			}
			got.Ahead, got.Behind = nil, nil
			if got != tt.branch {
				t.Errorf("ветка:\n получено  %+v\n ожидалось %+v", got, tt.branch)
			}
			if st.Worktree != tt.wt {
				t.Errorf("изменения: получено %+v, ожидалось %+v", st.Worktree, tt.wt)
			}
		})
	}
}

func readStatus(t *testing.T, dir string) Status {
	t.Helper()
	st, err := ReadStatus(context.Background(), runner.Exec{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func intp(p *int) string {
	if p == nil {
		return "nil"
	}
	return strconv.Itoa(*p)
}

func TestReadStatusClean(t *testing.T) { // A06
	repo := gittest.Clean(t)
	st := readStatus(t, repo)
	if st.Worktree != (model.WorktreeStatus{}) {
		t.Errorf("чистый репозиторий: %+v", st.Worktree)
	}
	b := st.Branch
	if b.Name != "main" || b.Detached || b.HeadSHA != gittest.Git(t, repo, "rev-parse", "HEAD") {
		t.Errorf("ветка: %+v", b)
	}
	// Нет upstream — ahead/behind не выдумываются (A08), fetch не выполнялся.
	if b.Upstream != "" || b.Ahead != nil || b.Behind != nil || b.LastFetchAt != nil {
		t.Errorf("без upstream и fetch: %+v", b)
	}
}

func TestReadStatusDirty(t *testing.T) { // A07
	st := readStatus(t, gittest.Dirty(t))
	want := model.WorktreeStatus{Staged: 1, Unstaged: 1, Untracked: 1}
	if st.Worktree != want {
		t.Errorf("получено %+v, ожидалось %+v", st.Worktree, want)
	}
}

func TestReadStatusDoesNotTouchIndex(t *testing.T) {
	repo := gittest.Clean(t)
	index := filepath.Join(repo, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	// Файл с новым mtime и прежним содержимым: обычный `git status` перехэширует
	// его и перепишет stat-кэш индекса. Чтение статуса этого делать не должно.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(repo, "README.md"), later, later); err != nil {
		t.Fatal(err)
	}
	readStatus(t, repo)
	after, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("индекс изменён при чтении статуса")
	}
}

func TestReadStatusDetached(t *testing.T) { // A10
	repo := gittest.Detached(t)
	b := readStatus(t, repo).Branch
	if !b.Detached || b.Name != "" || b.HeadSHA != gittest.Git(t, repo, "rev-parse", "HEAD") {
		t.Errorf("detached: %+v", b)
	}
}

func TestReadStatusUnborn(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "init", "-q", "-b", "trunk")
	b := readStatus(t, dir).Branch
	if !b.Unborn || b.Name != "trunk" || b.HeadSHA != "" || b.Detached {
		t.Errorf("пустой репозиторий: %+v", b)
	}
	branches, err := Branches(context.Background(), runner.Exec{}, dir)
	if err != nil || len(branches) != 0 {
		t.Errorf("ветки пустого репозитория: %v, %v", branches, err)
	}
}

func TestReadStatusMergeConflict(t *testing.T) {
	st := readStatus(t, gittest.MergeConflict(t))
	if st.Worktree.Conflicts != 1 || st.Worktree.Operation != "merge" {
		t.Errorf("конфликт слияния: %+v", st.Worktree)
	}
}

func TestReadStatusOperations(t *testing.T) {
	repo := gittest.Init(t, filepath.Join(t.TempDir(), "ops"))
	gittest.Git(t, repo, "checkout", "-q", "-b", "other")
	gittest.Commit(t, repo, "README.md", "other\n", "other change")
	gittest.Git(t, repo, "checkout", "-q", "main")
	gittest.Commit(t, repo, "README.md", "main\n", "main change")

	if _, err := gittest.TryGit(repo, "cherry-pick", "other"); err == nil {
		t.Fatal("ожидался конфликт cherry-pick")
	}
	if op := readStatus(t, repo).Worktree.Operation; op != "cherry-pick" {
		t.Errorf("cherry-pick: %q", op)
	}
	gittest.Git(t, repo, "cherry-pick", "--abort")

	gittest.Git(t, repo, "checkout", "-q", "other")
	if _, err := gittest.TryGit(repo, "rebase", "main"); err == nil {
		t.Fatal("ожидался конфликт rebase")
	}
	st := readStatus(t, repo)
	if st.Worktree.Operation != "rebase" || st.Worktree.Conflicts != 1 {
		t.Errorf("rebase: %+v", st.Worktree)
	}
}

func TestReadStatusUpstream(t *testing.T) { // A08
	ahead := readStatus(t, gittest.Ahead(t)).Branch
	if ahead.Upstream != "origin/main" || intp(ahead.Ahead) != "1" || intp(ahead.Behind) != "0" {
		t.Errorf("ahead: %+v", ahead)
	}

	behindRepo, bare := gittest.Behind(t)
	behind := readStatus(t, behindRepo).Branch
	if intp(behind.Ahead) != "0" || intp(behind.Behind) != "1" {
		t.Errorf("behind: ahead=%s behind=%s", intp(behind.Ahead), intp(behind.Behind))
	}
	// Сверка с ручной командой Git.
	if sb := gittest.Git(t, behindRepo, "status", "-sb"); !strings.Contains(sb, "[behind 1]") {
		t.Errorf("git status -sb: %q", sb)
	}

	// Ветка на сервере удалена, fetch --prune убрал tracking ref: upstream «gone».
	gittest.Git(t, bare, "branch", "-q", "-m", "main", "renamed")
	gittest.Git(t, behindRepo, "fetch", "-q", "--prune", "origin")
	gone := readStatus(t, behindRepo).Branch
	if !gone.UpstreamGone || gone.Ahead != nil || gone.Behind != nil {
		t.Errorf("gone: %+v", gone)
	}
}

func TestLastFetchAt(t *testing.T) {
	repo, _ := gittest.Upstream(t)
	if b := readStatus(t, repo).Branch; b.LastFetchAt != nil {
		t.Fatalf("fetch не выполнялся, а время есть: %v", b.LastFetchAt)
	}
	gittest.Git(t, repo, "fetch", "-q", "origin")
	first := readStatus(t, repo).Branch.LastFetchAt
	if first == nil {
		t.Fatal("после fetch нет времени")
	}
	fetchHead := filepath.Join(repo, ".git", "FETCH_HEAD")
	old := first.Add(-time.Hour)
	if err := os.Chtimes(fetchHead, old, old); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, repo, "fetch", "-q", "origin")
	second := readStatus(t, repo).Branch.LastFetchAt
	if second == nil || !second.After(old) {
		t.Errorf("время fetch не обновилось: %v", second)
	}
}

func TestBranches(t *testing.T) { // A15
	main, wt := gittest.Worktree(t)
	gittest.Git(t, main, "branch", "-q", "free")
	gittest.Git(t, main, "branch", "-q", "--set-upstream-to=main", "free")
	gittest.Commit(t, main, "main.txt", "x\n", "main moves on")

	branches, err := Branches(context.Background(), runner.Exec{}, main)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]model.BranchInfo{}
	for _, b := range branches {
		byName[b.Name] = b
	}
	if len(byName) != 3 {
		t.Fatalf("ветки: %+v", branches)
	}
	if m := byName["main"]; !m.Current || m.WorktreePath != "" || m.CommitAt == nil || m.HeadSHA != gittest.Git(t, main, "rev-parse", "main") {
		t.Errorf("main: %+v", m)
	}
	if f := byName["feature"]; f.Current || canonical(t, f.WorktreePath) != canonical(t, wt) {
		t.Errorf("feature занята другим worktree: %+v", f)
	}
	// Локальный upstream: free отстаёт от main на 1.
	if f := byName["free"]; f.Upstream != "main" || intp(f.Ahead) != "0" || intp(f.Behind) != "1" || f.WorktreePath != "" {
		t.Errorf("free: %+v", f)
	}
	if b := byName["main"]; b.Upstream != "" || b.Ahead != nil {
		t.Errorf("main без upstream: %+v", b)
	}

	// Из дополнительного worktree: текущая — feature, main занята основной копией.
	branches, err = Branches(context.Background(), runner.Exec{}, wt)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range branches {
		if b.Name == "feature" && !b.Current {
			t.Errorf("feature должна быть текущей в worktree")
		}
		if b.Name == "main" && canonical(t, b.WorktreePath) != canonical(t, main) {
			t.Errorf("main занята основной копией: %+v", b)
		}
	}
}

func TestParseTrack(t *testing.T) {
	for in, want := range map[string]string{"": "0/0", "ahead 2": "2/0", "behind 3": "0/3", "ahead 1, behind 4": "1/4"} {
		gone, a, b := parseTrack(in)
		if gone || intp(a)+"/"+intp(b) != want {
			t.Errorf("%q: %v %s/%s", in, gone, intp(a), intp(b))
		}
	}
	if gone, a, b := parseTrack("gone"); !gone || a != nil || b != nil {
		t.Errorf("gone")
	}
}
