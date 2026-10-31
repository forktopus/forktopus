package app

import (
	"context"
	"slices"
	"time"

	"github.com/forktopus/forktopus/internal/gitlocal"
	"github.com/forktopus/forktopus/internal/gitops"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/scan"
)

// opLogSize — сколько последних операций хранится в журнале.
const opLogSize = 100

// FetchRepo выполняет git fetch выбранного remote. expect — ветка и HEAD,
// которые видел пользователь (D-032).
func (s *AppService) FetchRepo(ctx context.Context, id string, expect model.OpExpect) (model.OpResult, error) {
	return s.runOp(ctx, id, model.OperationFetch, "", expect)
}

// FastForwardPull подтягивает upstream текущей ветки только fast-forward.
func (s *AppService) FastForwardPull(ctx context.Context, id string, expect model.OpExpect) (model.OpResult, error) {
	return s.runOp(ctx, id, model.OperationPull, "", expect)
}

// SwitchBranch переключает рабочую копию на существующую локальную ветку.
// Ветка ищется среди прочитанных с диска, а не берётся из строки UI как есть.
func (s *AppService) SwitchBranch(ctx context.Context, id, branch string, expect model.OpExpect) (model.OpResult, error) {
	return s.runOp(ctx, id, model.OperationSwitch, branch, expect)
}

// ListOperations возвращает журнал последних операций, новые — первыми.
func (s *AppService) ListOperations() []model.OpRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := slices.Clone(s.opLog)
	slices.Reverse(out)
	return out
}

// runOp — общий порядок безопасной операции: блокировка репозитория, свежее
// чтение статуса, сверка с тем, что видел пользователь, предусловия по
// матрице, Git, повторное чтение статуса. Ошибка возвращается только для
// неизвестного репозитория; блокировки и сбои Git — в OpResult.Error вместе
// со свежим статусом.
func (s *AppService) runOp(ctx context.Context, id string, op model.Operation, target string, expect model.OpExpect) (model.OpResult, error) {
	ctx, done := s.bind(ctx)
	defer done()
	start := time.Now()
	repo, err := s.GetRepo(id)
	if err != nil {
		return model.OpResult{}, err
	}
	s.mu.Lock()
	opts := s.scanOptionsLocked()
	s.mu.Unlock()

	rec := model.OpRecord{RepoID: id, Op: op, Target: target, At: start}
	finish := func(repo model.Repository, err error) (model.OpResult, error) {
		rec.DurationMs = time.Since(start).Milliseconds()
		rec.Error = model.AsError(err)
		s.mu.Lock()
		s.opLog = append(s.opLog, rec)
		if len(s.opLog) > opLogSize {
			s.opLog = slices.Clone(s.opLog[len(s.opLog)-opLogSize:])
		}
		s.mu.Unlock()
		return model.OpResult{OpRecord: rec, Repo: repo}, nil
	}

	unlock, err := s.locks.TryLock(id)
	if err != nil {
		return finish(repo, err)
	}
	defer unlock()

	fresh := s.storeRepo(scan.Refresh(ctx, s.runner, repo, opts))
	if fresh.Branch == nil || fresh.Worktree == nil {
		if fresh.Error != nil {
			return finish(fresh, fresh.Error)
		}
		return finish(fresh, model.Errorf(model.CodeOther, "не удалось прочитать статус репозитория"))
	}
	if !gitops.Matches(expect, fresh.Branch) {
		return finish(fresh, model.Errorf(model.CodeStateChanged, "состояние репозитория изменилось, пока вы его смотрели: проверьте его и повторите"))
	}

	st := gitops.State{Branch: fresh.Branch, Worktree: fresh.Worktree, Remote: fresh.Remote, Remotes: len(fresh.Remotes)}
	if op == model.OperationFetch {
		rec.Target = fresh.Remote
	}
	if op == model.OperationSwitch {
		branches, err := gitlocal.Branches(ctx, s.runner, fresh.Path)
		if err != nil {
			return finish(fresh, err)
		}
		if i := slices.IndexFunc(branches, func(b model.BranchInfo) bool { return b.Name == target }); i >= 0 {
			st.Target = &branches[i]
		}
		if fresh.Branch.Detached {
			if st.HeadReachable, err = gitops.HeadReachable(ctx, s.runner, fresh.Path); err != nil {
				return finish(fresh, err)
			}
		}
	}
	if blocked := gitops.Check(op, st); blocked != nil {
		return finish(fresh, blocked)
	}

	switch op {
	case model.OperationFetch:
		err = gitops.Fetch(ctx, s.runner, fresh.Path, fresh.Remote)
	case model.OperationPull:
		err = gitops.Pull(ctx, s.runner, fresh.Path)
	case model.OperationSwitch:
		err = gitops.Switch(ctx, s.runner, fresh.Path, target)
	}
	// Статус перечитывается и после сбоя или отмены: Git мог успеть что-то изменить.
	after := s.storeRepo(scan.Refresh(context.WithoutCancel(ctx), s.runner, fresh, opts))
	return finish(after, err)
}

// storeRepo заменяет запись репозитория в списке и кэше, если она ещё там
// (пока шла операция, корень могли убрать или пересканировать). Серверные
// данные текущей ветки сохраняются; для новой ветки запрашиваются в фоне.
func (s *AppService) storeRepo(repo model.Repository) model.Repository {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i, err := s.findLocked(repo.ID); err == nil {
		repo = s.withRemoteLocked(repo, s.prevLocked(repo.ID))
		repos := slices.Clone(s.snapshot.Repositories)
		repos[i] = repo
		s.snapshot.Repositories = repos
		s.saveCacheLocked()
		// Ветка сменилась (switch, переключение в терминале) — нужны PR/CI новой.
		if s.needsRemoteLocked(repo) {
			go s.remoteStatus(s.life, repo.ID, false) //nolint:errcheck // итог приходит событием
		}
	}
	return repo
}
