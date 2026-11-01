package providers

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// Gitea — адаптер `tea` (docs/providers.md, раздел Gitea). Каждая команда tea
// требует cwd внутри git-репозитория и явного логина (`-l`): логина по
// умолчанию может не быть, а на хосте их бывает несколько.
type Gitea struct {
	Runner runner.Runner
}

func (Gitea) Provider() model.Provider { return model.ProviderGitea }

var teaMarkers = []marker{
	{model.CodeNetworkError, "dial tcp"},
	{model.CodeNetworkError, "no such host"},
	{model.CodeNetworkError, "network is unreachable"},
	{model.CodeNetworkError, "i/o timeout"},
	{model.CodeNetworkError, "connection refused"},
	{model.CodeNetworkError, "x509:"},
	{model.CodeNetworkError, "TLS handshake"},
	{model.CodeAuthRequired, "no available login"},
	{model.CodeAuthRequired, "login name"}, // login name 'X' does not exist
	{model.CodeAuthRequired, "invalid username, password or token"},
	{model.CodeRepoUnresolved, "not found"},
	{model.CodeUnsupported, "is older than"},
}

func (g Gitea) exec(ctx context.Context, dir string, args ...string) (runner.Result, error) {
	return g.Runner.Run(ctx, runner.Command{Name: "tea", Args: args, Dir: dir, Timeout: cliTimeout})
}

func (g Gitea) run(ctx context.Context, dir string, args ...string) ([]byte, *model.Error) {
	res, err := g.exec(ctx, dir, args...)
	if err == nil && res.OK() {
		return res.Stdout, nil
	}
	return nil, classify("tea", res, err, teaMarkers)
}

// noLogin — у tea нет логина на этот сервер (резолвер не нашёл его в `tea logins`).
func noLogin(t Target) *model.Error {
	return model.Errorf(model.CodeAuthRequired, "у tea нет логина на %s: выполните tea login add", hostname(t.Host))
}

// api вызывает REST через `tea api -i`. tea выходит с 0 и на 401/404, поэтому
// успех определяется по строке статуса HTTP — первой строке stderr (D-041).
func (g Gitea) api(ctx context.Context, dir string, t Target, path string) ([]byte, *model.Error) {
	res, err := g.exec(ctx, dir, "api", "-l", t.Login, "-i", path)
	if err != nil || !res.OK() {
		return nil, classify("tea", res, err, teaMarkers)
	}
	code := httpStatus(res.Stderr)
	if code == http.StatusOK {
		return res.Stdout, nil
	}
	var body struct {
		Message string `json:"message"`
	}
	isJSON := json.Unmarshal(res.Stdout, &body) == nil
	msg := "tea: HTTP " + strconv.Itoa(code)
	if body.Message != "" {
		msg += ": " + body.Message
	}
	e := &model.Error{Code: model.CodeOther, Message: msg}
	switch {
	case code == 0:
		e.Message = "tea: нет строки статуса HTTP в ответе"
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		e.Code = model.CodeAuthRequired
	case code == http.StatusTooManyRequests:
		e.Code = model.CodeRateLimit
	case code == http.StatusNotFound && isJSON:
		e.Code = model.CodeRepoUnresolved
	case code == http.StatusNotFound:
		// Сам маршрут неизвестен серверу (`404 page not found` текстом):
		// эта версия Gitea не отдаёт запуски Actions по API (A18).
		e.Code = model.CodeUnsupported
		e.Message = "сервер Gitea не отдаёт запуски Actions по API (нужна версия с /actions/runs)"
	}
	return nil, e
}

// httpStatus — код из `HTTP/2.0 200 OK`; 0 — строки статуса нет.
func httpStatus(stderr []byte) int {
	f := strings.Fields(firstLine(string(stderr)))
	if len(f) < 2 || !strings.HasPrefix(f[0], "HTTP/") {
		return 0
	}
	code, _ := strconv.Atoi(f[1])
	return code
}

var versionRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// Diagnose проверяет tea и логин на сервере цели запросом `/user`.
func (g Gitea) Diagnose(ctx context.Context, dir string, t Target) model.ToolDiagnostic {
	d := model.ToolDiagnostic{Name: "tea"}
	fail := func(e *model.Error) model.ToolDiagnostic {
		d.Availability, d.Error = AvailabilityOf(e.Code), e
		return d
	}
	// `Version: 0.16.0	golang: …` — номер выделен ANSI-кодами даже при NO_COLOR.
	out, e := g.run(ctx, dir, "--version")
	if e != nil {
		return fail(e)
	}
	d.Version = versionRe.FindString(string(out))
	if t.Login == "" {
		return fail(noLogin(t))
	}
	if _, e := g.api(ctx, dir, t, "/user"); e != nil {
		return fail(e)
	}
	d.Availability = model.AvailabilityOK
	return d
}

type teaPR struct {
	Head  string `json:"head"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Страницы `tea pulls list`: фильтра по ветке нет, отбор на клиенте (D-040).
const (
	teaPageSize = 50
	teaMaxPages = 5
)

// teaDraft — Gitea считает PR черновиком по префиксу заголовка
// (по умолчанию `WIP:` и `[WIP]`, без учёта регистра).
func teaDraft(title string) bool {
	title = strings.ToUpper(strings.TrimSpace(title))
	return strings.HasPrefix(title, "WIP:") || strings.HasPrefix(title, "[WIP]")
}

// PullRequests — открытые PR, где ветка — источник. PR из форка tea не
// отличает (`head` — только имя ветки) — ограничение (docs/providers.md).
func (g Gitea) PullRequests(ctx context.Context, q Query) model.PRSummary {
	if q.Branch == "" {
		return prFailed(noBranch())
	}
	if q.Target.Login == "" {
		return prFailed(noLogin(q.Target))
	}
	var prs []teaPR
	for page := 1; ; page++ {
		if page > teaMaxPages {
			return prFailed(model.Errorf(model.CodeOther, "открытых PR больше %d: tea не фильтрует их по ветке, счёт был бы неполным", teaPageSize*teaMaxPages))
		}
		out, e := g.run(ctx, q.Dir, "pulls", "list", "-l", q.Target.Login, "-r", q.Target.Project, "--state", "open",
			"--limit", strconv.Itoa(teaPageSize), "--page", strconv.Itoa(page), "-f", "index,head,url,title", "-o", "json")
		if e != nil {
			return prFailed(e)
		}
		var list []teaPR
		if err := json.Unmarshal(out, &list); err != nil {
			return prFailed(badOutput("tea", err))
		}
		for _, p := range list {
			if p.Head == q.Branch {
				prs = append(prs, p)
			}
		}
		if len(list) < teaPageSize {
			break
		}
	}

	draft := 0
	for _, p := range prs {
		if teaDraft(p.Title) {
			draft++
		}
	}
	link := ""
	switch {
	case len(prs) == 1:
		link = prs[0].URL
	case len(prs) > 1:
		link = webURL(q.Target, "/pulls?state=open")
	}
	return withLast(prOK(len(prs), draft, link), func() *model.PRRef { return g.lastClosed(ctx, q) })
}

type teaBranch struct {
	Ref  string `json:"ref"`
	Repo *struct {
		ID int64 `json:"id"`
	} `json:"repo"`
}

type teaClosedPR struct {
	Number   int       `json:"number"`
	Merged   bool      `json:"merged"`
	MergedAt string    `json:"merged_at"`
	ClosedAt string    `json:"closed_at"`
	HTMLURL  string    `json:"html_url"`
	Head     teaBranch `json:"head"`
	Base     teaBranch `json:"base"`
}

// lastClosed — последний закрытый или влитый PR ветки через REST: в отличие
// от `tea pulls list`, там видно, из того ли репозитория PR. Читается одна
// страница недавно обновлённых закрытых PR — более старые не найдутся.
func (g Gitea) lastClosed(ctx context.Context, q Query) *model.PRRef {
	out, e := g.api(ctx, q.Dir, q.Target, "/repos/"+q.Target.Project+"/pulls?state=closed&sort=recentupdate&limit="+strconv.Itoa(teaPageSize))
	if e != nil {
		return nil
	}
	var prs []teaClosedPR
	if json.Unmarshal(out, &prs) != nil {
		return nil
	}
	var refs []model.PRRef
	for _, p := range prs {
		if p.Head.Ref != q.Branch || p.Head.Repo == nil || p.Base.Repo == nil || p.Head.Repo.ID != p.Base.Repo.ID {
			continue
		}
		at := p.ClosedAt
		if p.Merged && p.MergedAt != "" {
			at = p.MergedAt
		}
		refs = append(refs, model.PRRef{Number: p.Number, Merged: p.Merged, URL: p.HTMLURL, ClosedAt: parseTime(at)})
	}
	return latestPR(refs)
}

type teaRun struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Path       string `json:"path"`
	Event      string `json:"event"`
	HeadSHA    string `json:"head_sha"`
	HTMLURL    string `json:"html_url"`
}

// CIRuns — запуски Actions ветки через REST (D-041): формат как у GitHub,
// но `conclusion` нет, пока запуск не завершён. Итог — по правилам D-043.
func (g Gitea) CIRuns(ctx context.Context, q Query) model.CISummary {
	if q.Branch == "" {
		return ciFailed(noBranch())
	}
	if q.Target.Login == "" {
		return ciFailed(noLogin(q.Target))
	}
	out, e := g.api(ctx, q.Dir, q.Target, "/repos/"+q.Target.Project+"/actions/runs?branch="+url.QueryEscape(q.Branch)+"&limit=20")
	if e != nil {
		return ciFailed(e)
	}
	var v struct {
		Runs []teaRun `json:"workflow_runs"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return ciFailed(badOutput("tea", err))
	}
	runs := v.Runs
	// Новые первыми: id растёт с каждым запуском.
	slices.SortStableFunc(runs, func(a, b teaRun) int { return cmp.Compare(b.ID, a.ID) })

	active := 0
	states := make([]runState, len(runs))
	for i, r := range runs {
		if r.Status != "completed" {
			active++
		}
		// path — `ok.yml@refs/heads/B`: workflow — имя файла.
		wf, _, _ := strings.Cut(r.Path, "@")
		states[i] = runState{key: wf + "\x00" + r.Event, sha: r.HeadSHA, done: r.Status == "completed", conclusion: r.Conclusion}
	}
	link := ""
	if len(runs) > 0 {
		link = runs[0].HTMLURL
	}
	return ciOK(active, lastRunResult(states), link)
}
