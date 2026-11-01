package providers

import (
	"cmp"
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"strings"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// GitHub — адаптер `gh` (docs/providers.md, раздел GitHub).
type GitHub struct {
	Runner runner.Runner
}

func (GitHub) Provider() model.Provider { return model.ProviderGitHub }

// ghMarkers — признаки ошибок в stderr gh. Лимит проверяется раньше
// авторизации: превышение приходит как `HTTP 403: API rate limit exceeded`.
var ghMarkers = []marker{
	{model.CodeRateLimit, "rate limit"},
	{model.CodeRateLimit, "HTTP 429"},
	{model.CodeNetworkError, "error connecting to"},
	{model.CodeNetworkError, "dial tcp"},
	{model.CodeNetworkError, "i/o timeout"},
	{model.CodeNetworkError, "TLS handshake"},
	{model.CodeNetworkError, "x509:"},
	{model.CodeAuthRequired, "HTTP 401"},
	{model.CodeAuthRequired, "Bad credentials"},
	{model.CodeAuthRequired, "gh auth login"},
	{model.CodeAccessDenied, "HTTP 403"}, // после лимита: превышение — тоже 403
	{model.CodeRepoUnresolved, "Could not resolve to a Repository"},
	{model.CodeRepoUnresolved, "HTTP 404"},
	{model.CodeUnsupported, "Unknown JSON field"},
}

// ghExitAuth — код выхода gh, когда логина нет совсем.
const ghExitAuth = 4

func (g GitHub) run(ctx context.Context, dir string, args ...string) ([]byte, *model.Error) {
	res, err := g.Runner.Run(ctx, runner.Command{Name: "gh", Args: args, Dir: dir, Timeout: cliTimeout})
	switch {
	case err == nil && res.OK():
		return res.Stdout, nil
	case err == nil && res.ExitCode == ghExitAuth:
		return nil, &model.Error{Code: model.CodeAuthRequired, Message: "gh не авторизован: выполните gh auth login", ExitCode: res.ExitCode}
	}
	return nil, classify("gh", res, err, ghMarkers)
}

// repoArg — `-R HOST/OWNER/REPO`: хост нужен для GitHub Enterprise.
func repoArg(t Target) string { return t.Host + "/" + t.Project }

// webURL — адрес страницы проекта на сервере.
func webURL(t Target, rest string) string {
	base := t.BaseURL
	if base == "" {
		base = "https://" + t.Host
	}
	return base + "/" + t.Project + rest
}

// Diagnose проверяет gh и логин на хосте цели. `auth status --json` без сети
// честно отдаёт сетевую ошибку (текстовый вывод пишет «token invalid»).
func (g GitHub) Diagnose(ctx context.Context, _ string, t Target) model.ToolDiagnostic {
	d := model.ToolDiagnostic{Name: "gh"}
	fail := func(e *model.Error) model.ToolDiagnostic {
		d.Availability, d.Error = AvailabilityOf(e.Code), e
		return d
	}
	out, e := g.run(ctx, "", "--version")
	if e != nil {
		return fail(e)
	}
	// `gh version 2.103.3 (2026-10-26)`
	if f := strings.Fields(firstLine(string(out))); len(f) >= 3 {
		d.Version = f[2]
	}

	host := hostname(t.Host)
	out, e = g.run(ctx, "", "auth", "status", "--hostname", host, "--json", "hosts")
	if e != nil {
		return fail(e)
	}
	var v struct {
		Hosts map[string][]struct {
			State  string `json:"state"`
			Error  string `json:"error"`
			Active bool   `json:"active"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return fail(badOutput("gh", err))
	}
	accounts := v.Hosts[host]
	if len(accounts) == 0 {
		return fail(model.Errorf(model.CodeAuthRequired, "gh не авторизован на %s: выполните gh auth login --hostname %s", host, host))
	}
	acc := accounts[0]
	for _, a := range accounts {
		if a.Active {
			acc = a
		}
	}
	if acc.State != "success" {
		// Без признака сети — логин недействителен.
		return fail(classify("gh", runner.Result{Stderr: []byte(acc.Error)}, nil, slices.Concat(ghMarkers, []marker{{model.CodeAuthRequired, ""}})))
	}
	d.Availability = model.AvailabilityOK
	return d
}

type ghPR struct {
	IsDraft           bool   `json:"isDraft"`
	URL               string `json:"url"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	HeadRefName       string `json:"headRefName"`
}

// ghPRLimit — больше открытых PR из одной ветки не бывает на практике.
const ghPRLimit = "100"

// ghSelect — отбор PR: по ветке-источнику или, при detached HEAD, поиском
// по SHA (находит PR, где этот коммит есть среди коммитов).
func ghSelect(q Query) []string {
	if q.Branch != "" {
		return []string{"--head", q.Branch}
	}
	return []string{"--search", q.SHA}
}

// ghOwn — PR из этого репозитория и, если отбор по ветке, именно из неё:
// `--head` фильтрует только по имени, PR из форков с тем же именем отбрасываются.
func ghOwn(q Query, crossRepo bool, head string) bool {
	return !crossRepo && (q.Branch == "" || head == q.Branch)
}

// PullRequests — открытые PR, где ветка — источник (при detached HEAD —
// содержащие текущий коммит).
func (g GitHub) PullRequests(ctx context.Context, q Query) model.PRSummary {
	if q.noRef() {
		return prFailed(noBranch())
	}
	out, e := g.run(ctx, q.Dir, slices.Concat([]string{"pr", "list", "-R", repoArg(q.Target)}, ghSelect(q),
		[]string{"--state", "open", "--limit", ghPRLimit, "--json", "isDraft,url,isCrossRepository,headRefName"})...)
	if e != nil {
		return prFailed(e)
	}
	var prs []ghPR
	if err := json.Unmarshal(out, &prs); err != nil {
		return prFailed(badOutput("gh", err))
	}
	prs = slices.DeleteFunc(prs, func(p ghPR) bool { return !ghOwn(q, p.IsCrossRepository, p.HeadRefName) })

	draft := 0
	for _, p := range prs {
		if p.IsDraft {
			draft++
		}
	}
	link := ""
	switch {
	case len(prs) == 1:
		link = prs[0].URL
	case len(prs) > 1:
		filter := "head:" + q.Branch
		if q.Branch == "" {
			filter = q.SHA
		}
		link = webURL(q.Target, "/pulls?q="+url.QueryEscape("is:pr is:open "+filter))
	}
	return withLast(prOK(len(prs), draft, link), func() *model.PRRef { return g.lastClosed(ctx, q) })
}

type ghClosedPR struct {
	Number            int    `json:"number"`
	State             string `json:"state"`
	URL               string `json:"url"`
	ClosedAt          string `json:"closedAt"`
	MergedAt          string `json:"mergedAt"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	HeadRefName       string `json:"headRefName"`
}

// lastClosed — последний закрытый или влитый PR ветки; `--state closed`
// включает и влитые.
func (g GitHub) lastClosed(ctx context.Context, q Query) *model.PRRef {
	out, e := g.run(ctx, q.Dir, slices.Concat([]string{"pr", "list", "-R", repoArg(q.Target)}, ghSelect(q),
		[]string{"--state", "closed", "--limit", "20", "--json", "number,state,url,closedAt,mergedAt,isCrossRepository,headRefName"})...)
	if e != nil {
		return nil
	}
	var prs []ghClosedPR
	if json.Unmarshal(out, &prs) != nil {
		return nil
	}
	var refs []model.PRRef
	for _, p := range prs {
		if !ghOwn(q, p.IsCrossRepository, p.HeadRefName) {
			continue
		}
		merged := p.MergedAt != "" || p.State == "MERGED"
		at := p.ClosedAt
		if p.MergedAt != "" {
			at = p.MergedAt
		}
		refs = append(refs, model.PRRef{Number: p.Number, Merged: merged, URL: p.URL, ClosedAt: parseTime(at)})
	}
	return latestPR(refs)
}

type ghRun struct {
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	WorkflowName string `json:"workflowName"`
	Event        string `json:"event"`
	HeadSha      string `json:"headSha"`
	CreatedAt    string `json:"createdAt"`
}

// ghActive — незавершённые статусы запуска (по `gh run list --help`).
var ghActive = []string{"queued", "in_progress", "requested", "waiting", "pending"}

// ghRunLimit — сколько последних запусков ветки читать.
const ghRunLimit = "20"

// CIRuns — запуски Actions ветки (при detached HEAD — коммита): активные
// считаются по всем прочитанным, итог — по последнему коммиту (ghLastResult).
func (g GitHub) CIRuns(ctx context.Context, q Query) model.CISummary {
	if q.noRef() {
		return ciFailed(noBranch())
	}
	sel, link := []string{"--branch", q.Branch}, webURL(q.Target, "/actions?query="+url.QueryEscape("branch:"+q.Branch))
	if q.Branch == "" {
		sel, link = []string{"--commit", q.SHA}, webURL(q.Target, "/commit/"+q.SHA)
	}
	out, e := g.run(ctx, q.Dir, slices.Concat([]string{"run", "list", "-R", repoArg(q.Target)}, sel,
		[]string{"--limit", ghRunLimit, "--json", "status,conclusion,workflowName,event,headSha,createdAt"})...)
	if e != nil {
		return ciFailed(e)
	}
	var runs []ghRun
	if err := json.Unmarshal(out, &runs); err != nil {
		return ciFailed(badOutput("gh", err))
	}
	active := 0
	for _, r := range runs {
		if slices.Contains(ghActive, r.Status) {
			active++
		}
	}
	if len(runs) == 0 {
		link = ""
	}
	return ciOK(active, ghLastResult(runs), link)
}

// ghLastResult — итог последнего коммита ветки по правилам D-043 (lastRunResult).
func ghLastResult(runs []ghRun) string {
	runs = slices.Clone(runs)
	// RFC 3339 в UTC сравнивается как строка; новые — первыми.
	slices.SortStableFunc(runs, func(a, b ghRun) int { return cmp.Compare(b.CreatedAt, a.CreatedAt) })
	states := make([]runState, len(runs))
	for i, r := range runs {
		states[i] = runState{key: r.WorkflowName + "\x00" + r.Event, sha: r.HeadSha, done: r.Status == "completed", conclusion: r.Conclusion}
	}
	return lastRunResult(states)
}
