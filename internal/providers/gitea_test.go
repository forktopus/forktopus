package providers

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
)

// Фикстуры testdata/gitea — ответы tea 0.16.0 и Gitea 1.26.2 с пилотного
// репозитория; хост заменён на git.example.org, владелец — на OWNER, у запусков
// убраны actor / trigger_actor / repository.
var gtTarget = Target{Provider: model.ProviderGitea, Host: "git.example.org", BaseURL: "https://git.example.org", Project: "OWNER/forktopus-pilot", Login: "home"}

func gtWith(answers map[string]reply) (Gitea, *scripted) {
	s := &scripted{answers: answers}
	return Gitea{Runner: s}, s
}

func gtQuery(branch string) Query {
	return Query{Dir: "/repo", Target: gtTarget, Branch: branch}
}

// apiReply — ответ `tea api -i`: статус в первой строке stderr, тело в stdout, код 0.
func apiReply(status, body string) reply {
	return reply{res: resultOf(0, body, status+"\nContent-Type: application/json\n")}
}

// pullsPage — страница `tea pulls list` из n чужих PR.
func pullsPage(n int) string {
	items := make([]string, n)
	for i := range items {
		items[i] = fmt.Sprintf(`{"index":"%d","head":"other-%d","url":"u","title":"t"}`, i+10, i)
	}
	return "[" + strings.Join(items, ",") + "]"
}

func TestGiteaPullRequests(t *testing.T) { // A19, A21
	ctx := context.Background()
	cases := []struct {
		name, branch string
		open, draft  int
		url          string
	}{
		{"открытый PR", "pilot/pr-open", 1, 0, "https://git.example.org/OWNER/forktopus-pilot/pulls/1"},
		{"черновик по WIP:", "pilot/pr-draft", 1, 1, "https://git.example.org/OWNER/forktopus-pilot/pulls/2"},
		{"нет PR — достоверный ноль", "pilot/no-pr", 0, 0, ""},
	}
	for _, c := range cases {
		g, s := gtWith(map[string]reply{"pulls list": stdout(fixture(t, "gitea/pulls.json"))})
		got := g.PullRequests(ctx, gtQuery(c.branch))
		if got.Availability != model.AvailabilityOK || got.Open == nil || *got.Open != c.open || *got.Draft != c.draft || got.URL != c.url || got.Error != nil {
			t.Errorf("%s: %+v", c.name, got)
		}
		args := s.calls[0].Args
		if !slices.Contains(args, "home") || !slices.Contains(args, "OWNER/forktopus-pilot") || s.calls[0].Dir != "/repo" || len(s.calls) != 1+min(1, 1-c.open) {
			t.Errorf("%s: команды %v", c.name, s.calls)
		}
	}
}

func TestGiteaPullsPaging(t *testing.T) { // D-040
	ctx := context.Background()
	page2 := `[{"index":"3","head":"feat","url":"https://git.example.org/OWNER/forktopus-pilot/pulls/3","title":"[wip] ещё"},
{"index":"4","head":"feat","url":"https://git.example.org/OWNER/forktopus-pilot/pulls/4","title":"готово"}]`
	g, s := gtWith(map[string]reply{
		"pulls list -l home -r OWNER/forktopus-pilot --state open --limit 50 --page 1 ": stdout([]byte(pullsPage(50))),
		"pulls list -l home -r OWNER/forktopus-pilot --state open --limit 50 --page 2 ": stdout([]byte(page2)),
	})
	got := g.PullRequests(ctx, gtQuery("feat"))
	if got.Availability != model.AvailabilityOK || *got.Open != 2 || *got.Draft != 1 || len(s.calls) != 2 ||
		got.URL != "https://git.example.org/OWNER/forktopus-pilot/pulls?state=open" {
		t.Errorf("вторая страница: %+v, вызовов %d", got, len(s.calls))
	}

	// Все страницы полные — счёт был бы неполным: ошибка, а не заниженное число.
	g, s = gtWith(map[string]reply{"pulls list": stdout([]byte(pullsPage(50)))})
	got = g.PullRequests(ctx, gtQuery("feat"))
	if got.Open != nil || got.Error == nil || got.Availability != model.AvailabilityError || len(s.calls) != teaMaxPages {
		t.Errorf("слишком много PR: %+v, вызовов %d", got, len(s.calls))
	}
}

func TestTeaDraft(t *testing.T) {
	for title, want := range map[string]bool{"WIP: x": true, "[WIP] x": true, "wip: x": true, " WIP:x": true, "x WIP:": false, "WIPx": false} {
		if teaDraft(title) != want {
			t.Errorf("%q: %v", title, !want)
		}
	}
}

func TestGiteaCIRuns(t *testing.T) { // A20, A21
	ctx := context.Background()
	cases := []struct {
		name   string
		file   string
		active int
		last   string
		url    string
	}{
		{"fail упал, ok прошёл", "runs-fail.json", 0, "failure", "https://git.example.org/OWNER/forktopus-pilot/actions/runs/405"},
		{"успех", "runs-no-pr.json", 0, "success", "https://git.example.org/OWNER/forktopus-pilot/actions/runs/403"},
		// два slow выполняются (conclusion ещё нет), ok на том же коммите прошёл.
		{"выполняются", "runs-active.json", 2, "success", "https://git.example.org/OWNER/forktopus-pilot/actions/runs/411"},
		{"нет запусков", "runs-nothing.json", 0, "", ""},
	}
	for _, c := range cases {
		g, s := gtWith(map[string]reply{"api": apiReply("HTTP/2.0 200 OK", string(fixture(t, "gitea/"+c.file)))})
		got := g.CIRuns(ctx, gtQuery("pilot/x"))
		if got.Availability != model.AvailabilityOK || got.Active == nil || *got.Active != c.active || got.LastResult != c.last || got.URL != c.url {
			t.Errorf("%s: %+v", c.name, got)
		}
		want := []string{"api", "-l", "home", "-i", "/repos/OWNER/forktopus-pilot/actions/runs?branch=pilot%2Fx&limit=20"}
		if !slices.Equal(s.calls[0].Args, want) || s.calls[0].Dir != "/repo" {
			t.Errorf("%s: команда %v", c.name, s.calls[0].Args)
		}
	}
}

func TestGiteaErrors(t *testing.T) { // A18, A24
	ctx := context.Background()
	cases := []struct {
		name  string
		pulls reply
		api   reply
		code  model.ErrorCode
		avail model.Availability
	}{
		{"нет tea", missingTea, missingTea, model.CodeCLIMissing, model.AvailabilityUnavailable},
		{"нет сети", stderr(1, "Error: Get \"https://git.example.org/api/v1/repos/OWNER/forktopus-pilot/pulls?limit=50\": dial tcp: lookup git.example.org: network is unreachable\n"),
			stderr(1, "Error: request failed: Get \"https://git.example.org/api/v1/repos\": dial tcp: lookup git.example.org: network is unreachable\n"), model.CodeNetworkError, model.AvailabilityError},
		{"логин удалён", stderr(1, "Error: login name 'home' does not exist\n"), stderr(1, "Error: login name 'home' does not exist\n"), model.CodeAuthRequired, model.AvailabilityUnavailable},
		{"неверный токен", stderr(1, "Error: invalid username, password or token\n"),
			apiReply("HTTP/2.0 401 Unauthorized", `{"message":"invalid username, password or token","url":"https://git.example.org/api/swagger"}`), model.CodeAuthRequired, model.AvailabilityUnavailable},
		{"нет репозитория", stderr(1, "Error: not found\n"),
			apiReply("HTTP/2.0 404 Not Found", `{"message":"not found","url":"https://git.example.org/api/swagger","errors":null}`), model.CodeRepoUnresolved, model.AvailabilityError},
		{"мусор вместо JSON", stdout([]byte("<html>")), apiReply("HTTP/2.0 200 OK", "<html>"), model.CodeOther, model.AvailabilityError},
	}
	for _, c := range cases {
		g, _ := gtWith(map[string]reply{"pulls list": c.pulls, "api": c.api})
		pr := g.PullRequests(ctx, gtQuery("main"))
		ci := g.CIRuns(ctx, gtQuery("main"))
		if pr.Availability != c.avail || pr.Error == nil || pr.Error.Code != c.code || pr.Open != nil {
			t.Errorf("%s, PR: %v %+v", c.name, pr.Availability, pr.Error)
		}
		if ci.Availability != c.avail || ci.Error == nil || ci.Error.Code != c.code || ci.Active != nil {
			t.Errorf("%s, CI: %v %+v", c.name, ci.Availability, ci.Error)
		}
	}

	// A18: сервер не знает маршрута — CI «не поддерживается», PR работают.
	g, _ := gtWith(map[string]reply{
		"pulls list": stdout(fixture(t, "gitea/pulls.json")),
		"api":        apiReply("HTTP/2.0 404 Not Found", "404 page not found\n"),
	})
	ci := g.CIRuns(ctx, gtQuery("pilot/pr-open"))
	pr := g.PullRequests(ctx, gtQuery("pilot/pr-open"))
	if ci.Availability != model.AvailabilityNotSupported || ci.Error.Code != model.CodeUnsupported || ci.Active != nil || pr.Availability != model.AvailabilityOK || *pr.Open != 1 {
		t.Errorf("A18: CI %+v %+v, PR %+v", ci, ci.Error, pr)
	}

	for status, code := range map[string]model.ErrorCode{
		"HTTP/2.0 403 Forbidden":             model.CodeAuthRequired,
		"HTTP/2.0 429 Too Many Requests":     model.CodeRateLimit,
		"HTTP/2.0 500 Internal Server Error": model.CodeOther,
		"":                                   model.CodeOther,
	} {
		g, _ := gtWith(map[string]reply{"api": apiReply(status, "")})
		if ci := g.CIRuns(ctx, gtQuery("main")); ci.Error == nil || ci.Error.Code != code {
			t.Errorf("%q: %+v", status, ci.Error)
		}
	}

	// Нет логина tea на этом сервере — до вызова CLI.
	noLoginQ := gtQuery("main")
	noLoginQ.Target.Login = ""
	g, s := gtWith(nil)
	if pr := g.PullRequests(ctx, noLoginQ); pr.Error == nil || pr.Error.Code != model.CodeAuthRequired || len(s.calls) != 0 {
		t.Errorf("нет логина: %+v", pr.Error)
	}
	if ci := g.CIRuns(ctx, gtQuery("")); ci.Error == nil || ci.Error.Code != model.CodeInvalidInput {
		t.Errorf("detached HEAD: %+v", ci)
	}
}

var missingTea = reply{err: model.Errorf(model.CodeCLIMissing, "tea не найден в PATH")}

func TestGiteaDiagnose(t *testing.T) {
	ctx := context.Background()
	version := stdout([]byte("Version: \x1b[1m0.16.0\x1b[0m\tgolang: 1.26.6\tgo-sdk: v1.2.0\n"))
	cases := []struct {
		name    string
		target  Target
		answers map[string]reply
		avail   model.Availability
		code    model.ErrorCode
	}{
		{"всё хорошо", gtTarget, map[string]reply{"--version": version, "api -l home -i /user": apiReply("HTTP/2.0 200 OK", `{"login":"OWNER"}`)}, model.AvailabilityOK, ""},
		{"токен отозван", gtTarget, map[string]reply{"--version": version, "api": apiReply("HTTP/2.0 401 Unauthorized", `{"message":"invalid username, password or token"}`)}, model.AvailabilityUnavailable, model.CodeAuthRequired},
		{"нет сети", gtTarget, map[string]reply{"--version": version, "api": stderr(1, "Error: request failed: Get \"https://git.example.org/api/v1/user\": dial tcp: lookup git.example.org: network is unreachable\n")}, model.AvailabilityError, model.CodeNetworkError},
		{"нет логина на сервере", Target{Provider: model.ProviderGitea, Host: "git.example.org", Project: "OWNER/x"}, map[string]reply{"--version": version}, model.AvailabilityUnavailable, model.CodeAuthRequired},
		{"нет tea", gtTarget, map[string]reply{"--version": missingTea}, model.AvailabilityUnavailable, model.CodeCLIMissing},
	}
	for _, c := range cases {
		g, s := gtWith(c.answers)
		d := g.Diagnose(ctx, "/repo", c.target)
		if d.Name != "tea" || d.Availability != c.avail || (c.code == "") != (d.Error == nil) || (d.Error != nil && d.Error.Code != c.code) {
			t.Errorf("%s: %+v %+v", c.name, d, d.Error)
		}
		if c.code != model.CodeCLIMissing && d.Version != "0.16.0" {
			t.Errorf("%s: версия %q", c.name, d.Version)
		}
		for _, call := range s.calls {
			if call.Dir != "/repo" {
				t.Errorf("%s: tea без рабочей копии: %+v", c.name, call)
			}
		}
	}
}
