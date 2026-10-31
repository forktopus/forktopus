package providers

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// Фикстуры testdata/gitlab — ответы glab 1.126.0 с пилотного проекта на gitlab.com,
// сокращённые до нужных полей (без автора и участников), владелец — OWNER.
var glTarget = Target{Provider: model.ProviderGitLab, Host: "gitlab.com", BaseURL: "https://gitlab.com", Project: "OWNER/forktopus-pilot", Login: "gitlab.com"}

func glWith(answers map[string]reply) (GitLab, *scripted) {
	s := &scripted{answers: answers}
	return GitLab{Runner: s}, s
}

func glQuery(branch string) Query {
	return Query{Dir: "/repo", Target: glTarget, Branch: branch}
}

func TestGitLabPullRequests(t *testing.T) { // A19, A21
	ctx := context.Background()
	open := strings.TrimPrefix(string(fixture(t, "gitlab/mr-pr-open.json")), "[")
	fork := `[{"draft":false,"source_branch":"pilot/pr-open","source_project_id":1,"target_project_id":88339103,"web_url":"https://gitlab.com/OWNER/forktopus-pilot/-/merge_requests/9"},` + open
	two := `[{"draft":true,"source_branch":"pilot/pr-open","source_project_id":5,"target_project_id":5,"web_url":"https://gitlab.com/OWNER/forktopus-pilot/-/merge_requests/8"},` + open

	cases := []struct {
		name, branch string
		out          string
		open, draft  int
		url          string
	}{
		{"открытый MR", "pilot/pr-open", string(fixture(t, "gitlab/mr-pr-open.json")), 1, 0, "https://gitlab.com/OWNER/forktopus-pilot/-/merge_requests/1"},
		{"черновик", "pilot/pr-draft", string(fixture(t, "gitlab/mr-pr-draft.json")), 1, 1, "https://gitlab.com/OWNER/forktopus-pilot/-/merge_requests/2"},
		{"нет MR — достоверный ноль", "pilot/no-pr", string(fixture(t, "gitlab/mr-no-pr.json")), 0, 0, ""},
		{"MR из форка не считается", "pilot/pr-open", fork, 1, 0, "https://gitlab.com/OWNER/forktopus-pilot/-/merge_requests/1"},
		{"несколько — ссылка на список", "pilot/pr-open", two, 2, 1, "https://gitlab.com/OWNER/forktopus-pilot/-/merge_requests?state=opened&source_branch=pilot%2Fpr-open"},
	}
	for _, c := range cases {
		g, s := glWith(map[string]reply{"mr list": stdout([]byte(c.out))})
		got := g.PullRequests(ctx, glQuery(c.branch))
		if got.Availability != model.AvailabilityOK || got.Open == nil || *got.Open != c.open || *got.Draft != c.draft || got.URL != c.url || got.Error != nil {
			t.Errorf("%s: %+v", c.name, got)
		}
		args := s.calls[0].Args
		if !slices.Contains(args, "https://gitlab.com/OWNER/forktopus-pilot") || !slices.Contains(args, c.branch) || s.calls[0].Dir != "/repo" {
			t.Errorf("%s: команда %v", c.name, args)
		}
	}
}

func TestGitLabSubpathRepo(t *testing.T) {
	g, s := glWith(map[string]reply{"mr list": stdout([]byte("[]"))})
	sub := Target{Provider: model.ProviderGitLab, Host: "code.corp:2222", BaseURL: "https://code.corp/gitlab", Project: "team/sub/repo", Login: "code.corp"}
	g.PullRequests(context.Background(), Query{Target: sub, Branch: "main"})
	if !slices.Contains(s.calls[0].Args, "https://code.corp/gitlab/team/sub/repo") {
		t.Errorf("-R для self-hosted с подпутём: %v", s.calls[0].Args)
	}
}

func TestGitLabErrors(t *testing.T) { // A17, A24
	ctx := context.Background()
	cases := []struct {
		name  string
		reply reply
		code  model.ErrorCode
		avail model.Availability
	}{
		{"нет glab", reply{err: model.Errorf(model.CodeCLIMissing, "glab не найден в PATH")}, model.CodeCLIMissing, model.AvailabilityUnavailable},
		{"auth сломан", stderr(1, "ERROR: GET https://gitlab.com/api/v4/projects/OWNER%2Fforktopus-pilot/merge_requests: 401 {message: 401 Unauthorized}\n"), model.CodeAuthRequired, model.AvailabilityUnavailable},
		{"нет логина на хосте", stderr(1, "x gl.example has not been authenticated with glab; run `glab auth login --hostname gl.example` to authenticate.\n"), model.CodeAuthRequired, model.AvailabilityUnavailable},
		// Вывод glab как есть: рамка ERROR и перенос строк посреди URL и «dial tcp».
		{"нет сети", stderr(1, "          \n   ERROR  \n          \n  Get \"https://gitlab.com/api/v4/projects/OWNER%2Fforktopus-   \n  pilot/pipelines?ref=main\": dial\n  tcp: lookup gitlab.com on 127.0.0.53:53: network is unreachable.   \n\n"), model.CodeNetworkError, model.AvailabilityError},
		{"нет проекта", stderr(1, "ERROR: GET https://gitlab.com/api/v4/projects/OWNER%2Fnope: 404 {message: 404 Project Not Found}\n"), model.CodeRepoUnresolved, model.AvailabilityError},
		{"ошибка JSON-ом в stdout", reply{res: resultOf(1, `{"error":{"message":"GET https://gitlab.com/api/v4/projects/OWNER%2Fnope: 404 Not Found"}}`, "")}, model.CodeRepoUnresolved, model.AvailabilityError},
		{"лимит", stderr(1, "ERROR: GET https://gitlab.com/api/v4/projects/1/pipelines: 429 Too Many Requests\n"), model.CodeRateLimit, model.AvailabilityError},
		{"старый glab", stderr(1, "ERROR: unknown flag: --per-page\n"), model.CodeUnsupported, model.AvailabilityNotSupported},
		{"мусор вместо JSON", stdout([]byte("<html>")), model.CodeOther, model.AvailabilityError},
	}
	for _, c := range cases {
		g, _ := glWith(map[string]reply{"mr list": c.reply, "ci list": c.reply})
		pr := g.PullRequests(ctx, glQuery("main"))
		ci := g.CIRuns(ctx, glQuery("main"))
		if pr.Availability != c.avail || pr.Error == nil || pr.Error.Code != c.code || pr.Open != nil {
			t.Errorf("%s, MR: %v %+v", c.name, pr.Availability, pr.Error)
		}
		if ci.Availability != c.avail || ci.Error == nil || ci.Error.Code != c.code || ci.Active != nil {
			t.Errorf("%s, CI: %v %+v", c.name, ci.Availability, ci.Error)
		}
	}
	if ci := (GitLab{}).CIRuns(ctx, glQuery("")); ci.Error == nil || ci.Error.Code != model.CodeInvalidInput {
		t.Errorf("detached HEAD: %+v", ci)
	}
}

func TestGitLabCIRuns(t *testing.T) { // A20, A21
	ctx := context.Background()
	cases := []struct {
		name   string
		file   string
		active int
		last   string
	}{
		{"падение", "ci-fail.json", 0, "failure"},
		{"успех", "ci-no-pr.json", 0, "success"},
		// последний (ручной) pipeline коммита отменён — перекрывает успешный push.
		{"отмена", "ci-slow.json", 0, "failure"},
		// новый pipeline выполняется: итога нет, активный один.
		{"выполняется", "ci-running.json", 1, ""},
	}
	for _, c := range cases {
		g, s := glWith(map[string]reply{"ci list": stdout(fixture(t, "gitlab/"+c.file))})
		got := g.CIRuns(ctx, glQuery("pilot/x"))
		if got.Availability != model.AvailabilityOK || got.Active == nil || *got.Active != c.active || got.LastResult != c.last {
			t.Errorf("%s: %+v", c.name, got)
		}
		if got.URL != "https://gitlab.com/OWNER/forktopus-pilot/-/pipelines?ref=pilot%2Fx" {
			t.Errorf("%s: ссылка %q", c.name, got.URL)
		}
		if !slices.Contains(s.calls[0].Args, "--ref") {
			t.Errorf("%s: команда %v", c.name, s.calls[0].Args)
		}
	}

	g, _ := glWith(map[string]reply{"ci list": stdout([]byte("[]"))})
	if got := g.CIRuns(ctx, glQuery("pilot/x")); got.Availability != model.AvailabilityOK || *got.Active != 0 || got.LastResult != "" || got.URL != "" {
		t.Errorf("нет pipelines: %+v", got)
	}
}

func TestUnwrapGlab(t *testing.T) {
	in := "          \n   ERROR  \n          \n  Get \"https://gitlab.com/api/v4/projects/OWNER%2Fforktopus-   \n  pilot/pipelines\": dial\n  tcp: lookup gitlab.com\n\n"
	want := `Get "https://gitlab.com/api/v4/projects/OWNER%2Fforktopus-pilot/pipelines": dial tcp: lookup gitlab.com`
	if got := unwrapGlab(in); got != want {
		t.Errorf("unwrapGlab:\n got %q\nwant %q", got, want)
	}
}

func TestGLLastResult(t *testing.T) {
	p := func(status, at string) glPipeline { return glPipeline{Status: status, CreatedAt: at} }
	cases := []struct {
		name  string
		pipes []glPipeline
		want  string
	}{
		{"порядок ответа не важен", []glPipeline{p("failed", "2026-01-01T00:00:01Z"), p("success", "2026-01-01T00:00:02Z")}, "success"},
		{"skipped — не провал", []glPipeline{p("skipped", "1")}, "success"},
		{"manual ждёт человека", []glPipeline{p("manual", "2"), p("failed", "1")}, ""},
		{"canceling — ещё активен", []glPipeline{p("canceling", "1")}, ""},
	}
	for _, c := range cases {
		if got := glLastResult(c.pipes); got != c.want {
			t.Errorf("%s: %q, ожидался %q", c.name, got, c.want)
		}
	}
}

func TestGitLabDiagnose(t *testing.T) {
	ctx := context.Background()
	version := stdout([]byte("glab 1.126.0 (4dd0d32d1)\n"))
	cases := []struct {
		name    string
		answers map[string]reply
		avail   model.Availability
		code    model.ErrorCode
	}{
		{"всё хорошо", map[string]reply{"--version": version, "auth status": stdout(fixture(t, "gitlab/auth-ok.json"))}, model.AvailabilityOK, ""},
		{"нет сети", map[string]reply{"--version": version, "auth status": {res: runner.Result{ExitCode: 1, Stdout: fixture(t, "gitlab/auth-offline.json")}}}, model.AvailabilityError, model.CodeNetworkError},
		// Логин не прошёл проверку: glab выходит с 1, но печатает JSON со статусом.
		{"auth сломан (A17)", map[string]reply{"--version": version, "auth status": {res: resultOf(1, `{"hosts":[{"host":"gitlab.com","state":"error","error":"GET https://gitlab.com/api/v4/user: 401 {message: 401 Unauthorized}"}]}`, "")}}, model.AvailabilityUnavailable, model.CodeAuthRequired},
		{"нет логина на хосте", map[string]reply{"--version": version, "auth status": reply{res: resultOf(1,
			`{"error":{"message":"x gitlab.com has not been authenticated with glab; run `+"`glab auth login --hostname gitlab.com`"+` to authenticate"}}`, "ERROR\n")}}, model.AvailabilityUnavailable, model.CodeAuthRequired},
		{"нет glab", map[string]reply{"--version": {err: model.Errorf(model.CodeCLIMissing, "glab не найден в PATH")}}, model.AvailabilityUnavailable, model.CodeCLIMissing},
	}
	for _, c := range cases {
		g, s := glWith(c.answers)
		d := g.Diagnose(ctx, "/repo", glTarget)
		if d.Name != "glab" || d.Availability != c.avail || (c.code == "") != (d.Error == nil) || (d.Error != nil && d.Error.Code != c.code) {
			t.Errorf("%s: %+v %+v", c.name, d, d.Error)
		}
		if c.code != model.CodeCLIMissing && d.Version != "1.126.0" {
			t.Errorf("%s: версия %q", c.name, d.Version)
		}
		if len(s.calls) == 2 && !slices.Contains(s.calls[1].Args, "gitlab.com") {
			t.Errorf("%s: auth status без хоста: %v", c.name, s.calls[1].Args)
		}
	}
}
