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
	{model.CodeAuthRequired, "HTTP 403"},
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
func (g GitHub) Diagnose(ctx context.Context, t Target) model.ToolDiagnostic {
	d := model.ToolDiagnostic{Name: "gh"}
	fail := func(e *model.Error) model.ToolDiagnostic {
		d.Availability, d.Error = availabilityOf(e.Code), e
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

// PullRequests — открытые PR, где ветка — источник. `--head` фильтрует только
// по имени ветки, поэтому PR из форков с тем же именем отбрасываются.
func (g GitHub) PullRequests(ctx context.Context, q Query) model.PRSummary {
	if q.Branch == "" {
		return prFailed(noBranch())
	}
	out, e := g.run(ctx, q.Dir, "pr", "list", "-R", repoArg(q.Target), "--head", q.Branch, "--state", "open",
		"--limit", ghPRLimit, "--json", "isDraft,url,isCrossRepository,headRefName")
	if e != nil {
		return prFailed(e)
	}
	var prs []ghPR
	if err := json.Unmarshal(out, &prs); err != nil {
		return prFailed(badOutput("gh", err))
	}
	prs = slices.DeleteFunc(prs, func(p ghPR) bool { return p.IsCrossRepository || p.HeadRefName != q.Branch })

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
		link = webURL(q.Target, "/pulls?q="+url.QueryEscape("is:pr is:open head:"+q.Branch))
	}
	return prOK(len(prs), draft, link)
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

// CIRuns — запуски Actions ветки: активные считаются по всем прочитанным,
// итог — по последнему коммиту (ghLastResult).
func (g GitHub) CIRuns(ctx context.Context, q Query) model.CISummary {
	if q.Branch == "" {
		return ciFailed(noBranch())
	}
	out, e := g.run(ctx, q.Dir, "run", "list", "-R", repoArg(q.Target), "--branch", q.Branch,
		"--limit", ghRunLimit, "--json", "status,conclusion,workflowName,event,headSha,createdAt")
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
	link := ""
	if len(runs) > 0 {
		link = webURL(q.Target, "/actions?query="+url.QueryEscape("branch:"+q.Branch))
	}
	return ciOK(active, ghLastResult(runs), link)
}

// ghOK — завершения, которые GitHub не считает провалом.
var ghOK = []string{"success", "neutral", "skipped"}

// ghLastResult — итог последнего коммита ветки: по каждому workflow (и событию)
// берётся его последний запуск на этом коммите. Повторный успешный запуск
// перекрывает упавший; незавершённый не даёт итога. Провал любого workflow,
// в том числе cancelled и timed_out, — failure, как в сводке GitHub.
func ghLastResult(runs []ghRun) string {
	if len(runs) == 0 {
		return ""
	}
	runs = slices.Clone(runs)
	// RFC 3339 в UTC сравнивается как строка; новые — первыми.
	slices.SortStableFunc(runs, func(a, b ghRun) int { return cmp.Compare(b.CreatedAt, a.CreatedAt) })
	head := runs[0].HeadSha
	seen := map[string]bool{}
	result := ""
	for _, r := range runs {
		key := r.WorkflowName + "\x00" + r.Event
		if r.HeadSha != head || seen[key] {
			continue
		}
		seen[key] = true
		switch {
		case r.Status != "completed":
		case !slices.Contains(ghOK, r.Conclusion):
			return "failure"
		default:
			result = "success"
		}
	}
	return result
}
