package providers

import (
	"context"
	"slices"
	"strconv"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
)

// gt 0.2.0 ходит в тот же REST Gitea, что и `tea api` (D-074): фикстуры
// testdata/gitea подходят обоим. Ответ вне 2xx — код 1, тело в stdout, статус в
// тексте ошибки; нет аккаунта — код 4.
var gtTarget = Target{Provider: model.ProviderGitea, Host: "git.example.org", BaseURL: "https://git.example.org", Project: "OWNER/forktopus-pilot", Login: "OWNER", Tool: "gt"}

func gtQ(branch string) Query { return Query{Dir: "/repo", Target: gtTarget, Branch: branch} }

// gtFlags — инстанс и пользователь, которые адаптер передаёт gt явно.
var gtFlags = []string{"--host", "https://git.example.org", "--user", "OWNER", "--no-input"}

func gtOK(body string) reply { return reply{res: resultOf(0, body, "")} }

func gtHTTP(status int, method, path, body string) reply {
	msg := map[int]string{401: "invalid username, password or token", 403: "forbidden", 404: "not found", 429: "too many requests"}[status]
	return reply{res: resultOf(1, body, "Error: "+msg+" (HTTP "+strconv.Itoa(status)+" "+method+" /api/v1/"+path+")\n")}
}

func TestGtPullRequests(t *testing.T) {
	ctx := context.Background()
	open := "api repos/OWNER/forktopus-pilot/pulls?state=open&limit=50&page=1"
	closed := "api repos/OWNER/forktopus-pilot/pulls?state=closed"
	rest := string(fixture(t, "gitea/pulls-open-rest.json"))
	cases := []struct {
		name, branch string
		open, draft  int
		url          string
	}{
		{"открытый PR", "pilot/pr-open", 1, 0, "https://git.example.org/OWNER/forktopus-pilot/pulls/1"},
		{"черновик по полю draft", "pilot/pr-draft", 1, 1, "https://git.example.org/OWNER/forktopus-pilot/pulls/2"},
	}
	for _, c := range cases {
		g, s := gtWith(map[string]reply{open: gtOK(rest)})
		got := g.PullRequests(ctx, gtQ(c.branch))
		if got.Availability != model.AvailabilityOK || *got.Open != c.open || *got.Draft != c.draft || got.URL != c.url || len(s.calls) != 1 {
			t.Errorf("%s: %+v, вызовы %v", c.name, got, s.calls)
		}
		call := s.calls[0]
		if call.Name != "gt" || !slices.Equal(call.Args[2:], gtFlags) || call.Dir != "/repo" {
			t.Errorf("%s: команда %s %v", c.name, call.Name, call.Args)
		}
	}

	// PR из форка с той же веткой не считается (у tea pulls list их не отличить).
	fork := `[{"number":9,"title":"из форка","html_url":"u9","head":{"ref":"pilot/pr-open","sha":"x","repo":{"id":40}},"base":{"ref":"main","repo":{"id":25}}}]`
	g, _ := gtWith(map[string]reply{open: gtOK(fork), closed: gtOK(string(fixture(t, "gitea/pulls-closed.json")))})
	if got := g.PullRequests(ctx, gtQ("pilot/pr-open")); got.Availability != model.AvailabilityOK || *got.Open != 0 {
		t.Errorf("форк: %+v", got)
	}

	// Нет открытых — последний закрытый через тот же REST.
	g, _ = gtWith(map[string]reply{open: gtOK("[]"), closed: gtOK(string(fixture(t, "gitea/pulls-closed.json")))})
	wantLast(t, "gt: закрыт без слияния", g.PullRequests(ctx, gtQ("pilot/closed-pr")),
		3, false, "https://git.example.org/OWNER/forktopus-pilot/pulls/3", "2026-10-31T17:07:36+03:00")
}

func TestGtCIRuns(t *testing.T) {
	ctx := context.Background()
	g, s := gtWith(map[string]reply{"api repos/OWNER/forktopus-pilot/actions/runs?branch=pilot%2Fx&limit=20": gtOK(string(fixture(t, "gitea/runs-fail.json")))})
	got := g.CIRuns(ctx, gtQ("pilot/x"))
	if got.Availability != model.AvailabilityOK || *got.Active != 0 || got.LastResult != "failure" || got.URL != "https://git.example.org/OWNER/forktopus-pilot/actions/runs/405" || len(s.calls) != 1 {
		t.Errorf("CI: %+v, вызовы %v", got, s.calls)
	}
}

func TestGtErrors(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		api   reply
		code  model.ErrorCode
		avail model.Availability
	}{
		{"нет gt", reply{err: model.Errorf(model.CodeCLIMissing, "gt не найден в PATH")}, model.CodeCLIMissing, model.AvailabilityUnavailable},
		{"нет аккаунта — код 4", stderr(4, "Error: account \"OWNER\" is not logged in on https://git.example.org (accounts: other)\n"), model.CodeAuthRequired, model.AvailabilityUnavailable},
		{"код 4 без знакомого текста", stderr(4, "Error: что-то с учёткой\n"), model.CodeAuthRequired, model.AvailabilityUnavailable},
		{"токен отклонён", gtHTTP(401, "GET", "repos/OWNER/forktopus-pilot", `{"message":"invalid username, password or token"}`), model.CodeAuthRequired, model.AvailabilityUnavailable},
		{"нет прав", gtHTTP(403, "GET", "repos/OWNER/forktopus-pilot", `{"message":"forbidden"}`), model.CodeAccessDenied, model.AvailabilityNotSupported},
		// Так gt 0.2.0 сообщает о 403 на запуски Actions без прав на раздел (D-068).
		{"нет прав, текст gt", stderr(1, "Error: permission denied by the Gitea instance: user should have specific read permission or be a repo admin or a site admin\n"), model.CodeAccessDenied, model.AvailabilityNotSupported},
		{"лимит", gtHTTP(429, "GET", "repos/OWNER/forktopus-pilot", `{"message":"too many requests"}`), model.CodeRateLimit, model.AvailabilityError},
		{"нет репозитория", gtHTTP(404, "GET", "repos/OWNER/forktopus-pilot", `{"errors":null,"message":"not found","url":"https://git.example.org/api/swagger"}`), model.CodeRepoUnresolved, model.AvailabilityError},
		{"нет сети", stderr(1, "Error: cannot reach https://git.example.org (dial tcp: lookup git.example.org: no such host); check --host or your network\n"), model.CodeNetworkError, model.AvailabilityError},
		{"мусор вместо JSON", gtOK("<html>"), model.CodeOther, model.AvailabilityError},
	}
	for _, c := range cases {
		g, _ := gtWith(map[string]reply{"api": c.api})
		pr := g.PullRequests(ctx, gtQ("main"))
		ci := g.CIRuns(ctx, gtQ("main"))
		if pr.Availability != c.avail || pr.Error == nil || pr.Error.Code != c.code || pr.Open != nil {
			t.Errorf("%s, PR: %v %+v", c.name, pr.Availability, pr.Error)
		}
		if ci.Availability != c.avail || ci.Error == nil || ci.Error.Code != c.code || ci.Active != nil {
			t.Errorf("%s, CI: %v %+v", c.name, ci.Availability, ci.Error)
		}
	}

	// A18: маршрута запусков нет (`404 page not found` текстом) — CI «не поддерживается».
	g, _ := gtWith(map[string]reply{"api": gtHTTP(404, "GET", "repos/OWNER/forktopus-pilot/actions/runs", "404 page not found\n")})
	if ci := g.CIRuns(ctx, gtQ("main")); ci.Availability != model.AvailabilityNotSupported || ci.Error.Code != model.CodeUnsupported {
		t.Errorf("A18: %+v %+v", ci, ci.Error)
	}
}

func TestGtDiagnose(t *testing.T) {
	ctx := context.Background()
	version := gtOK("gt version 0.2.0 (2026-10-26)\nhttps://gitea.com/gitea/cli/releases/tag/v0.2.0\n")
	cases := []struct {
		name    string
		target  Target
		answers map[string]reply
		avail   model.Availability
		code    model.ErrorCode
	}{
		{"всё хорошо", gtTarget, map[string]reply{"--version": version, "api user --host https://git.example.org --user OWNER": gtOK(`{"login":"OWNER"}`)}, model.AvailabilityOK, ""},
		{"учётка отклонена", gtTarget, map[string]reply{"--version": version, "api": gtHTTP(401, "GET", "user", `{"message":"invalid username, password or token"}`)}, model.AvailabilityUnavailable, model.CodeAuthRequired},
		// Ни у gt, ни у tea нет входа — проверяется и советуется gt.
		{"нет входа на сервер", Target{Provider: model.ProviderGitea, Host: "git.example.org", Project: "OWNER/x"}, map[string]reply{"--version": version}, model.AvailabilityUnavailable, model.CodeAuthRequired},
		{"нет gt", gtTarget, map[string]reply{"--version": {err: model.Errorf(model.CodeCLIMissing, "gt не найден в PATH")}}, model.AvailabilityUnavailable, model.CodeCLIMissing},
	}
	for _, c := range cases {
		g, s := gtWith(c.answers)
		d := g.Diagnose(ctx, "/repo", c.target)
		if d.Name != "gt" || d.Availability != c.avail || (c.code == "") != (d.Error == nil) || (d.Error != nil && d.Error.Code != c.code) {
			t.Errorf("%s: %+v %+v", c.name, d, d.Error)
		}
		if c.code != model.CodeCLIMissing && d.Version != "0.2.0" {
			t.Errorf("%s: версия %q", c.name, d.Version)
		}
		if len(s.calls) == 0 || s.calls[0].Name != "gt" {
			t.Errorf("%s: вызовы %v", c.name, s.calls)
		}
	}
	if e := noLogin(Target{Host: "git.example.org:3000"}); e.Message != "у gt нет входа на git.example.org: выполните gt auth login --host https://git.example.org:3000" {
		t.Errorf("подсказка: %s", e.Message)
	}
}

func TestResolveGtFirst(t *testing.T) { // D-074
	logins := []Login{
		{Provider: model.ProviderGitea, Name: "home", URL: "https://git.home.example", SSHHost: "ssh.home.example", Default: true, Tool: "tea"},
		{Provider: model.ProviderGitea, Name: "OWNER", URL: "https://git.home.example", Default: true, Tool: "gt"},
	}
	res := Resolver{Logins: logins}
	t1, e := res.Resolve(remote("https://git.home.example/me/repo.git"))
	if e != nil || t1.Tool != "gt" || t1.Login != "OWNER" || CLI(t1) != "gt" {
		t.Errorf("есть вход gt — gt: %+v %v", t1, e)
	}
	// SSH-хост знает только tea — запасной tea.
	t2, e := res.Resolve(remote("git@ssh.home.example:me/repo.git"))
	if e != nil || t2.Tool != "tea" || t2.Login != "home" || CLI(t2) != "tea" {
		t.Errorf("только tea: %+v %v", t2, e)
	}
}
