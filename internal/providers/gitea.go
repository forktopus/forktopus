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

// Gitea — адаптер Gitea через `gt` или, если у gt нет входа на сервер, через
// `tea` (D-074; docs/providers.md, разделы Gitea). CLI выбирает резолвер по
// логину (Target.Tool). gt получает инстанс и пользователя явно (`--host`,
// `--user`); каждая команда tea требует cwd внутри git-репозитория и явного
// логина (`-l`): логина по умолчанию может не быть, а на хосте их бывает несколько.
type Gitea struct {
	Runner runner.Runner
}

func (Gitea) Provider() model.Provider { return model.ProviderGitea }

var teaMarkers = []marker{
	// Запрет аккаунта важнее общего отказа прав HTTP 403.
	{model.CodeAccountBlocked, "This account is prohibited from signing in"},
	{model.CodeAccountBlocked, "This account is not activated"},
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

// gtMarkers — тексты ошибок gt 0.2.0 (docs/providers.md, раздел gt); нет
// аккаунта или учётка отклонена — ещё и код выхода 4.
var gtMarkers = []marker{
	// Запрет аккаунта важнее общего отказа прав HTTP 403.
	{model.CodeAccountBlocked, "This account is prohibited from signing in"},
	{model.CodeAccountBlocked, "This account is not activated"},
	{model.CodeNetworkError, "cannot reach"},
	{model.CodeNetworkError, "dial tcp"},
	{model.CodeNetworkError, "x509:"},
	{model.CodeAuthRequired, "no account for"},
	{model.CodeAuthRequired, "is not logged in"},
	{model.CodeAuthRequired, "log in again"},
	{model.CodeAuthRequired, "run gt auth login"},
	// 403 gt печатает без статуса HTTP: вход есть, но роли не хватает прав на
	// раздел, например Actions (D-068).
	{model.CodeAccessDenied, "permission denied by the gitea instance"},
	{model.CodeRepoUnresolved, "not found"},
	{model.CodeUnsupported, "is older than"},
}

// gtAuthExit — код выхода gt «нет аккаунта или учётка отклонена».
const gtAuthExit = 4

func (g Gitea) exec(ctx context.Context, dir string, t Target, args ...string) (runner.Result, error) {
	name := CLI(t)
	if name == "gt" {
		args = append(args, "--host", t.BaseURL, "--user", t.Login, "--no-input")
	}
	return g.Runner.Run(ctx, runner.Command{Name: name, Args: args, Dir: dir, Timeout: cliTimeout})
}

func (g Gitea) run(ctx context.Context, dir string, t Target, args ...string) ([]byte, *model.Error) {
	res, err := g.exec(ctx, dir, t, args...)
	if err == nil && res.OK() {
		return res.Stdout, nil
	}
	return nil, g.classify(t, res, err)
}

func (g Gitea) classify(t Target, res runner.Result, err error) *model.Error {
	if CLI(t) == "tea" {
		return classify("tea", res, err, teaMarkers)
	}
	e := classify("gt", res, err, gtMarkers)
	if err == nil && res.ExitCode == gtAuthExit && e.Code != model.CodeAccountBlocked {
		e.Code = model.CodeAuthRequired
	}
	return e
}

// noLogin — ни у gt, ни у tea нет логина на этот сервер (резолвер не нашёл его
// в `gt auth status` и `tea logins`). Советуется gt: tea архивирован.
func noLogin(t Target) *model.Error {
	return model.Errorf(model.CodeAuthRequired, "у gt нет входа на %s: выполните gt auth login --host https://%s", hostname(t.Host), t.Host)
}

// api вызывает REST через `gt api` или `tea api -i`.
func (g Gitea) api(ctx context.Context, dir string, t Target, path string) ([]byte, *model.Error) {
	if CLI(t) == "gt" {
		return g.gtAPI(ctx, dir, t, path)
	}
	// tea выходит с 0 и на 401/404, поэтому успех определяется по строке
	// статуса HTTP — первой строке stderr (D-041).
	res, err := g.exec(ctx, dir, t, "api", "-l", t.Login, "-i", path)
	if err != nil || !res.OK() {
		return nil, g.classify(t, res, err)
	}
	code := httpStatus(res.Stderr)
	if code == http.StatusOK {
		return res.Stdout, nil
	}
	if code == 0 {
		return nil, model.Errorf(model.CodeOther, "tea: нет строки статуса HTTP в ответе")
	}
	return nil, statusError("tea", code, res.Stdout)
}

// gtStatusRe — статус в ошибке `gt api`: `Error: not found (HTTP 404 GET /api/v1/…)`.
var gtStatusRe = regexp.MustCompile(`\(HTTP (\d{3}) `)

// gtAPI — REST через `gt api`: путь — без /api/v1. Ответ вне 2xx — код 1,
// тело в stdout, статус — в тексте ошибки.
func (g Gitea) gtAPI(ctx context.Context, dir string, t Target, path string) ([]byte, *model.Error) {
	res, err := g.exec(ctx, dir, t, "api", strings.TrimPrefix(path, "/"))
	if err == nil && res.OK() {
		return res.Stdout, nil
	}
	if err == nil && res.ExitCode != gtAuthExit {
		if m := gtStatusRe.FindSubmatch(res.Stderr); m != nil {
			code, _ := strconv.Atoi(string(m[1]))
			return nil, statusError("gt", code, res.Stdout)
		}
	}
	return nil, g.classify(t, res, err)
}

// statusError — ответ REST Gitea вне 2xx.
func statusError(cli string, code int, out []byte) *model.Error {
	var body struct {
		Message string `json:"message"`
	}
	isJSON := json.Unmarshal(out, &body) == nil
	msg := cli + ": HTTP " + strconv.Itoa(code)
	if body.Message != "" {
		msg += ": " + body.Message
	}
	e := &model.Error{Code: model.CodeOther, Message: msg}
	switch {
	case code == http.StatusUnauthorized:
		e.Code = model.CodeAuthRequired
	case code == http.StatusForbidden && giteaAccountBlocked(body.Message):
		e.Code = model.CodeAccountBlocked
	case code == http.StatusForbidden:
		// Вход есть, но роли не хватает прав на раздел (например, Actions при
		// доступе к коду и PR) — не проблема настройки CLI (D-068).
		e.Code = model.CodeAccessDenied
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
	return e
}

// giteaAccountBlocked распознаёт тот же ответ в JSON, который gt/tea могут
// печатать текстом. Поля учётной записи по этому сообщению различить нельзя.
func giteaAccountBlocked(message string) bool {
	text := strings.ToLower(message)
	return strings.Contains(text, "this account is prohibited from signing in") ||
		strings.Contains(text, "this account is not activated")
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

// Diagnose проверяет CLI цели и логин на сервере запросом `/user`. Логина нет
// ни у gt, ни у tea — проверяется gt: ставить советуем его.
func (g Gitea) Diagnose(ctx context.Context, dir string, t Target) model.ToolDiagnostic {
	d := model.ToolDiagnostic{Name: CLI(t)}
	fail := func(e *model.Error) model.ToolDiagnostic {
		d.Availability, d.Error = AvailabilityOf(e.Code), e
		return d
	}
	// tea: `Version: 0.16.0	golang: …` — номер выделен ANSI-кодами даже при
	// NO_COLOR; gt: `gt version 0.2.0 (2026-10-26)`.
	res, err := g.Runner.Run(ctx, runner.Command{Name: d.Name, Args: []string{"--version"}, Dir: dir, Timeout: cliTimeout})
	if err != nil || !res.OK() {
		return fail(g.classify(t, res, err))
	}
	out := res.Stdout
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
// При detached HEAD — PR, у которых текущий коммит — последний (commitPRs).
func (g Gitea) PullRequests(ctx context.Context, q Query) (result model.PRSummary) {
	defer prListLink(q, &result)
	if q.noRef() {
		return prFailed(noBranch())
	}
	if q.Target.Login == "" {
		return prFailed(noLogin(q.Target))
	}
	// `gt pr list --head` отбирает на клиенте и читает только первую страницу,
	// поэтому у gt — тот же REST, что для detached HEAD у tea.
	if q.Branch == "" || CLI(q.Target) == "gt" {
		return g.restPRs(ctx, q)
	}
	var prs []teaPR
	for page := 1; ; page++ {
		if page > teaMaxPages {
			return prFailed(model.Errorf(model.CodeOther, "открытых PR больше %d: tea не фильтрует их по ветке, счёт был бы неполным", teaPageSize*teaMaxPages))
		}
		out, e := g.run(ctx, q.Dir, q.Target, "pulls", "list", "-l", q.Target.Login, "-r", q.Target.Project, "--state", "open",
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
	SHA  string `json:"sha"`
	Repo *struct {
		ID int64 `json:"id"`
	} `json:"repo"`
}

// teaRESTPR — PR в ответе REST (`/pulls`); в отличие от `tea pulls list`,
// там видны репозитории head и base, коммит head и флаг черновика.
type teaRESTPR struct {
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	Draft    bool      `json:"draft"`
	Merged   bool      `json:"merged"`
	MergedAt string    `json:"merged_at"`
	ClosedAt string    `json:"closed_at"`
	HTMLURL  string    `json:"html_url"`
	Head     teaBranch `json:"head"`
	Base     teaBranch `json:"base"`
}

// teaOwn — PR из этого репозитория (у PR из удалённого форка head.repo — null)
// и из ветки запроса, а при detached HEAD — с текущим коммитом на конце.
func teaOwn(q Query, p teaRESTPR) bool {
	if p.Head.Repo == nil || p.Base.Repo == nil || p.Head.Repo.ID != p.Base.Repo.ID {
		return false
	}
	if q.Branch == "" {
		return p.Head.SHA == q.SHA
	}
	return p.Head.Ref == q.Branch
}

// lastClosed — последний закрытый или влитый PR ветки через REST: в отличие
// от `tea pulls list`, там видно, из того ли репозитория PR. Читается одна
// страница недавно обновлённых закрытых PR — более старые не найдутся.
func (g Gitea) lastClosed(ctx context.Context, q Query) *model.PRRef {
	out, e := g.api(ctx, q.Dir, q.Target, "/repos/"+q.Target.Project+"/pulls?state=closed&sort=recentupdate&limit="+strconv.Itoa(teaPageSize))
	if e != nil {
		return nil
	}
	var prs []teaRESTPR
	if json.Unmarshal(out, &prs) != nil {
		return nil
	}
	var refs []model.PRRef
	for _, p := range prs {
		if !teaOwn(q, p) {
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

// restPRs — открытые PR ветки из этого репозитория через REST (gt), а при
// detached HEAD — PR, у которых head — текущий коммит: найти PR, где коммит в
// середине, Gitea по API не позволяет. Страницы — как у PullRequests (D-040).
func (g Gitea) restPRs(ctx context.Context, q Query) model.PRSummary {
	var prs []teaRESTPR
	for page := 1; ; page++ {
		if page > teaMaxPages {
			what := "сервер не фильтрует их по ветке, счёт"
			if q.Branch == "" {
				what = "поиск по коммиту"
			}
			return prFailed(model.Errorf(model.CodeOther, "открытых PR больше %d: %s был бы неполным", teaPageSize*teaMaxPages, what))
		}
		out, e := g.api(ctx, q.Dir, q.Target, "/repos/"+q.Target.Project+"/pulls?state=open&limit="+strconv.Itoa(teaPageSize)+"&page="+strconv.Itoa(page))
		if e != nil {
			return prFailed(e)
		}
		var list []teaRESTPR
		if err := json.Unmarshal(out, &list); err != nil {
			return prFailed(badOutput(CLI(q.Target), err))
		}
		for _, p := range list {
			if teaOwn(q, p) {
				prs = append(prs, p)
			}
		}
		if len(list) < teaPageSize {
			break
		}
	}
	draft := 0
	for _, p := range prs {
		if p.Draft || teaDraft(p.Title) {
			draft++
		}
	}
	link := ""
	switch {
	case len(prs) == 1:
		link = prs[0].HTMLURL
	case len(prs) > 1:
		link = webURL(q.Target, "/pulls?state=open")
	}
	return withLast(prOK(len(prs), draft, link), func() *model.PRRef { return g.lastClosed(ctx, q) })
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

// CIRuns — запуски Actions ветки (при detached HEAD — коммита) через REST
// (D-041): формат как у GitHub, но `conclusion` нет, пока запуск не
// завершён. Итог — по правилам D-043.
func (g Gitea) CIRuns(ctx context.Context, q Query) (result model.CISummary) {
	defer ciListLink(q, &result)
	if q.noRef() {
		return ciFailed(noBranch())
	}
	if q.Target.Login == "" {
		return ciFailed(noLogin(q.Target))
	}
	filter := "branch=" + url.QueryEscape(q.Branch)
	if q.Branch == "" {
		filter = "head_sha=" + q.SHA
	}
	out, e := g.api(ctx, q.Dir, q.Target, "/repos/"+q.Target.Project+"/actions/runs?"+filter+"&limit=20")
	if e != nil {
		return ciFailed(e)
	}
	var v struct {
		Runs []teaRun `json:"workflow_runs"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return ciFailed(badOutput(CLI(q.Target), err))
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
