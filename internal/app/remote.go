package app

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/providers"
	"github.com/forktopus/forktopus/internal/remote"
	"github.com/forktopus/forktopus/internal/tools"
)

// EventRemoteStatus — получено серверное состояние ветки репозитория (PR/MR и CI).
const EventRemoteStatus = "remote:status"

func init() {
	application.RegisterEvent[model.RemoteStatus](EventRemoteStatus)
}

// GetRemoteStatus возвращает PR/MR и CI текущей ветки: из кэша, если данные
// свежие (D-046), иначе от CLI провайдера. Итог также приходит событием
// EventRemoteStatus и попадает в список репозиториев.
func (s *AppService) GetRemoteStatus(ctx context.Context, id string) (model.RemoteStatus, error) {
	return s.remoteStatus(ctx, id, false)
}

// RefreshRemote — то же без кэша: ручное обновление серверных данных.
func (s *AppService) RefreshRemote(ctx context.Context, id string) (model.RemoteStatus, error) {
	return s.remoteStatus(ctx, id, true)
}

func (s *AppService) remoteStatus(ctx context.Context, id string, force bool) (st model.RemoteStatus, err error) {
	ctx, done := s.bind(ctx)
	defer done()
	// Паника в разборе ответа CLI превращается в ошибку PR/CI этого
	// репозитория, а не роняет приложение (запрос идёт и в фоновой горутине).
	defer func() {
		if p := recover(); p != nil {
			e := model.Errorf(model.CodeOther, "внутренняя ошибка при запросе к серверу: %v", p)
			slog.Error("паника при запросе PR/CI", "error", e.Message)
			at := time.Now()
			st = model.RemoteStatus{RepoID: id, Branch: st.Branch,
				PR: model.PRSummary{Availability: model.AvailabilityError, Error: e, LastAttemptAt: &at},
				CI: model.CISummary{Availability: model.AvailabilityError, Error: e, LastAttemptAt: &at}}
			emit(EventRemoteStatus, st)
			err = nil
		}
	}()
	repo, err := s.GetRepo(id)
	if err != nil {
		return model.RemoteStatus{}, err
	}
	s.mu.Lock()
	overrides, fetcher := s.cfg.HostProviders, s.remote
	s.mu.Unlock()

	branch := remoteRef(repo)
	st.Branch = branch
	res := fetcher.Status(ctx, remote.Request{Dir: repo.Path, Remote: selectedRemote(repo), Branch: branchOf(repo), SHA: detachedSHA(repo), Overrides: overrides, Force: force})
	st = model.RemoteStatus{RepoID: id, Branch: branch, Provider: res.Provider, PR: res.PR, CI: res.CI}
	if err := ctx.Err(); err != nil {
		return st, model.Errorf(model.CodeOpCancelled, "запрос серверных данных отменён")
	}

	s.mu.Lock()
	i, found := s.findLocked(id)
	if found == nil {
		// Fetcher помнит последний успех только с запуска; после перезапуска
		// он есть лишь в кэше списка (там же ветка, к которой он относится).
		if old := s.snapshot.Repositories[i]; remoteRef(old) == branch {
			if st.PR.LastSuccessAt == nil {
				st.PR.LastSuccessAt = old.PR.LastSuccessAt
			}
			if st.CI.LastSuccessAt == nil {
				st.CI.LastSuccessAt = old.CI.LastSuccessAt
			}
		}
	}
	s.remoteByRepo[id] = st
	if found == nil {
		repos := slices.Clone(s.snapshot.Repositories)
		repos[i] = s.withRemoteLocked(repos[i], nil)
		s.snapshot.Repositories = repos
		s.saveCacheLocked()
	}
	s.mu.Unlock()
	emit(EventRemoteStatus, st)
	return st, nil
}

// refreshRemoteAll запрашивает серверные данные всех репозиториев в фоне;
// ограничения по хостам и кэш — в remote.Fetcher.
func (s *AppService) refreshRemoteAll() {
	s.mu.Lock()
	ids := make([]string, len(s.snapshot.Repositories))
	for i, r := range s.snapshot.Repositories {
		ids[i] = r.ID
	}
	s.mu.Unlock()
	for _, id := range ids {
		go s.remoteStatus(s.life, id, false) //nolint:errcheck // итог приходит событием
	}
}

// withRemoteLocked подставляет в свежепрочитанный репозиторий серверные данные:
// полученные для его текущей ветки, а если их нет — прежние из prev той же
// ветки (например, из кэша прошлого запуска; LastSuccessAt покажет их возраст).
func (s *AppService) withRemoteLocked(repo model.Repository, prev *model.Repository) model.Repository {
	branch := remoteRef(repo)
	if st, ok := s.remoteByRepo[repo.ID]; ok && st.Branch == branch {
		repo.PR, repo.CI = st.PR, st.CI
		// Тип сервера по резолверу точнее, чем по хосту при сканировании
		// (self-hosted узнаётся по логинам CLI, D-042).
		if i := slices.IndexFunc(repo.Remotes, func(r model.RemoteInfo) bool { return r.Name == repo.Remote }); i >= 0 && st.Provider != "" {
			repo.Remotes = slices.Clone(repo.Remotes)
			repo.Remotes[i].Provider = st.Provider
		}
		return repo
	}
	// Пришедшие вместе с repo PR/CI (scan.Refresh копирует запись целиком)
	// могли относиться к другой ветке — доверяем только prev той же ветки.
	repo.PR, repo.CI = model.PRSummary{}, model.CISummary{}
	if prev != nil && remoteRef(*prev) == branch {
		repo.PR, repo.CI = prev.PR, prev.CI
	}
	return repo
}

// needsRemote — для текущей ветки ещё нет серверных данных.
func (s *AppService) needsRemoteLocked(repo model.Repository) bool {
	st, ok := s.remoteByRepo[repo.ID]
	return !ok || st.Branch != remoteRef(repo)
}

// remoteRef — к чему относятся серверные данные: ветка или `@SHA` при
// detached HEAD (D-059); пусто — статус не прочитан.
func remoteRef(r model.Repository) string {
	if sha := detachedSHA(r); sha != "" {
		return "@" + sha
	}
	return branchOf(r)
}

// detachedSHA — текущий коммит при detached HEAD, иначе пусто.
func detachedSHA(r model.Repository) string {
	if r.Branch == nil || !r.Branch.Detached {
		return ""
	}
	return r.Branch.HeadSHA
}

// branchOf — текущая ветка; пусто — detached HEAD или статус не прочитан.
func branchOf(r model.Repository) string {
	if r.Branch == nil || r.Branch.Detached {
		return ""
	}
	return r.Branch.Name
}

func selectedRemote(r model.Repository) *model.RemoteInfo {
	i := slices.IndexFunc(r.Remotes, func(rm model.RemoteInfo) bool { return rm.Name == r.Remote })
	if i < 0 {
		return nil
	}
	rm := r.Remotes[i]
	return &rm
}

// cliOf — CLI провайдера для диагностики.
var cliOf = map[model.Provider]string{
	model.ProviderGitHub: "gh",
	model.ProviderGitLab: "glab",
	model.ProviderGitea:  "tea",
}

// CheckTools проверяет git и CLI провайдеров: для серверов, к которым относятся
// репозитории, — версию и авторизацию (по одной проверке на CLI, сервер и
// логин), для остальных CLI — только наличие и версию.
func (s *AppService) CheckTools(ctx context.Context) []model.ToolDiagnostic {
	ctx, done := s.bind(ctx)
	defer done()
	s.mu.Lock()
	repos := slices.Clone(s.snapshot.Repositories)
	overrides, fetcher := s.cfg.HostProviders, s.remote
	s.mu.Unlock()

	type check struct {
		dir string
		t   providers.Target
	}
	var checks []check
	seen := map[string]bool{}
	for _, r := range repos {
		rm := selectedRemote(r)
		if rm == nil || rm.Host == "" {
			continue
		}
		t, e := fetcher.Resolve(ctx, *rm, overrides, false)
		if e != nil {
			continue
		}
		key := string(t.Provider) + "|" + t.Host + "|" + t.Login
		if !seen[key] {
			seen[key] = true
			checks = append(checks, check{dir: r.Path, t: t})
		}
	}

	diags := make([]model.ToolDiagnostic, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					e := model.Errorf(model.CodeOther, "внутренняя ошибка проверки: %v", p)
					slog.Error("паника при проверке CLI", "error", e.Message)
					diags[i] = model.ToolDiagnostic{Name: cliOf[c.t.Provider], Host: c.t.Host, Availability: model.AvailabilityError, Error: e}
				}
			}()
			diags[i] = fetcher.Diagnose(ctx, c.dir, c.t)
		}()
	}
	wg.Wait()

	for _, p := range []model.Provider{model.ProviderGitHub, model.ProviderGitLab, model.ProviderGitea} {
		if !slices.ContainsFunc(checks, func(c check) bool { return c.t.Provider == p }) {
			diags = append(diags, tools.CheckCLI(ctx, s.runner, cliOf[p]))
		}
	}
	slices.SortStableFunc(diags, func(a, b model.ToolDiagnostic) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(a.Host, b.Host)
	})
	out := append([]model.ToolDiagnostic{tools.CheckGit(ctx, s.runner)}, diags...)
	s.mu.Lock()
	s.lastTools, s.toolsAt = slices.Clone(out), time.Now()
	s.mu.Unlock()
	return out
}
