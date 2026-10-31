package scan

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/forktopus/forktopus/internal/gitlocal"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// Concurrency — сколько репозиториев читается одновременно (rev-parse,
// remote -v, status). По замерам 11.2 (TestLoad) время скана падает почти
// линейно до 8 и слабо — дальше; 8 — верхняя граница рекомендации idea (4–8),
// чтобы не забивать диск на больших рабочих копиях. Переменная — только чтобы
// нагрузочный тест сравнивал уровни.
var Concurrency = 8

// Options — параметры сканирования.
type Options struct {
	// SkipDirs — имена каталогов, в которые не заходить; пусто — DefaultSkipDirs.
	SkipDirs []string
	// Remotes — выбранный пользователем remote: ID репозитория → имя remote.
	Remotes map[string]string
	// HostProviders — тип провайдера для self-hosted хостов.
	HostProviders map[string]model.Provider
	// OnRepository вызывается для каждого нового репозитория по мере
	// нахождения (из одной горутины). ParentID и Nesting в нём ещё не заполнены:
	// окончательный список — в результате Run.
	OnRepository func(model.Repository)
	// Lock не даёт читать статус репозитория посреди изменяющей операции
	// (fetch/pull/switch); nil — без блокировки. Refresh её не берёт: блокировку
	// держит вызывающий.
	Lock func(ctx context.Context, id string) (unlock func(), err error)
}

type candidate struct {
	rootID string
	dir    string
}

type found struct {
	repo         model.Repository
	superproject string
	skip         bool // bare-репозиторий: пропускается молча
}

// Run сканирует корни. Ошибки отдельных каталогов и репозиториев не
// прерывают обход: каталоги попадают в Issues, репозитории — с полем Error.
// При отмене ctx возвращается найденное к этому моменту и ошибка OP_CANCELLED.
func Run(ctx context.Context, r runner.Runner, roots []model.Root, opts Options) (model.ScanResult, error) {
	skipNames := opts.SkipDirs
	if len(skipNames) == 0 {
		skipNames = DefaultSkipDirs
	}
	skip := map[string]bool{}
	for _, name := range skipNames {
		skip[name] = true
	}

	candidates := make(chan candidate)
	results := make(chan found)
	var issues []model.ScanIssue
	var walkErr error

	go func() {
		defer close(candidates)
		for _, root := range roots {
			err := walk(ctx, root.Path, skip,
				func(dir string) {
					select {
					case candidates <- candidate{root.ID, dir}:
					case <-ctx.Done():
					}
				},
				func(path string, err error) {
					issues = append(issues, model.ScanIssue{RootID: root.ID, Path: path, Error: model.Errorf(model.CodeOther, "%v", err)})
				})
			if err != nil {
				walkErr = err
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for range Concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range candidates {
				if ctx.Err() != nil {
					continue
				}
				results <- inspect(ctx, r, c, opts)
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	rootPath := map[string]string{}
	for _, root := range roots {
		rootPath[root.ID] = root.Path
	}
	byID := map[string]*found{}
	var order []string
	for f := range results {
		if f.skip {
			continue
		}
		if prev, ok := byID[f.repo.ID]; ok {
			// Один репозиторий через два корня — одна запись; корень — самый конкретный.
			if len(rootPath[f.repo.RootID]) > len(rootPath[prev.repo.RootID]) {
				prev.repo.RootID = f.repo.RootID
			}
			continue
		}
		byID[f.repo.ID] = &f
		order = append(order, f.repo.ID)
		if opts.OnRepository != nil {
			opts.OnRepository(f.repo)
		}
	}
	// issues и walkErr пишет горутина обхода; results закрыт после закрытия
	// candidates, значит она уже завершилась.

	repos := make([]model.Repository, 0, len(order))
	for _, id := range order {
		repos = append(repos, byID[id].repo)
	}
	linkParents(repos, byID)
	slices.SortFunc(repos, func(a, b model.Repository) int { return strings.Compare(a.Path, b.Path) })

	result := model.ScanResult{Repositories: repos, Issues: issues, ScannedAt: time.Now()}
	if result.Issues == nil {
		result.Issues = []model.ScanIssue{}
	}
	if walkErr != nil || ctx.Err() != nil {
		return result, model.Errorf(model.CodeOpCancelled, "сканирование прервано")
	}
	return result, nil
}

func inspect(ctx context.Context, r runner.Runner, c candidate, opts Options) found {
	repo := model.Repository{RootID: c.rootID, Path: c.dir, Remotes: []model.RemoteInfo{}, UpdatedAt: time.Now()}
	info, err := gitlocal.Inspect(ctx, r, c.dir)
	if model.CodeOf(err) == model.CodeUnsupported {
		return found{skip: true}
	}
	if err != nil {
		repo.ID = model.PathID(c.dir)
		repo.Error = model.AsError(err)
		return found{repo: repo}
	}
	repo.Path = info.TopLevel
	if p, err := filepath.EvalSymlinks(info.TopLevel); err == nil {
		repo.Path = p
	}
	repo.ID = model.PathID(repo.Path)
	repo.GitDir = info.GitDir
	if opts.Lock != nil {
		unlock, err := opts.Lock(ctx, repo.ID)
		if err != nil {
			repo.Error = model.AsError(err)
			return found{repo: repo, superproject: info.Superproject}
		}
		defer unlock()
	}
	readLocal(ctx, r, &repo, opts)
	return found{repo: repo, superproject: info.Superproject}
}

// Refresh перечитывает remote и локальный статус уже найденного репозитория
// (после fetch/pull/switch или по запросу пользователя). Расположение и связи
// с родителем сохраняются; сбой чтения — в Error этого репозитория.
func Refresh(ctx context.Context, r runner.Runner, repo model.Repository, opts Options) model.Repository {
	repo.Error = nil
	repo.Remotes = []model.RemoteInfo{}
	repo.Remote = ""
	repo.Branch, repo.Worktree = nil, nil
	readLocal(ctx, r, &repo, opts)
	return repo
}

// readLocal заполняет remote и локальный статус. Ошибка одного шага не
// отменяет другой: без remote статус всё равно полезен, и наоборот.
func readLocal(ctx context.Context, r runner.Runner, repo *model.Repository, opts Options) {
	remotes, err := gitlocal.Remotes(ctx, r, repo.Path, opts.HostProviders)
	if err != nil {
		repo.Error = model.AsError(err)
	} else {
		repo.Remotes = remotes
		repo.Remote = gitlocal.SelectRemote(remotes, opts.Remotes[repo.ID])
	}
	st, err := gitlocal.ReadStatus(ctx, r, repo.Path)
	if err != nil {
		if repo.Error == nil {
			repo.Error = model.AsError(err)
		}
	} else {
		repo.Branch, repo.Worktree = &st.Branch, &st.Worktree
	}
	repo.UpdatedAt = time.Now()
}

// linkParents заполняет ParentID ближайшим найденным репозиторием, внутри
// рабочей копии которого лежит репозиторий, и Nesting (D-025).
func linkParents(repos []model.Repository, byID map[string]*found) {
	idByPath := map[string]string{}
	for _, r := range repos {
		idByPath[r.Path] = r.ID
	}
	for i := range repos {
		r := &repos[i]
		for dir := filepath.Dir(r.Path); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			if id, ok := idByPath[dir]; ok {
				r.ParentID = id
				break
			}
		}
		switch {
		case byID[r.ID].superproject != "":
			r.Nesting = model.NestingSubmodule
			if id, ok := idByPath[byID[r.ID].superproject]; ok {
				r.ParentID = id
			}
		case r.ParentID != "":
			r.Nesting = model.NestingNested
		}
	}
}
