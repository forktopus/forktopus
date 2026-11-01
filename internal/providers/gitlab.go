package providers

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// GitLab — адаптер `glab` (docs/providers.md, раздел GitLab).
type GitLab struct {
	Runner runner.Runner
}

func (GitLab) Provider() model.Provider { return model.ProviderGitLab }

// glMarkers — признаки ошибок в выводе glab. Без сети glab повторяет DNS-запросы
// и отвечает через ~12 с, отсюда cliTimeout с запасом.
var glMarkers = []marker{
	{model.CodeRateLimit, "429 Too Many Requests"},
	{model.CodeRateLimit, "rate limit"},
	{model.CodeNetworkError, "dial tcp"},
	{model.CodeNetworkError, "no such host"},
	{model.CodeNetworkError, "network is unreachable"},
	{model.CodeNetworkError, "i/o timeout"},
	{model.CodeNetworkError, "connection refused"},
	{model.CodeNetworkError, "x509:"},
	{model.CodeAuthRequired, "401 Unauthorized"},
	{model.CodeAuthRequired, "has not been authenticated"},
	{model.CodeAuthRequired, "glab auth login"},
	{model.CodeAuthRequired, "403 Forbidden"},
	{model.CodeRepoUnresolved, "404 Not Found"},
	{model.CodeRepoUnresolved, "404 Project Not Found"},
	{model.CodeUnsupported, "unknown flag"},
}

func (g GitLab) exec(ctx context.Context, dir string, args ...string) (runner.Result, error) {
	return g.Runner.Run(ctx, runner.Command{Name: "glab", Args: args, Dir: dir, Timeout: cliTimeout})
}

func (g GitLab) run(ctx context.Context, dir string, args ...string) ([]byte, *model.Error) {
	res, err := g.exec(ctx, dir, args...)
	if err == nil && res.OK() {
		return res.Stdout, nil
	}
	return nil, glFailure(res, err)
}

// glFailure — ошибка неудачного запуска glab. С -F json glab пишет ошибку
// JSON-ом в stdout, а текст — в stderr; stdout разбирается только ради сообщения.
func glFailure(res runner.Result, err error) *model.Error {
	var v struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	res.Stderr = []byte(unwrapGlab(string(res.Stderr)))
	if err == nil && json.Unmarshal(res.Stdout, &v) == nil && v.Error.Message != "" {
		res.Stderr = append([]byte(v.Error.Message+"\n"), res.Stderr...)
	}
	return classify("glab", res, err, glMarkers)
}

// unwrapGlab склеивает ошибку glab в одну строку: glab печатает её в рамке
// «ERROR» и переносит по ширине ~120 символов (COLUMNS не учитывается),
// разрывая и URL, и признаки вроде «dial tcp».
func unwrapGlab(s string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "ERROR" {
			continue
		}
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte(' ')
		}
		b.WriteString(line)
	}
	return b.String()
}

// glRepo — `-R` полным URL проекта: так работает и self-hosted с подпутём.
func glRepo(t Target) string { return webURL(t, "") }

// glHost — хост для `auth status`: как он записан в логине glab.
func glHost(t Target) string {
	if t.Login != "" {
		return t.Login
	}
	return hostname(t.Host)
}

type glAuthHost struct {
	Host  string `json:"host"`
	State string `json:"state"`
	Error string `json:"error"`
}

// Diagnose проверяет glab и логин на хосте цели. Как и у gh, JSON-вывод
// `auth status` без сети отдаёт сетевую ошибку, а не «нет авторизации».
func (g GitLab) Diagnose(ctx context.Context, _ string, t Target) model.ToolDiagnostic {
	d := model.ToolDiagnostic{Name: "glab"}
	fail := func(e *model.Error) model.ToolDiagnostic {
		d.Availability, d.Error = AvailabilityOf(e.Code), e
		return d
	}
	out, e := g.run(ctx, "", "--version")
	if e != nil {
		return fail(e)
	}
	// `glab 1.126.0 (4dd0d32d1)`
	if f := strings.Fields(firstLine(string(out))); len(f) >= 2 {
		d.Version = f[1]
	}

	// Если логин не прошёл проверку, glab выходит с 1, но JSON со статусом
	// хоста всё равно печатает — его и разбираем.
	host := glHost(t)
	var v struct {
		Hosts []glAuthHost `json:"hosts"`
	}
	res, err := g.exec(ctx, "", "auth", "status", "--hostname", host, "--output", "json")
	if err != nil || json.Unmarshal(res.Stdout, &v) != nil || v.Hosts == nil {
		if err == nil && res.OK() {
			return fail(badOutput("glab", errors.New("нет списка hosts")))
		}
		return fail(glFailure(res, err))
	}
	i := slices.IndexFunc(v.Hosts, func(h glAuthHost) bool { return strings.EqualFold(h.Host, host) })
	if i < 0 {
		return fail(model.Errorf(model.CodeAuthRequired, "glab не авторизован на %s: выполните glab auth login --hostname %s", host, host))
	}
	if h := v.Hosts[i]; h.State != "success" {
		return fail(classify("glab", runner.Result{Stderr: []byte(h.Error)}, nil, slices.Concat(glMarkers, []marker{{model.CodeAuthRequired, ""}})))
	}
	d.Availability = model.AvailabilityOK
	return d
}

type glMR struct {
	Draft           bool   `json:"draft"`
	SourceBranch    string `json:"source_branch"`
	SourceProjectID int64  `json:"source_project_id"`
	TargetProjectID int64  `json:"target_project_id"`
	WebURL          string `json:"web_url"`
}

// PullRequests — открытые MR, где ветка — источник; MR из форков
// (source_project_id ≠ target_project_id) не считаются.
func (g GitLab) PullRequests(ctx context.Context, q Query) model.PRSummary {
	if q.Branch == "" {
		return prFailed(noBranch())
	}
	out, e := g.run(ctx, q.Dir, "mr", "list", "-R", glRepo(q.Target), "--source-branch", q.Branch, "--per-page", "100", "-F", "json")
	if e != nil {
		return prFailed(e)
	}
	var mrs []glMR
	if err := json.Unmarshal(out, &mrs); err != nil {
		return prFailed(badOutput("glab", err))
	}
	mrs = slices.DeleteFunc(mrs, func(m glMR) bool {
		return m.SourceProjectID != m.TargetProjectID || m.SourceBranch != q.Branch
	})

	draft := 0
	for _, m := range mrs {
		if m.Draft {
			draft++
		}
	}
	link := ""
	switch {
	case len(mrs) == 1:
		link = mrs[0].WebURL
	case len(mrs) > 1:
		link = webURL(q.Target, "/-/merge_requests?state=opened&source_branch="+url.QueryEscape(q.Branch))
	}
	return withLast(prOK(len(mrs), draft, link), func() *model.PRRef { return g.lastClosed(ctx, q) })
}

type glClosedMR struct {
	IID             int    `json:"iid"`
	State           string `json:"state"`
	SourceBranch    string `json:"source_branch"`
	SourceProjectID int64  `json:"source_project_id"`
	TargetProjectID int64  `json:"target_project_id"`
	WebURL          string `json:"web_url"`
	MergedAt        string `json:"merged_at"`
	ClosedAt        string `json:"closed_at"`
}

// lastClosed — последний закрытый или влитый MR ветки. `--all` отдаёт и
// открытые, и закрытые (отдельного флага «закрытые или влитые» у glab нет).
func (g GitLab) lastClosed(ctx context.Context, q Query) *model.PRRef {
	out, e := g.run(ctx, q.Dir, "mr", "list", "-R", glRepo(q.Target), "--source-branch", q.Branch, "--all", "--per-page", "20", "-F", "json")
	if e != nil {
		return nil
	}
	var mrs []glClosedMR
	if json.Unmarshal(out, &mrs) != nil {
		return nil
	}
	var refs []model.PRRef
	for _, m := range mrs {
		if m.SourceProjectID != m.TargetProjectID || m.SourceBranch != q.Branch {
			continue
		}
		switch m.State {
		case "merged":
			refs = append(refs, model.PRRef{Number: m.IID, Merged: true, URL: m.WebURL, ClosedAt: parseTime(m.MergedAt)})
		case "closed":
			refs = append(refs, model.PRRef{Number: m.IID, URL: m.WebURL, ClosedAt: parseTime(m.ClosedAt)})
		}
	}
	return latestPR(refs)
}

type glPipeline struct {
	Status    string `json:"status"`
	SHA       string `json:"sha"`
	CreatedAt string `json:"created_at"`
}

// glActive — pipeline ещё выполняется. manual и scheduled ждут человека
// или времени — не активны (docs/providers.md).
var glActive = []string{"created", "waiting_for_resource", "preparing", "pending", "running", "canceling"}

// CIRuns — pipelines ветки (20 последних): активные — по всем, итог —
// по последнему pipeline последнего коммита (glLastResult).
func (g GitLab) CIRuns(ctx context.Context, q Query) model.CISummary {
	if q.Branch == "" {
		return ciFailed(noBranch())
	}
	out, e := g.run(ctx, q.Dir, "ci", "list", "-R", glRepo(q.Target), "--ref", q.Branch, "--per-page", "20", "-F", "json")
	if e != nil {
		return ciFailed(e)
	}
	var pipes []glPipeline
	if err := json.Unmarshal(out, &pipes); err != nil {
		return ciFailed(badOutput("glab", err))
	}
	active := 0
	for _, p := range pipes {
		if slices.Contains(glActive, p.Status) {
			active++
		}
	}
	link := ""
	if len(pipes) > 0 {
		link = webURL(q.Target, "/-/pipelines?ref="+url.QueryEscape(q.Branch))
	}
	return ciOK(active, glLastResult(pipes), link)
}

// glLastResult — итог последнего коммита: в GitLab pipeline описывает коммит
// целиком, поэтому берётся самый новый pipeline ветки. Незавершённый
// (и manual / scheduled) итога не даёт; canceled — провал, как в D-043.
func glLastResult(pipes []glPipeline) string {
	if len(pipes) == 0 {
		return ""
	}
	// created_at в UTC (`…Z`) сравнивается как строка.
	newest := slices.MaxFunc(pipes, func(a, b glPipeline) int { return cmp.Compare(a.CreatedAt, b.CreatedAt) })
	switch newest.Status {
	case "success", "skipped":
		return "success"
	case "failed", "canceled":
		return "failure"
	}
	return ""
}
