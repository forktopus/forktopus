package providers

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// Фикстуры testdata/github — ответы gh 2.103.3 с пилотного репозитория
// (docs/providers.md), владелец заменён на OWNER.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// scripted отвечает на команды по префиксу аргументов и запоминает вызовы.
type scripted struct {
	answers map[string]reply // ключ — первые аргументы через пробел
	calls   []runner.Command
}

type reply struct {
	res runner.Result
	err error
}

func (s *scripted) Run(_ context.Context, c runner.Command) (runner.Result, error) {
	s.calls = append(s.calls, c)
	line := strings.Join(c.Args, " ")
	for prefix, r := range s.answers {
		if strings.HasPrefix(line, prefix) {
			return r.res, r.err
		}
	}
	return runner.Result{ExitCode: 1, Stderr: []byte("unexpected call: " + line)}, nil
}

func stdout(b []byte) reply { return reply{res: runner.Result{Stdout: b}} }

func resultOf(code int, out, errOut string) runner.Result {
	return runner.Result{ExitCode: code, Stdout: []byte(out), Stderr: []byte(errOut)}
}

func stderr(code int, msg string) reply {
	return reply{res: runner.Result{ExitCode: code, Stderr: []byte(msg)}}
}

var (
	ghTarget = Target{Provider: model.ProviderGitHub, Host: "github.com", BaseURL: "https://github.com", Project: "OWNER/forktopus-pilot"}
	missing  = reply{err: model.Errorf(model.CodeCLIMissing, "gh не найден в PATH")}
)

func ghWith(answers map[string]reply) (GitHub, *scripted) {
	s := &scripted{answers: answers}
	return GitHub{Runner: s}, s
}

func query(branch string) Query {
	return Query{Dir: "/repo", Target: ghTarget, Branch: branch}
}

func TestGitHubPullRequests(t *testing.T) { // A19, A21
	ctx := context.Background()
	fork := `[{"isDraft":false,"url":"https://github.com/OWNER/forktopus-pilot/pull/7","isCrossRepository":true,"headRefName":"pilot/pr-open"},
` + strings.TrimPrefix(string(fixture(t, "github/pr-open.json")), "[")
	two := `[{"isDraft":true,"url":"https://github.com/OWNER/forktopus-pilot/pull/8","isCrossRepository":false,"headRefName":"pilot/pr-open"},
` + strings.TrimPrefix(string(fixture(t, "github/pr-open.json")), "[")

	cases := []struct {
		name, branch string
		out          string
		open, draft  int
		url          string
	}{
		{"открытый PR", "pilot/pr-open", string(fixture(t, "github/pr-open.json")), 1, 0, "https://github.com/OWNER/forktopus-pilot/pull/1"},
		{"черновик", "pilot/pr-draft", string(fixture(t, "github/pr-draft.json")), 1, 1, "https://github.com/OWNER/forktopus-pilot/pull/2"},
		{"нет PR — достоверный ноль", "pilot/no-pr", string(fixture(t, "github/pr-none.json")), 0, 0, ""},
		{"PR из форка не считается", "pilot/pr-open", fork, 1, 0, "https://github.com/OWNER/forktopus-pilot/pull/1"},
		{"несколько — ссылка на список", "pilot/pr-open", two, 2, 1, "https://github.com/OWNER/forktopus-pilot/pulls?q=is%3Apr+is%3Aopen+head%3Apilot%2Fpr-open"},
	}
	for _, c := range cases {
		g, s := ghWith(map[string]reply{"pr list": stdout([]byte(c.out))})
		got := g.PullRequests(ctx, query(c.branch))
		if got.Availability != model.AvailabilityOK || got.Open == nil || *got.Open != c.open || *got.Draft != c.draft || got.URL != c.url || got.Error != nil {
			t.Errorf("%s: %+v", c.name, got)
		}
		if got.LastSuccessAt == nil || got.LastAttemptAt == nil {
			t.Errorf("%s: нет времени обновления", c.name)
		}
		args := s.calls[0].Args
		if !slices.Contains(args, "github.com/OWNER/forktopus-pilot") || !slices.Contains(args, c.branch) || s.calls[0].Dir != "/repo" {
			t.Errorf("%s: команда %v", c.name, args)
		}
	}
}

func TestGitHubErrors(t *testing.T) { // A16, A24
	ctx := context.Background()
	cases := []struct {
		name  string
		reply reply
		code  model.ErrorCode
		avail model.Availability
	}{
		{"нет gh", missing, model.CodeCLIMissing, model.AvailabilityUnavailable},
		{"нет логина", stderr(4, "To get started with GitHub CLI, please run:  gh auth login\n"), model.CodeAuthRequired, model.AvailabilityUnavailable},
		{"неверный токен", stderr(1, "HTTP 401: Bad credentials (https://api.github.com/graphql)\nTry authenticating with:  gh auth login\n"), model.CodeAuthRequired, model.AvailabilityUnavailable},
		{"нет сети", stderr(1, "error connecting to api.github.com\ncheck your internet connection or https://githubstatus.com\n"), model.CodeNetworkError, model.AvailabilityError},
		{"нет репозитория", stderr(1, "GraphQL: Could not resolve to a Repository with the name 'OWNER/nope'. (repository)\n"), model.CodeRepoUnresolved, model.AvailabilityError},
		{"нет репозитория (REST)", stderr(1, "failed to get runs: HTTP 404: Not Found (https://api.github.com/repos/OWNER/nope/actions/runs)\n"), model.CodeRepoUnresolved, model.AvailabilityError},
		{"нет прав", stderr(1, "HTTP 403: Resource not accessible by personal access token (https://api.github.com/repos/OWNER/repo/actions/runs)\n"), model.CodeAccessDenied, model.AvailabilityNotSupported},
		{"лимит", stderr(1, "HTTP 403: API rate limit exceeded for user ID 1. (https://api.github.com/graphql)\n"), model.CodeRateLimit, model.AvailabilityError},
		{"старый gh", stderr(1, "Unknown JSON field: \"isDraft\"\n"), model.CodeUnsupported, model.AvailabilityNotSupported},
		{"таймаут", reply{err: model.Errorf(model.CodeTimeout, "gh не ответил за 20s")}, model.CodeTimeout, model.AvailabilityError},
		{"прочее", stderr(2, "\nsomething odd\n"), model.CodeOther, model.AvailabilityError},
		{"мусор вместо JSON", stdout([]byte("<html>")), model.CodeOther, model.AvailabilityError},
	}
	for _, c := range cases {
		g, _ := ghWith(map[string]reply{"pr list": c.reply, "run list": c.reply})
		pr := g.PullRequests(ctx, query("main"))
		ci := g.CIRuns(ctx, query("main"))
		for what, sum := range map[string]struct {
			avail model.Availability
			err   *model.Error
			zero  bool
		}{
			"PR": {pr.Availability, pr.Error, pr.Open == nil && pr.Draft == nil},
			"CI": {ci.Availability, ci.Error, ci.Active == nil && ci.LastResult == ""},
		} {
			if sum.avail != c.avail || sum.err == nil || sum.err.Code != c.code || !sum.zero {
				t.Errorf("%s, %s: %v %+v", c.name, what, sum.avail, sum.err)
			}
		}
	}
	if pr := (GitHub{}).PullRequests(ctx, query("")); pr.Error == nil || pr.Error.Code != model.CodeInvalidInput || pr.Open != nil {
		t.Errorf("detached HEAD: %+v", pr)
	}
}

func TestGitHubCIRuns(t *testing.T) { // A20, A21
	ctx := context.Background()
	cases := []struct {
		name   string
		file   string
		active int
		last   string
	}{
		// fail и ok на одном коммите упали.
		{"падение", "runs-fail.json", 0, "failure"},
		// ok упал на push, ручной запуск ok прошёл: это разные проверки коммита,
		// и GitHub показывает коммит упавшим.
		{"push упал, ручной запуск прошёл", "runs-no-pr.json", 0, "failure"},
		// последний slow отменён (cancelled), ok упал.
		{"отмена", "runs-slow.json", 0, "failure"},
		{"в очереди", "runs-queued.json", 1, "failure"},
		{"выполняется", "runs-active.json", 1, "failure"},
	}
	for _, c := range cases {
		g, s := ghWith(map[string]reply{"run list": stdout(fixture(t, "github/"+c.file))})
		got := g.CIRuns(ctx, query("pilot/x"))
		if got.Availability != model.AvailabilityOK || got.Active == nil || *got.Active != c.active || got.LastResult != c.last {
			t.Errorf("%s: %+v", c.name, got)
		}
		if got.URL != "https://github.com/OWNER/forktopus-pilot/actions?query=branch%3Apilot%2Fx" {
			t.Errorf("%s: ссылка %q", c.name, got.URL)
		}
		if !slices.Contains(s.calls[0].Args, "--branch") {
			t.Errorf("%s: команда %v", c.name, s.calls[0].Args)
		}
	}

	g, _ := ghWith(map[string]reply{"run list": stdout([]byte("[]"))})
	if got := g.CIRuns(ctx, query("pilot/x")); got.Availability != model.AvailabilityOK || *got.Active != 0 || got.LastResult != "" || got.URL != "" {
		t.Errorf("нет запусков: %+v", got)
	}
}

func TestGHLastResult(t *testing.T) {
	run := func(wf, status, concl, sha, at string) ghRun {
		return ghRun{WorkflowName: wf, Event: "push", Status: status, Conclusion: concl, HeadSha: sha, CreatedAt: at}
	}
	cases := []struct {
		name string
		runs []ghRun
		want string
	}{
		{"повторный запуск перекрывает упавший", []ghRun{run("ci", "completed", "success", "a", "2"), run("ci", "completed", "failure", "a", "1")}, "success"},
		{"старый коммит не влияет", []ghRun{run("ci", "completed", "success", "new", "2"), run("ci", "completed", "failure", "old", "1")}, "success"},
		{"перезапуск идёт — итога нет", []ghRun{run("ci", "in_progress", "", "a", "2"), run("ci", "completed", "failure", "a", "1")}, ""},
		{"skipped и neutral — не провал", []ghRun{run("a", "completed", "skipped", "a", "2"), run("b", "completed", "neutral", "a", "1")}, "success"},
		{"timed_out — провал", []ghRun{run("a", "completed", "success", "a", "2"), run("b", "completed", "timed_out", "a", "1")}, "failure"},
		{"порядок ответа не важен", []ghRun{run("ci", "completed", "failure", "old", "1"), run("ci", "completed", "success", "new", "2")}, "success"},
	}
	for _, c := range cases {
		if got := ghLastResult(c.runs); got != c.want {
			t.Errorf("%s: %q, ожидался %q", c.name, got, c.want)
		}
	}
}

func TestGitHubDiagnose(t *testing.T) {
	ctx := context.Background()
	version := stdout([]byte("gh version 2.103.3 (2026-10-26)\nhttps://github.com/cli/cli/releases/tag/v2.103.3\n"))
	cases := []struct {
		name    string
		answers map[string]reply
		avail   model.Availability
		code    model.ErrorCode
	}{
		{"всё хорошо", map[string]reply{"--version": version, "auth status": stdout(fixture(t, "github/auth-ok.json"))}, model.AvailabilityOK, ""},
		{"нет сети", map[string]reply{"--version": version, "auth status": stdout(fixture(t, "github/auth-offline.json"))}, model.AvailabilityError, model.CodeNetworkError},
		{"токен отозван", map[string]reply{"--version": version, "auth status": stdout([]byte(`{"hosts":{"github.com":[{"state":"error","error":"HTTP 401: Bad credentials","active":true}]}}`))}, model.AvailabilityUnavailable, model.CodeAuthRequired},
		{"нет логина на хосте", map[string]reply{"--version": version, "auth status": stdout([]byte(`{"hosts":{}}`))}, model.AvailabilityUnavailable, model.CodeAuthRequired},
		{"нет gh", map[string]reply{"--version": missing}, model.AvailabilityUnavailable, model.CodeCLIMissing},
	}
	for _, c := range cases {
		g, s := ghWith(c.answers)
		d := g.Diagnose(ctx, "/repo", ghTarget)
		if d.Name != "gh" || d.Availability != c.avail || (c.code == "") != (d.Error == nil) || (d.Error != nil && d.Error.Code != c.code) {
			t.Errorf("%s: %+v %+v", c.name, d, d.Error)
		}
		if c.code != model.CodeCLIMissing && d.Version != "2.103.3" {
			t.Errorf("%s: версия %q", c.name, d.Version)
		}
		if len(s.calls) == 2 && !slices.Contains(s.calls[1].Args, "github.com") {
			t.Errorf("%s: auth status без хоста: %v", c.name, s.calls[1].Args)
		}
	}
}
