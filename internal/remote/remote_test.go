package remote

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/providers"
)

// fakeAdapter отвечает заданной ошибкой (или успехом) и считает вызовы.
type fakeAdapter struct {
	mu      sync.Mutex
	err     map[string]*model.Error // проект → ошибка; нет — успех
	calls   atomic.Int32
	ciCalls atomic.Int32
	gate    chan struct{} // не nil — запрос ждёт, пока канал не закроют
	running atomic.Int32
	maxRun  atomic.Int32
}

func (a *fakeAdapter) Provider() model.Provider { return model.ProviderGitHub }

func (a *fakeAdapter) Diagnose(context.Context, string, providers.Target) model.ToolDiagnostic {
	return model.ToolDiagnostic{Name: "gh", Availability: model.AvailabilityOK}
}

func (a *fakeAdapter) PullRequests(ctx context.Context, q providers.Query) model.PRSummary {
	a.calls.Add(1)
	n := a.running.Add(1)
	for {
		m := a.maxRun.Load()
		if n <= m || a.maxRun.CompareAndSwap(m, n) {
			break
		}
	}
	defer a.running.Add(-1)
	if a.gate != nil {
		<-a.gate
	}
	a.mu.Lock()
	e := a.err[q.Target.Project]
	a.mu.Unlock()
	at := time.Now()
	if e != nil {
		return model.PRSummary{Availability: providers.AvailabilityOf(e.Code), Error: e, LastAttemptAt: &at}
	}
	one := 1
	return model.PRSummary{Availability: model.AvailabilityOK, Open: &one, Draft: new(int), LastSuccessAt: &at, LastAttemptAt: &at}
}

func (a *fakeAdapter) CIRuns(ctx context.Context, q providers.Query) model.CISummary {
	a.ciCalls.Add(1)
	at := time.Now()
	a.mu.Lock()
	e := a.err[q.Target.Project]
	a.mu.Unlock()
	if e != nil {
		return model.CISummary{Availability: providers.AvailabilityOf(e.Code), Error: e, LastAttemptAt: &at}
	}
	return model.CISummary{Availability: model.AvailabilityOK, Active: new(int), LastResult: "success", LastSuccessAt: &at, LastAttemptAt: &at}
}

func (a *fakeAdapter) fail(project string, code model.ErrorCode) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.err == nil {
		a.err = map[string]*model.Error{}
	}
	if code == "" {
		delete(a.err, project)
		return
	}
	a.err[project] = model.Errorf(code, "%s: ошибка", code)
}

// clock — управляемое время.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func setup(t *testing.T) (*Fetcher, *fakeAdapter, *clock, *atomic.Int32) {
	t.Helper()
	a := &fakeAdapter{}
	var discovered atomic.Int32
	f := newFetcher(map[model.Provider]providers.RemoteProvider{model.ProviderGitHub: a}, func(context.Context) []providers.Login {
		discovered.Add(1)
		return nil
	}, Options{})
	c := &clock{t: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)}
	f.now = c.now
	return f, a, c, &discovered
}

func req(project, branch string) Request {
	return Request{Dir: "/repo", Remote: &model.RemoteInfo{Name: "origin", FetchURL: "git@github.com:" + project + ".git", Host: "github.com", Path: project}, Branch: branch}
}

func TestCacheTTLAndForce(t *testing.T) {
	ctx := context.Background()
	f, a, c, _ := setup(t)

	res := f.Status(ctx, req("o/r", "main"))
	if res.Provider != model.ProviderGitHub || res.PR.Availability != model.AvailabilityOK || *res.PR.Open != 1 || res.CI.LastResult != "success" {
		t.Fatalf("первый запрос: %+v", res)
	}
	f.Status(ctx, req("o/r", "main"))
	c.add(defaultTTL - time.Second)
	f.Status(ctx, req("o/r", "main"))
	if n := a.calls.Load(); n != 1 {
		t.Errorf("в пределах TTL CLI вызван %d раз", n)
	}
	f.Status(ctx, req("o/r", "feature")) // другая ветка — другой ключ
	c.add(2 * time.Second)
	f.Status(ctx, req("o/r", "main")) // TTL истёк
	r := req("o/r", "main")
	r.Force = true
	f.Status(ctx, r) // ручное обновление
	if n := a.calls.Load(); n != 4 {
		t.Errorf("ожидалось 4 вызова CLI, было %d", n)
	}
}

func TestDetachedBySHA(t *testing.T) { // D-059
	ctx := context.Background()
	f, a, _, _ := setup(t)
	byCommit := func(sha string) Request {
		r := req("o/r", "")
		r.SHA = sha
		return r
	}
	if res := f.Status(ctx, byCommit("abc")); res.PR.Availability != model.AvailabilityOK || res.CI.Availability != model.AvailabilityOK {
		t.Fatalf("detached HEAD с коммитом: %+v", res)
	}
	f.Status(ctx, byCommit("abc"))
	f.Status(ctx, byCommit("def")) // другой коммит — другой ключ
	if n := a.calls.Load(); n != 2 {
		t.Errorf("ожидалось 2 вызова CLI, было %d", n)
	}
}

func TestSingleFlightAndPerHost(t *testing.T) {
	ctx := context.Background()
	f, a, _, discovered := setup(t)
	a.gate = make(chan struct{})

	var wg sync.WaitGroup
	results := make([]Result, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 4 одинаковых запроса и 4 разных веток одного хоста.
			branch := "same"
			if i%2 == 1 {
				branch = string(rune('a' + i))
			}
			results[i] = f.Status(ctx, req("o/r", branch))
		}()
	}
	time.Sleep(100 * time.Millisecond)
	close(a.gate)
	wg.Wait()

	if n := a.calls.Load(); n != 5 {
		t.Errorf("одинаковые запросы не склеены: %d вызовов вместо 5", n)
	}
	if m := a.maxRun.Load(); m > defaultPerHost {
		t.Errorf("к хосту одновременно %d запросов, лимит %d", m, defaultPerHost)
	}
	if n := discovered.Load(); n != 1 {
		t.Errorf("логины CLI прочитаны %d раз", n)
	}
	for i, r := range results {
		if r.PR.Availability != model.AvailabilityOK {
			t.Errorf("запрос %d: %+v", i, r.PR)
		}
	}
}

func TestHostDown(t *testing.T) { // A24
	ctx := context.Background()
	f, a, c, _ := setup(t)
	a.fail("o/a", model.CodeNetworkError)

	res := f.Status(ctx, req("o/a", "main"))
	if res.PR.Error == nil || res.PR.Error.Code != model.CodeNetworkError || res.CI.Error.Code != model.CodeNetworkError || res.PR.Open != nil {
		t.Fatalf("нет сети: %+v", res)
	}
	if a.ciCalls.Load() != 0 {
		t.Error("после сетевой ошибки PR запрошен ещё и CI")
	}
	// Другой проект того же хоста — без вызова CLI.
	res = f.Status(ctx, req("o/b", "main"))
	if res.PR.Error == nil || res.PR.Error.Code != model.CodeNetworkError || a.calls.Load() != 1 {
		t.Errorf("хост недоступен, но CLI вызван снова: %d, %+v", a.calls.Load(), res.PR)
	}
	// Ручное обновление пробует снова; сеть вернулась.
	a.fail("o/a", "")
	r := req("o/b", "main")
	r.Force = true
	if res = f.Status(ctx, r); res.PR.Availability != model.AvailabilityOK || a.calls.Load() != 2 {
		t.Errorf("принудительно: %+v", res.PR)
	}
	c.add(time.Second)
	if res = f.Status(ctx, req("o/c", "main")); res.PR.Availability != model.AvailabilityOK {
		t.Errorf("успех снял пометку хоста: %+v", res.PR)
	}
}

func TestRateLimitBackoff(t *testing.T) {
	ctx := context.Background()
	f, a, c, _ := setup(t)
	a.fail("o/r", model.CodeRateLimit)

	f.Status(ctx, req("o/r", "main"))
	r := req("o/r", "main")
	r.Force = true
	res := f.Status(ctx, r)
	if res.PR.Error == nil || res.PR.Error.Code != model.CodeRateLimit || a.calls.Load() != 1 {
		t.Errorf("при лимите запрос всё равно ушёл: %d, %+v", a.calls.Load(), res.PR.Error)
	}
	c.add(minBackoff + time.Second)
	f.Status(ctx, r) // снова лимит — пауза удваивается
	c.add(minBackoff + time.Second)
	f.Status(ctx, r)
	if n := a.calls.Load(); n != 2 {
		t.Errorf("пауза не удвоилась: %d вызовов", n)
	}
	c.add(minBackoff)
	a.fail("o/r", "")
	if res = f.Status(ctx, r); res.PR.Availability != model.AvailabilityOK || a.calls.Load() != 3 {
		t.Errorf("после паузы: %+v", res.PR)
	}
}

func TestLastSuccessKept(t *testing.T) {
	ctx := context.Background()
	f, a, c, _ := setup(t)
	ok := f.Status(ctx, req("o/r", "main"))
	a.fail("o/r", model.CodeAuthRequired)
	c.add(defaultTTL + time.Second)
	res := f.Status(ctx, req("o/r", "main"))
	if res.PR.Availability != model.AvailabilityUnavailable || res.PR.Open != nil {
		t.Errorf("ошибка не должна давать чисел: %+v", res.PR)
	}
	if res.PR.LastSuccessAt == nil || !res.PR.LastSuccessAt.Equal(*ok.PR.LastSuccessAt) || res.CI.LastSuccessAt == nil {
		t.Errorf("время последнего успеха потеряно: %+v", res.PR)
	}
}

func TestNotQueried(t *testing.T) { // A30
	ctx := context.Background()
	f, a, _, _ := setup(t)
	cases := []struct {
		name string
		req  Request
		code model.ErrorCode
	}{
		{"нет remote", Request{Branch: "main"}, model.CodeNoRemote},
		{"detached HEAD", req("o/r", ""), model.CodeInvalidInput},
		{"неизвестный сервер", Request{Remote: &model.RemoteInfo{Name: "origin", FetchURL: "git@git.example.org:o/r.git", Host: "git.example.org", Path: "o/r"}, Branch: "main"}, model.CodeUnsupported},
	}
	for _, c := range cases {
		res := f.Status(ctx, c.req)
		if res.PR.Availability != model.AvailabilityNotSupported || res.CI.Availability != model.AvailabilityNotSupported || res.PR.Error.Code != c.code {
			t.Errorf("%s: %+v", c.name, res)
		}
	}
	if a.calls.Load() != 0 {
		t.Error("CLI вызван без нужды")
	}
}

func TestLoginsRefresh(t *testing.T) {
	ctx := context.Background()
	f, _, c, discovered := setup(t)
	rm := model.RemoteInfo{Host: "github.com", Path: "o/r", FetchURL: "git@github.com:o/r.git"}
	f.Resolve(ctx, rm, nil, false)
	f.Resolve(ctx, rm, nil, true) // принудительно, но логины только что прочитаны
	c.add(loginsMinAge)
	f.Resolve(ctx, rm, nil, false) // ещё в пределах LoginsTTL
	f.Resolve(ctx, rm, nil, true)
	c.add(defaultLoginsTTL)
	f.Resolve(ctx, rm, nil, false)
	if n := discovered.Load(); n != 3 {
		t.Errorf("логины прочитаны %d раз, ожидалось 3", n)
	}
}

// panicAdapter — адаптер со сломанным разбором ответа CLI.
type panicAdapter struct{ fakeAdapter }

func (*panicAdapter) PullRequests(context.Context, providers.Query) model.PRSummary {
	panic("сломанный разбор ответа")
}

// Паника адаптера — ошибка этого запроса; склеенные запросы и следующие не зависают (11.4).
func TestAdapterPanicIsolated(t *testing.T) {
	a := &panicAdapter{}
	f := newFetcher(map[model.Provider]providers.RemoteProvider{model.ProviderGitHub: a}, func(context.Context) []providers.Login { return nil }, Options{})
	done := make(chan Result, 3)
	for range 3 {
		go func() {
			done <- f.Status(context.Background(), Request{Dir: "/repo", Remote: req("o/r", "main").Remote, Branch: "main", Force: true})
		}()
	}
	for range 3 {
		select {
		case res := <-done:
			if res.PR.Availability != model.AvailabilityError || res.CI.Availability != model.AvailabilityError || model.CodeOf(res.PR.Error) != model.CodeOther {
				t.Fatalf("ожидалась ошибка: %+v", res)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("запрос завис после паники адаптера")
		}
	}
}
