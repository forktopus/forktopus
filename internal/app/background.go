package app

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/forktopus/forktopus/internal/gitops"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
	"github.com/forktopus/forktopus/internal/scan"
)

// Фоновая проверка (D-063): раз в интервал из настроек перечитываются
// локальный статус всех репозиториев (перед ним — fetch, если включён) и
// PR/CI через кэш. Работает и при скрытом окне: опрос в Go, а не в UI.
const (
	// EventRepository — свежий статус репозитория, прочитанный в фоне; UI
	// заменяет им запись в списке.
	EventRepository = "repo:updated"
	// EventChecked — фоновая проверка закончена; данные — её время.
	EventChecked = "background:checked"
)

func init() {
	application.RegisterEvent[model.Repository](EventRepository)
	application.RegisterEvent[time.Time](EventChecked)
}

// backgroundWorkers — сколько репозиториев проверяется одновременно.
const backgroundWorkers = 4

// ServiceStartup запускает фоновую проверку; вызывается Wails при старте.
func (s *AppService) ServiceStartup(context.Context, application.ServiceOptions) error {
	go s.backgroundLoop()
	return nil
}

// CheckNow — внеочередная фоновая проверка (меню трея). Если проверка уже
// идёт, следующая начнётся сразу после неё.
func (s *AppService) CheckNow() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// rearmBackground — интервал изменился: отсчёт начинается заново.
func (s *AppService) rearmBackground() {
	select {
	case s.rearm <- struct{}{}:
	default:
	}
}

func (s *AppService) backgroundLoop() {
	for {
		s.mu.Lock()
		every := s.cfg.UI.BackgroundInterval()
		s.mu.Unlock()
		timer := time.NewTimer(every)
		select {
		case <-s.life.Done():
			timer.Stop()
			return
		case <-s.rearm:
			timer.Stop()
			continue
		case <-s.wake:
			timer.Stop()
		case <-timer.C:
		}
		s.backgroundPass(s.life)
	}
}

// backgroundPass проверяет все репозитории. Пока идёт сканирование, проход
// пропускается: сканирование само прочитает статус и запросит PR/CI.
func (s *AppService) backgroundPass(ctx context.Context) {
	if !s.scanMu.TryLock() {
		return
	}
	s.scanMu.Unlock()

	s.mu.Lock()
	repos := slices.Clone(s.snapshot.Repositories)
	opts, fetch, prune := s.scanOptionsLocked(), s.cfg.UI.AutoFetch, s.cfg.UI.FetchPrune
	s.mu.Unlock()

	jobs := make(chan model.Repository)
	var wg sync.WaitGroup
	for range min(backgroundWorkers, len(repos)) {
		wg.Go(func() {
			for repo := range jobs {
				s.backgroundRepo(ctx, repo, opts, fetch, prune)
			}
		})
	}
	for _, repo := range repos {
		if ctx.Err() != nil {
			break
		}
		jobs <- repo
	}
	close(jobs)
	wg.Wait()
	if ctx.Err() != nil {
		return
	}

	at := time.Now()
	s.mu.Lock()
	if s.snapshot.ScannedAt != nil {
		s.snapshot.ScannedAt = &at
		s.snapshot.Stale = false
		s.saveCacheLocked()
	}
	s.mu.Unlock()
	emit(EventChecked, at)
	s.refreshRemoteAll()
}

// backgroundRepo перечитывает один репозиторий. Над ним идёт операция —
// пропускаем: её итог и так обновит статус.
func (s *AppService) backgroundRepo(ctx context.Context, repo model.Repository, opts scan.Options, fetch, prune bool) {
	unlock, err := s.locks.TryLock(repo.ID)
	if err != nil {
		return
	}
	defer unlock()
	if fetch && repo.Remote != "" {
		if err := gitops.Fetch(ctx, batchRunner{s.runner}, repo.Path, repo.Remote, prune); err != nil && ctx.Err() == nil {
			slog.Warn("фоновый fetch не удался", "repo", repo.Path, "error", model.AsError(err).Message)
		}
	}
	fresh := scan.Refresh(ctx, s.runner, repo, opts)
	if ctx.Err() != nil {
		return
	}
	emit(EventRepository, s.storeRepo(fresh))
}

// batchRunner — команды фоновой проверки не спрашивают пароль SSH-ключа или
// учётные данные окном (терминала у них и так нет): без агента fetch просто
// завершится ошибкой, а не будет всплывать окно каждые несколько минут.
type batchRunner struct{ runner.Runner }

func (b batchRunner) Run(ctx context.Context, c runner.Command) (runner.Result, error) {
	c.Env = append(slices.Clone(c.Env), "SSH_ASKPASS_REQUIRE=never", "GCM_INTERACTIVE=never")
	return b.Runner.Run(ctx, c)
}
