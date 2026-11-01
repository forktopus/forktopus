package providers

import (
	"context"
	"strings"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
)

// Detached HEAD: PR/CI по коммиту (D-059). Фикстуры — ответы с пилотов,
// чужие записи собраны по тем же форматам.

func shaQuery(t Target, sha string) Query { return Query{Dir: "/repo", Target: t, SHA: sha} }

func TestGitHubByCommit(t *testing.T) {
	ctx := context.Background()
	const sha = "a2062d4d1a2ef209f52f9f53d07995832b37b289"
	fork := `[{"isDraft":false,"url":"https://github.com/OWNER/forktopus-pilot/pull/7","isCrossRepository":true,"headRefName":"other"},
` + strings.TrimPrefix(string(fixture(t, "github/pr-open.json")), "[")
	g, s := ghWith(map[string]reply{"pr list -R github.com/OWNER/forktopus-pilot --search " + sha + " --state open": stdout([]byte(fork))})
	got := g.PullRequests(ctx, shaQuery(ghTarget, sha))
	if got.Availability != model.AvailabilityOK || *got.Open != 1 || got.URL != "https://github.com/OWNER/forktopus-pilot/pull/1" || len(s.calls) != 1 {
		t.Errorf("открытый PR с коммитом, форк не считается: %+v, вызовы %v", got, s.calls)
	}

	// Открытых нет — последний закрытый тоже по коммиту, ветка любая.
	g, s = ghWith(map[string]reply{
		"pr list -R github.com/OWNER/forktopus-pilot --search " + sha + " --state open":   stdout([]byte("[]")),
		"pr list -R github.com/OWNER/forktopus-pilot --search " + sha + " --state closed": stdout(fixture(t, "github/pr-closed.json")),
	})
	got = g.PullRequests(ctx, shaQuery(ghTarget, sha))
	if got.Last == nil || got.Last.Number != 3 || len(s.calls) != 2 {
		t.Errorf("закрытый по коммиту: %+v", got)
	}

	g, s = ghWith(map[string]reply{"run list -R github.com/OWNER/forktopus-pilot --commit " + sha: stdout(fixture(t, "github/runs-fail.json"))})
	ci := g.CIRuns(ctx, shaQuery(ghTarget, sha))
	if ci.Availability != model.AvailabilityOK || ci.URL != "https://github.com/OWNER/forktopus-pilot/commit/"+sha || len(s.calls) != 1 {
		t.Errorf("CI по коммиту: %+v, вызовы %v", ci, s.calls)
	}
	g, _ = ghWith(map[string]reply{"run list": stdout([]byte("[]"))})
	if ci := g.CIRuns(ctx, shaQuery(ghTarget, sha)); ci.URL != "" || *ci.Active != 0 {
		t.Errorf("нет запусков — без ссылки: %+v", ci)
	}
}

func TestGitLabByCommit(t *testing.T) {
	ctx := context.Background()
	const sha = "b1a0f387245a2e2d4136aff69c3a18503e3a3909"
	api := "api --hostname gitlab.com projects/OWNER%2Fforktopus-pilot/repository/commits/" + sha + "/merge_requests"

	// Один ответ — и открытые, и последний закрытый.
	g, s := glWith(map[string]reply{api: stdout(fixture(t, "gitlab/commit-mrs-closed.json"))})
	wantLast(t, "только закрытый", g.PullRequests(ctx, shaQuery(glTarget, sha)),
		3, false, "https://gitlab.com/OWNER/forktopus-pilot/-/merge_requests/3", "2026-10-31T14:02:20.028Z")
	if len(s.calls) != 1 {
		t.Errorf("вызовы %v", s.calls)
	}

	mixed := `[{"iid":5,"state":"opened","draft":true,"source_branch":"a","source_project_id":5,"target_project_id":5,"web_url":"u5"},
{"iid":6,"state":"opened","draft":false,"source_branch":"b","source_project_id":5,"target_project_id":5,"web_url":"u6"},
{"iid":7,"state":"opened","draft":false,"source_branch":"c","source_project_id":1,"target_project_id":5,"web_url":"u7"}]`
	g, _ = glWith(map[string]reply{api: stdout([]byte(mixed))})
	got := g.PullRequests(ctx, shaQuery(glTarget, sha))
	if *got.Open != 2 || *got.Draft != 1 || got.Last != nil || got.URL != "https://gitlab.com/OWNER/forktopus-pilot/-/commit/"+sha {
		t.Errorf("два открытых из разных веток, форк не считается: %+v", got)
	}

	// Коммит не отправлен на сервер — достоверный ноль.
	g, _ = glWith(map[string]reply{api: {res: resultOf(1, `{"message":"404 Commit Not Found"}`, "glab: 404 Commit Not Found (HTTP 404)\n")}})
	if got := g.PullRequests(ctx, shaQuery(glTarget, sha)); got.Availability != model.AvailabilityOK || *got.Open != 0 {
		t.Errorf("коммита нет на сервере: %+v", got)
	}
	g, _ = glWith(map[string]reply{api: {res: resultOf(1, `{"message":"404 Project Not Found"}`, "glab: 404 Project Not Found (HTTP 404)\n")}})
	if got := g.PullRequests(ctx, shaQuery(glTarget, sha)); got.Availability == model.AvailabilityOK {
		t.Errorf("проекта нет — ошибка: %+v", got)
	}

	g, s = glWith(map[string]reply{"ci list -R https://gitlab.com/OWNER/forktopus-pilot --sha " + sha: stdout(fixture(t, "gitlab/ci-fail.json"))})
	ci := g.CIRuns(ctx, shaQuery(glTarget, sha))
	if ci.Availability != model.AvailabilityOK || ci.URL != "https://gitlab.com/OWNER/forktopus-pilot/-/commit/"+sha+"/pipelines" || len(s.calls) != 1 {
		t.Errorf("CI по коммиту: %+v, вызовы %v", ci, s.calls)
	}
}

func TestGiteaByCommit(t *testing.T) {
	ctx := context.Background()
	open := "api -l home -i /repos/OWNER/forktopus-pilot/pulls?state=open"
	closed := "api -l home -i /repos/OWNER/forktopus-pilot/pulls?state=closed"
	ok := func(body string) reply { return apiReply("HTTP/2.0 200 OK", body) }
	rest := string(fixture(t, "gitea/pulls-open-rest.json"))

	cases := []struct {
		name, sha   string
		open, draft int
		url         string
	}{
		{"head открытого PR", "1f1c85b4682c410ac412f0425daeaff0c863f7e3", 1, 0, "https://git.example.org/OWNER/forktopus-pilot/pulls/1"},
		{"черновик по полю draft", "d784e5f5eb0a7120b394f8310c19555169a5a2ea", 1, 1, "https://git.example.org/OWNER/forktopus-pilot/pulls/2"},
	}
	for _, c := range cases {
		g, s := gtWith(map[string]reply{open: ok(rest)})
		got := g.PullRequests(ctx, shaQuery(gtTarget, c.sha))
		if got.Availability != model.AvailabilityOK || *got.Open != c.open || *got.Draft != c.draft || got.URL != c.url || len(s.calls) != 1 {
			t.Errorf("%s: %+v, вызовы %v", c.name, got, s.calls)
		}
	}

	// Коммит — head закрытого PR.
	g, _ := gtWith(map[string]reply{open: ok(rest), closed: ok(string(fixture(t, "gitea/pulls-closed.json")))})
	wantLast(t, "закрытый по коммиту", g.PullRequests(ctx, shaQuery(gtTarget, "0d85cbf3a0ec477f2b2b4382940ed8ac36aecf00")),
		3, false, "https://git.example.org/OWNER/forktopus-pilot/pulls/3", "2026-10-31T17:07:36+03:00")

	g, s := gtWith(map[string]reply{"api -l home -i /repos/OWNER/forktopus-pilot/actions/runs?head_sha=abc&": ok(string(fixture(t, "gitea/runs-fail.json")))})
	if ci := g.CIRuns(ctx, shaQuery(gtTarget, "abc")); ci.Availability != model.AvailabilityOK || len(s.calls) != 1 {
		t.Errorf("CI по коммиту: %+v, вызовы %v", ci, s.calls)
	}
}

func TestNoRef(t *testing.T) {
	ctx := context.Background()
	for _, p := range []RemoteProvider{GitHub{}, GitLab{}, Gitea{}} {
		q := Query{Dir: "/repo", Target: Target{Login: "home"}}
		if pr := p.PullRequests(ctx, q); model.CodeOf(pr.Error) != model.CodeInvalidInput {
			t.Errorf("%s PR: %+v", p.Provider(), pr)
		}
		if ci := p.CIRuns(ctx, q); model.CodeOf(ci.Error) != model.CodeInvalidInput {
			t.Errorf("%s CI: %+v", p.Provider(), ci)
		}
	}
}
