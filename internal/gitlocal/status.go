package gitlocal

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// Status — локальное состояние рабочей копии.
type Status struct {
	Branch   model.BranchStatus
	Worktree model.WorktreeStatus
}

// ReadStatus читает текущую ветку, upstream, ahead/behind, изменения и
// незавершённую операцию. Репозиторий не меняется, в сеть не ходит.
func ReadStatus(ctx context.Context, r runner.Runner, dir string) (Status, error) {
	// porcelain v2 с --branch отдаёт ветку, upstream и ahead/behind вместе с
	// изменениями — один процесс на репозиторий вместо четырёх (D-030).
	out, err := git(ctx, r, dir, "status", "--porcelain=v2", "--branch", "-z", "--untracked-files=all")
	if err != nil {
		return Status{}, err
	}
	st := parseStatus([]byte(out))
	paths, err := gitPaths(ctx, r, dir, opMarkers...)
	if err != nil {
		return Status{}, err
	}
	st.Worktree.Operation = detectOperation(paths)
	if fi, err := os.Stat(paths["FETCH_HEAD"]); err == nil {
		t := fi.ModTime()
		st.Branch.LastFetchAt = &t
	}
	return st, nil
}

// parseStatus разбирает `git status --porcelain=v2 --branch -z`.
// Записи разделены NUL; у переименования (тип 2) исходный путь — отдельной записью.
func parseStatus(out []byte) Status {
	var st Status
	upstreamSet, abSet := false, false
	fields := bytes.Split(out, []byte{0})
	for i := 0; i < len(fields); i++ {
		line := string(fields[i])
		if line == "" {
			continue
		}
		switch line[0] {
		case '#':
			key, value, _ := strings.Cut(strings.TrimPrefix(line, "# "), " ")
			switch key {
			case "branch.oid":
				if value == "(initial)" {
					st.Branch.Unborn = true
				} else {
					st.Branch.HeadSHA = value
				}
			case "branch.head":
				if value == "(detached)" {
					st.Branch.Detached = true
				} else {
					st.Branch.Name = value
				}
			case "branch.upstream":
				st.Branch.Upstream = value
				upstreamSet = true
			case "branch.ab":
				// «+A -B»
				var ahead, behind int
				for _, part := range strings.Fields(value) {
					n, _ := strconv.Atoi(part[1:])
					if part[0] == '+' {
						ahead = n
					} else {
						behind = n
					}
				}
				st.Branch.Ahead, st.Branch.Behind = &ahead, &behind
				abSet = true
			}
		case '1', '2':
			// «1 XY sub mH mI mW hH hI path», у «2» перед путём ещё «Xscore»;
			// X — индекс (staged), Y — рабочее дерево (unstaged); «.» — без изменений.
			n := 9
			if line[0] == '2' {
				n = 10
			}
			parts := strings.SplitN(line, " ", n)
			if len(parts) < n || len(parts[1]) != 2 {
				continue
			}
			xy := parts[1]
			if xy[0] != '.' {
				st.Worktree.Staged++
			}
			if xy[1] != '.' {
				st.Worktree.Unstaged++
			}
			f := model.ChangedFile{Path: parts[n-1], Code: strings.ReplaceAll(xy, ".", " ")}
			if line[0] == '2' && i+1 < len(fields) {
				i++ // исходный путь переименования/копирования — следующая запись
				f.OrigPath = string(fields[i])
			}
			addFile(&st.Worktree, f)
		case 'u':
			// «u XY sub m1 m2 m3 mW h1 h2 h3 path»
			st.Worktree.Conflicts++
			if parts := strings.SplitN(line, " ", 11); len(parts) == 11 {
				addFile(&st.Worktree, model.ChangedFile{Path: parts[10], Code: parts[1]})
			}
		case '?':
			st.Worktree.Untracked++
			addFile(&st.Worktree, model.ChangedFile{Path: strings.TrimPrefix(line, "? "), Code: "??"})
		}
	}
	// upstream есть, а ahead/behind нет — remote-tracking ветка пропала.
	st.Branch.UpstreamGone = upstreamSet && !abSet
	return st
}

// addFile добавляет файл в список, пока не набралось MaxChangedFiles.
func addFile(w *model.WorktreeStatus, f model.ChangedFile) {
	if len(w.Files) < model.MaxChangedFiles {
		w.Files = append(w.Files, f)
	} else {
		w.MoreFiles++
	}
}

// opMarkers — файлы и каталоги в git dir, по которым видна незавершённая операция.
var opMarkers = []string{"rebase-merge", "rebase-apply", "rebase-apply/applying", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "FETCH_HEAD"}

// gitPaths возвращает абсолютные пути файлов в git dir. `--git-path` учитывает,
// что у worktree и сабмодуля git dir лежит не в `.git` рабочей копии.
func gitPaths(ctx context.Context, r runner.Runner, dir string, names ...string) (map[string]string, error) {
	args := []string{"rev-parse"}
	for _, n := range names {
		args = append(args, "--git-path", n)
	}
	out, err := git(ctx, r, dir, args...)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != len(names) {
		return nil, model.Errorf(model.CodeOther, "неожиданный вывод git rev-parse --git-path в %s", dir)
	}
	paths := make(map[string]string, len(names))
	for i, n := range names {
		p := lines[i]
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		paths[n] = p
	}
	return paths, nil
}

func detectOperation(paths map[string]string) string {
	exists := func(name string) bool {
		_, err := os.Stat(paths[name])
		return err == nil
	}
	switch {
	case exists("rebase-merge"):
		return "rebase"
	case exists("rebase-apply/applying"):
		return "am"
	case exists("rebase-apply"):
		return "rebase"
	case exists("MERGE_HEAD"):
		return "merge"
	case exists("CHERRY_PICK_HEAD"):
		return "cherry-pick"
	case exists("REVERT_HEAD"):
		return "revert"
	}
	return ""
}

// branchFormat — поля for-each-ref через NUL. %(worktreepath) — путь рабочей
// копии, где ветка checked out (Git 2.23+); заменяет разбор `git worktree list`.
const branchFormat = "%(HEAD)%00%(refname)%00%(objectname)%00%(committerdate:iso-strict)%00%(upstream:short)%00%(upstream:track,nobracket)%00%(worktreepath)"

// Branches возвращает локальные ветки с upstream и ahead/behind по локальным
// remote-tracking refs.
func Branches(ctx context.Context, r runner.Runner, dir string) ([]model.BranchInfo, error) {
	out, err := git(ctx, r, dir, "for-each-ref", "--format="+branchFormat, "refs/heads/")
	if err != nil {
		return nil, err
	}
	return parseBranches(out), nil
}

func parseBranches(out string) []model.BranchInfo {
	branches := []model.BranchInfo{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(line, "\x00")
		if len(f) != 7 {
			continue
		}
		b := model.BranchInfo{
			Name:     strings.TrimPrefix(f[1], "refs/heads/"),
			Current:  f[0] == "*",
			HeadSHA:  f[2],
			Upstream: f[4],
		}
		if t, err := time.Parse(time.RFC3339, f[3]); err == nil {
			b.CommitAt = &t
		}
		if b.Upstream != "" {
			b.UpstreamGone, b.Ahead, b.Behind = parseTrack(f[5])
		}
		if !b.Current {
			b.WorktreePath = f[6]
		}
		branches = append(branches, b)
	}
	return branches
}

// parseTrack разбирает %(upstream:track,nobracket): «», «ahead N», «behind N»,
// «ahead N, behind M» или «gone».
func parseTrack(s string) (gone bool, ahead, behind *int) {
	if s == "gone" {
		return true, nil, nil
	}
	a, b := 0, 0
	for _, part := range strings.Split(s, ", ") {
		kind, num, ok := strings.Cut(part, " ")
		if !ok {
			continue
		}
		n, _ := strconv.Atoi(num)
		switch kind {
		case "ahead":
			a = n
		case "behind":
			b = n
		}
	}
	return false, &a, &b
}
