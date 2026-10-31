package gitops

import "github.com/forktopus/forktopus/internal/model"

// State — свежее состояние репозитория, по которому проверяются предусловия.
type State struct {
	Branch   *model.BranchStatus
	Worktree *model.WorktreeStatus
	Remote   string // выбранный remote
	Remotes  int    // сколько remote всего
	// Target — ветка, на которую переключаемся (только switch); nil — такой локальной ветки нет.
	Target *model.BranchInfo
	// HeadReachable — коммит detached HEAD входит в какую-либо ветку (только switch, D-034).
	HeadReachable bool
}

// Matches — состояние совпадает с тем, что видел пользователь (D-032).
func Matches(e model.OpExpect, b *model.BranchStatus) bool {
	return b != nil && e.Branch == b.Name && e.HeadSHA == b.HeadSHA
}

// Check проверяет операцию по безопасной матрице (idea 04-GIT-AND-PROVIDERS §3).
// nil — можно выполнять; иначе код и причина блокировки. Git не вызывается.
func Check(op model.Operation, st State) *model.Error {
	if st.Branch == nil || st.Worktree == nil {
		return model.Errorf(model.CodeOther, "статус репозитория не прочитан")
	}
	b, w := st.Branch, st.Worktree
	busy := w.Conflicts > 0 || w.Operation != ""
	dirty := w.Staged+w.Unstaged+w.Untracked > 0

	switch op {
	case model.OperationFetch:
		// Fetch не меняет рабочее дерево: разрешён при изменениях, конфликте и detached HEAD.
		switch {
		case st.Remotes == 0:
			return model.Errorf(model.CodeNoRemote, "нечего fetch: у репозитория нет remote")
		case st.Remote == "":
			return model.Errorf(model.CodeNoRemote, "remote не выбран: выберите remote репозитория")
		}
		return nil

	case model.OperationPull:
		switch {
		case busy:
			return conflictError(w)
		case dirty:
			return model.Errorf(model.CodeDirtyWorktree, "есть локальные изменения: закоммитьте или отложите их в терминале")
		case b.Detached:
			return model.Errorf(model.CodeNoUpstream, "detached HEAD: pull выполняется только на ветке")
		case b.Unborn:
			return model.Errorf(model.CodeNoUpstream, "в ветке ещё нет коммитов")
		case b.Upstream == "":
			return model.Errorf(model.CodeNoUpstream, "у ветки %s нет upstream: настройте его в терминале (git branch --set-upstream-to)", b.Name)
		case b.UpstreamGone:
			return model.Errorf(model.CodeNoUpstream, "upstream %s удалён на сервере", b.Upstream)
		case b.Ahead != nil && b.Behind != nil && *b.Ahead > 0 && *b.Behind > 0:
			return model.Errorf(model.CodeDiverged, "история разошлась с %s: fast-forward невозможен, объедините ветки в терминале", b.Upstream)
		}
		return nil

	case model.OperationSwitch:
		t := st.Target
		switch {
		case t == nil:
			return model.Errorf(model.CodeInvalidInput, "такой локальной ветки нет")
		case t.Current:
			return model.Errorf(model.CodeInvalidInput, "ветка %s уже выбрана", t.Name)
		case t.WorktreePath != "":
			return model.Errorf(model.CodeBranchInWorktree, "ветка %s открыта в другой рабочей копии: %s", t.Name, t.WorktreePath)
		case busy:
			return conflictError(w)
		case dirty:
			return model.Errorf(model.CodeDirtyWorktree, "есть локальные изменения: закоммитьте или отложите их в терминале")
		case b.Detached && !st.HeadReachable:
			return model.Errorf(model.CodeDetachedUnreachable, "коммит %s не входит ни в одну ветку и потеряется: создайте для него ветку в терминале", short(b.HeadSHA))
		}
		return nil
	}
	return model.Errorf(model.CodeInvalidInput, "неизвестная операция %q", op)
}

func conflictError(w *model.WorktreeStatus) *model.Error {
	if w.Operation != "" {
		return model.Errorf(model.CodeGitConflict, "не завершена операция Git: %s", w.Operation)
	}
	return model.Errorf(model.CodeGitConflict, "есть неразрешённые конфликты")
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
