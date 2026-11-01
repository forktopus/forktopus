// Package remote получает серверное состояние ветки (PR/MR и CI) через
// адаптеры providers и бережёт серверы и время пользователя (D-046): кэш
// с TTL, не больше PerHost запросов на хост, склейка одинаковых запросов,
// backoff при RATE_LIMIT, а сетевая ошибка хоста на время TTL относится ко
// всем его репозиториям — без сети не ждать таймаута на каждом.
package remote

import (
	"context"
	"sync"
	"time"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/providers"
	"github.com/forktopus/forktopus/internal/runner"
)

// Options — настройки кэша и лимитов; нули заменяются значениями по умолчанию.
type Options struct {
	TTL       time.Duration // срок жизни PR/CI и сетевой ошибки хоста
	LoginsTTL time.Duration // как часто перечитывать логины CLI
	PerHost   int           // параллельных запросов к одному хосту
}

const (
	defaultTTL       = 2 * time.Minute
	defaultLoginsTTL = 10 * time.Minute
	defaultPerHost   = 2
	// loginsMinAge — принудительное обновление не перечитывает логины чаще.
	loginsMinAge = 30 * time.Second
	minBackoff   = time.Minute
	maxBackoff   = 16 * time.Minute
)

// Request — запрос состояния ветки.
type Request struct {
	Dir       string            // рабочая копия: tea работает только внутри неё
	Remote    *model.RemoteInfo // nil — у репозитория нет выбранного remote
	Branch    string            // пусто — detached HEAD
	SHA       string            // текущий коммит при detached HEAD: PR/CI ищутся по нему
	Overrides map[string]model.Provider
	// Force — не брать из кэша и не ждать сетевую ошибку хоста (ручное обновление).
	Force bool
}

// Result — состояние ветки и тип сервера по резолверу.
type Result struct {
	Provider model.Provider
	PR       model.PRSummary
	CI       model.CISummary
}

// Fetcher — точка входа: кэш и лимиты поверх адаптеров.
type Fetcher struct {
	adapters map[model.Provider]providers.RemoteProvider
	discover func(context.Context) []providers.Login
	opts     Options
	now      func() time.Time

	loginsMu sync.Mutex // одно чтение логинов за раз
	mu       sync.Mutex
	logins   []providers.Login
	loginsAt time.Time
	entries  map[string]entry
	hosts    map[string]*host
	inflight map[string]*call
}

type entry struct {
	res Result
	at  time.Time
}

// host — ограничения по одному серверу.
type host struct {
	sem        chan struct{}
	down       *model.Error // сетевая ошибка, действует до downUntil
	downUntil  time.Time
	limitUntil time.Time // RATE_LIMIT: запросы не отправляются до этого времени
	backoff    time.Duration
}

type call struct {
	done chan struct{}
	res  Result
}

// New создаёт Fetcher с адаптерами gh, glab и tea.
func New(r runner.Runner, opts Options) *Fetcher {
	return newFetcher(map[model.Provider]providers.RemoteProvider{
		model.ProviderGitHub: providers.GitHub{Runner: r},
		model.ProviderGitLab: providers.GitLab{Runner: r},
		model.ProviderGitea:  providers.Gitea{Runner: r},
	}, func(ctx context.Context) []providers.Login { return providers.DiscoverLogins(ctx, r) }, opts)
}

func newFetcher(adapters map[model.Provider]providers.RemoteProvider, discover func(context.Context) []providers.Login, opts Options) *Fetcher {
	if opts.TTL <= 0 {
		opts.TTL = defaultTTL
	}
	if opts.LoginsTTL <= 0 {
		opts.LoginsTTL = defaultLoginsTTL
	}
	if opts.PerHost <= 0 {
		opts.PerHost = defaultPerHost
	}
	return &Fetcher{
		adapters: adapters, discover: discover, opts: opts, now: time.Now,
		entries: map[string]entry{}, hosts: map[string]*host{}, inflight: map[string]*call{},
	}
}

// Status возвращает состояние ветки: из кэша, если оно свежее, иначе от CLI.
// Проблема никогда не превращается в ноль: она в Availability и Error.
func (f *Fetcher) Status(ctx context.Context, req Request) Result {
	if req.Remote == nil {
		return failed("", model.Errorf(model.CodeNoRemote, "нет remote: серверных данных нет"), model.AvailabilityNotSupported)
	}
	if req.Branch == "" && req.SHA == "" {
		return failed("", model.Errorf(model.CodeInvalidInput, "нет текущей ветки (detached HEAD)"), model.AvailabilityNotSupported)
	}
	if req.Remote.Host == "" {
		// Локальный путь: сервера нет, логины CLI не нужны.
		return failed(model.ProviderOther, model.Errorf(model.CodeUnsupported, "remote %s — локальный путь, серверных данных нет", req.Remote.Name), model.AvailabilityNotSupported)
	}
	t, e := f.Resolve(ctx, *req.Remote, req.Overrides, req.Force)
	if e != nil {
		return failed(req.Remote.Provider, e, providers.AvailabilityOf(e.Code))
	}
	adapter := f.adapters[t.Provider]
	if adapter == nil {
		return failed(t.Provider, model.Errorf(model.CodeUnsupported, "для провайдера %q нет адаптера", t.Provider), model.AvailabilityNotSupported)
	}

	key := string(t.Provider) + "|" + t.Host + "|" + t.Project + "|" + t.Login + "|" + req.Branch + "|" + req.SHA
	f.mu.Lock()
	prev, cached := f.entries[key]
	if cached && !req.Force && f.now().Sub(prev.at) < f.opts.TTL {
		f.mu.Unlock()
		return prev.res
	}
	if c := f.inflight[key]; c != nil {
		f.mu.Unlock()
		select {
		case <-c.done:
			return c.res
		case <-ctx.Done():
			return failed(t.Provider, model.Errorf(model.CodeOpCancelled, "запрос отменён"), model.AvailabilityError)
		}
	}
	c := &call{done: make(chan struct{})}
	f.inflight[key] = c
	h := f.hostLocked(t.Host)
	f.mu.Unlock()

	res := f.fetch(ctx, h, adapter, providers.Query{Dir: req.Dir, Target: t, Branch: req.Branch, SHA: req.SHA}, req.Force)
	// Ошибка не стирает время последнего успеха: UI покажет, насколько данные устарели.
	if res.PR.LastSuccessAt == nil {
		res.PR.LastSuccessAt = prev.res.PR.LastSuccessAt
	}
	if res.CI.LastSuccessAt == nil {
		res.CI.LastSuccessAt = prev.res.CI.LastSuccessAt
	}

	f.mu.Lock()
	if model.CodeOf(res.PR.Error) != model.CodeOpCancelled {
		f.entries[key] = entry{res: res, at: f.now()}
	}
	delete(f.inflight, key)
	c.res = res
	close(c.done)
	f.mu.Unlock()
	return res
}

// fetch опрашивает адаптер с учётом ограничений хоста. Паника адаптера
// становится ошибкой этого запроса: иначе запрос остался бы «в полёте»
// навсегда и склеенные с ним ждали бы вечно.
func (f *Fetcher) fetch(ctx context.Context, h *host, a providers.RemoteProvider, q providers.Query, force bool) (res Result) {
	p := q.Target.Provider
	defer func() {
		if r := recover(); r != nil {
			res = failed(p, model.Errorf(model.CodeOther, "внутренняя ошибка адаптера %s: %v", p, r), model.AvailabilityError)
		}
	}()
	if e := f.gate(h, q.Target.Host, force); e != nil {
		return failed(p, e, providers.AvailabilityOf(e.Code))
	}
	select {
	case h.sem <- struct{}{}:
	case <-ctx.Done():
		return failed(p, model.Errorf(model.CodeOpCancelled, "запрос отменён"), model.AvailabilityError)
	}
	defer func() { <-h.sem }()
	// Пока ждали очереди, другой запрос мог узнать, что хост недоступен.
	if e := f.gate(h, q.Target.Host, force); e != nil {
		return failed(p, e, providers.AvailabilityOf(e.Code))
	}

	res = Result{Provider: p, PR: a.PullRequests(ctx, q)}
	if hostWide(res.PR.Error) {
		// Нет сети или лимит — второй запрос ответит так же, но позже.
		res.CI = model.CISummary{Availability: res.PR.Availability, Error: res.PR.Error, LastAttemptAt: res.PR.LastAttemptAt}
	} else {
		res.CI = a.CIRuns(ctx, q)
	}
	f.noteHost(h, res)
	return res
}

// hostWide — ошибка касается всего сервера, а не одного проекта.
func hostWide(e *model.Error) bool {
	switch model.CodeOf(e) {
	case model.CodeNetworkError, model.CodeTimeout, model.CodeRateLimit:
		return true
	}
	return false
}

// gate — нельзя ли сейчас обращаться к хосту: лимит действует всегда,
// сетевая ошибка — только без принудительного обновления.
func (f *Fetcher) gate(h *host, name string, force bool) *model.Error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	if now.Before(h.limitUntil) {
		return model.Errorf(model.CodeRateLimit, "%s ограничил частоту запросов; повтор после %s", name, h.limitUntil.Local().Format("15:04"))
	}
	if !force && h.down != nil && now.Before(h.downUntil) {
		e := *h.down
		return &e
	}
	return nil
}

// noteHost запоминает сетевую ошибку и лимит хоста; успешный ответ их снимает.
func (f *Fetcher) noteHost(h *host, res Result) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	for _, e := range []*model.Error{res.PR.Error, res.CI.Error} {
		switch model.CodeOf(e) {
		case model.CodeRateLimit:
			h.backoff = min(max(h.backoff*2, minBackoff), maxBackoff)
			h.limitUntil = now.Add(h.backoff)
			return
		case model.CodeNetworkError, model.CodeTimeout:
			h.down, h.downUntil = e, now.Add(f.opts.TTL)
			return
		}
	}
	h.backoff, h.down = 0, nil
}

func (f *Fetcher) hostLocked(name string) *host {
	h := f.hosts[name]
	if h == nil {
		h = &host{sem: make(chan struct{}, f.opts.PerHost)}
		f.hosts[name] = h
	}
	return h
}

// Resolve сопоставляет remote с проектом на сервере; логины CLI кэшируются.
func (f *Fetcher) Resolve(ctx context.Context, rm model.RemoteInfo, overrides map[string]model.Provider, force bool) (providers.Target, *model.Error) {
	return providers.Resolver{Overrides: overrides, Logins: f.loginsFor(ctx, force)}.Resolve(rm)
}

// loginsFor — логины gh, glab и tea. Читаются одним вызовом за раз: при
// запуске десятки репозиториев не должны запускать по три CLI каждый.
func (f *Fetcher) loginsFor(ctx context.Context, force bool) []providers.Login {
	f.loginsMu.Lock()
	defer f.loginsMu.Unlock()
	f.mu.Lock()
	age := f.now().Sub(f.loginsAt)
	fresh := !f.loginsAt.IsZero() && ((age < f.opts.LoginsTTL && !force) || age < loginsMinAge)
	logins := f.logins
	f.mu.Unlock()
	if fresh {
		return logins
	}
	logins = f.discover(ctx)
	f.mu.Lock()
	f.logins, f.loginsAt = logins, f.now()
	f.mu.Unlock()
	return logins
}

// Diagnose проверяет CLI и авторизацию для сервера цели.
func (f *Fetcher) Diagnose(ctx context.Context, dir string, t providers.Target) model.ToolDiagnostic {
	a := f.adapters[t.Provider]
	if a == nil {
		return model.ToolDiagnostic{Name: string(t.Provider), Host: t.Host, Availability: model.AvailabilityNotSupported}
	}
	d := a.Diagnose(ctx, dir, t)
	d.Host = t.Host
	return d
}

// failed — одинаковая ошибка для PR и CI.
func failed(p model.Provider, e *model.Error, a model.Availability) Result {
	at := time.Now()
	return Result{
		Provider: p,
		PR:       model.PRSummary{Availability: a, Error: e, LastAttemptAt: &at},
		CI:       model.CISummary{Availability: a, Error: e, LastAttemptAt: &at},
	}
}
