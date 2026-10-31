package gitops

import (
	"testing"

	"github.com/forktopus/forktopus/internal/model"
)

func intp(n int) *int { return &n }

// clean — чистая ветка main с upstream, совпадает с ним.
func clean() State {
	return State{
		Branch:   &model.BranchStatus{Name: "main", HeadSHA: "abc", Upstream: "origin/main", Ahead: intp(0), Behind: intp(0)},
		Worktree: &model.WorktreeStatus{},
		Remote:   "origin",
		Remotes:  1,
		Target:   &model.BranchInfo{Name: "feature"},
	}
}

// Матрица из idea 04-GIT-AND-PROVIDERS §3: строка — состояние, столбцы —
// ожидаемый код для fetch / pull / switch ("" — разрешено).
func TestCheckMatrix(t *testing.T) {
	cases := []struct {
		name                 string
		mod                  func(*State)
		fetch, pull, switch_ model.ErrorCode
	}{
		{"чистая, есть upstream", func(*State) {}, "", "", ""},
		{"staged", func(s *State) { s.Worktree.Staged = 1 }, "", model.CodeDirtyWorktree, model.CodeDirtyWorktree},
		{"unstaged", func(s *State) { s.Worktree.Unstaged = 2 }, "", model.CodeDirtyWorktree, model.CodeDirtyWorktree},
		{"untracked", func(s *State) { s.Worktree.Untracked = 1 }, "", model.CodeDirtyWorktree, model.CodeDirtyWorktree},
		{"конфликт", func(s *State) { s.Worktree.Conflicts = 1 }, "", model.CodeGitConflict, model.CodeGitConflict},
		{"merge в процессе", func(s *State) { s.Worktree.Operation = "merge" }, "", model.CodeGitConflict, model.CodeGitConflict},
		{"rebase в процессе", func(s *State) { s.Worktree.Operation = "rebase" }, "", model.CodeGitConflict, model.CodeGitConflict},
		{"cherry-pick в процессе", func(s *State) { s.Worktree.Operation = "cherry-pick" }, "", model.CodeGitConflict, model.CodeGitConflict},
		{"detached, коммит в ветке", func(s *State) {
			s.Branch = &model.BranchStatus{Detached: true, HeadSHA: "abc"}
			s.HeadReachable = true
		}, "", model.CodeNoUpstream, ""},
		{"detached, коммит вне веток", func(s *State) {
			s.Branch = &model.BranchStatus{Detached: true, HeadSHA: "abc"}
		}, "", model.CodeNoUpstream, model.CodeDetachedUnreachable},
		{"нет upstream", func(s *State) {
			s.Branch.Upstream, s.Branch.Ahead, s.Branch.Behind = "", nil, nil
		}, "", model.CodeNoUpstream, ""},
		{"upstream удалён", func(s *State) {
			s.Branch.UpstreamGone, s.Branch.Ahead, s.Branch.Behind = true, nil, nil
		}, "", model.CodeNoUpstream, ""},
		{"нет коммитов", func(s *State) {
			s.Branch = &model.BranchStatus{Name: "main", Unborn: true}
		}, "", model.CodeNoUpstream, ""},
		{"нет remote", func(s *State) {
			s.Remote, s.Remotes = "", 0
			s.Branch.Upstream, s.Branch.Ahead, s.Branch.Behind = "", nil, nil
		}, model.CodeNoRemote, model.CodeNoUpstream, ""},
		{"remote не выбран из нескольких", func(s *State) { s.Remote, s.Remotes = "", 2 }, model.CodeNoRemote, "", ""},
		{"впереди upstream", func(s *State) { s.Branch.Ahead = intp(2) }, "", "", ""},
		{"позади upstream", func(s *State) { s.Branch.Behind = intp(3) }, "", "", ""},
		{"история разошлась", func(s *State) { s.Branch.Ahead, s.Branch.Behind = intp(1), intp(1) }, "", model.CodeDiverged, ""},
		{"ветка занята другим worktree", func(s *State) { s.Target.WorktreePath = "/elsewhere" }, "", "", model.CodeBranchInWorktree},
		{"ветка занята и дерево грязное", func(s *State) {
			s.Target.WorktreePath = "/elsewhere"
			s.Worktree.Unstaged = 1
		}, "", model.CodeDirtyWorktree, model.CodeBranchInWorktree},
		{"целевая ветка — текущая", func(s *State) { s.Target.Current = true }, "", "", model.CodeInvalidInput},
		{"целевой ветки нет", func(s *State) { s.Target = nil }, "", "", model.CodeInvalidInput},
		{"статус не прочитан", func(s *State) { s.Branch = nil }, model.CodeOther, model.CodeOther, model.CodeOther},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for op, want := range map[model.Operation]model.ErrorCode{
				model.OperationFetch:  c.fetch,
				model.OperationPull:   c.pull,
				model.OperationSwitch: c.switch_,
			} {
				st := clean()
				c.mod(&st)
				got := Check(op, st)
				switch {
				case want == "" && got != nil:
					t.Errorf("%s: ожидалось разрешено, получено %v", op, got)
				case want != "" && (got == nil || got.Code != want):
					t.Errorf("%s: ожидалось %s, получено %v", op, want, got)
				case got != nil && got.Message == "":
					t.Errorf("%s: у блокировки нет причины", op)
				}
			}
		})
	}
}

func TestMatches(t *testing.T) {
	b := &model.BranchStatus{Name: "main", HeadSHA: "abc"}
	if !Matches(model.OpExpect{Branch: "main", HeadSHA: "abc"}, b) {
		t.Error("одинаковое состояние не совпало")
	}
	for _, e := range []model.OpExpect{{Branch: "feature", HeadSHA: "abc"}, {Branch: "main", HeadSHA: "def"}, {}} {
		if Matches(e, b) {
			t.Errorf("%+v не должно совпадать с %+v", e, b)
		}
	}
	if Matches(model.OpExpect{}, nil) {
		t.Error("непрочитанный статус не должен совпадать")
	}
	if !Matches(model.OpExpect{HeadSHA: "abc"}, &model.BranchStatus{Detached: true, HeadSHA: "abc"}) {
		t.Error("detached HEAD с тем же коммитом не совпал")
	}
}

func TestUnknownOperation(t *testing.T) {
	if got := Check("reset", clean()); got == nil || got.Code != model.CodeInvalidInput {
		t.Fatalf("ожидался INVALID_INPUT, получено %v", got)
	}
}
