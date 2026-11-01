package providers

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/model"
)

// Закрытые PR: фикстуры *-closed.json — ответы с пилотов (PR #3 закрыт без
// слияния), влитые и чужие записи собраны по тем же форматам — влитых PR в
// пилотах нет.

func wantLast(t *testing.T, name string, got model.PRSummary, number int, merged bool, url, at string) {
	t.Helper()
	if got.Availability != model.AvailabilityOK || got.Open == nil || *got.Open != 0 {
		t.Fatalf("%s: сводка %+v", name, got)
	}
	l := got.Last
	if l == nil || l.Number != number || l.Merged != merged || l.URL != url || l.ClosedAt == nil {
		t.Fatalf("%s: last %+v", name, l)
	}
	if want, _ := time.Parse(time.RFC3339, at); !l.ClosedAt.Equal(want) {
		t.Errorf("%s: время %v, ожидалось %v", name, l.ClosedAt, want)
	}
}

func TestGitHubLastClosed(t *testing.T) {
	ctx := context.Background()
	open := "pr list -R github.com/OWNER/forktopus-pilot --head pilot/closed-pr --state open"
	closed := "pr list -R github.com/OWNER/forktopus-pilot --head pilot/closed-pr --state closed"

	g, s := ghWith(map[string]reply{open: stdout([]byte("[]")), closed: stdout(fixture(t, "github/pr-closed.json"))})
	wantLast(t, "закрыт без слияния", g.PullRequests(ctx, query("pilot/closed-pr")),
		3, false, "https://github.com/OWNER/forktopus-pilot/pull/3", "2026-10-31T13:56:29Z")
	if args := s.calls[1].Args; !slices.Contains(args, "closed") || s.calls[1].Dir != "/repo" {
		t.Errorf("команда %v", args)
	}

	merged := `[{"closedAt":"2026-11-01T09:00:00Z","headRefName":"pilot/closed-pr","isCrossRepository":true,"mergedAt":"2026-11-01T09:00:00Z","number":9,"state":"MERGED","url":"https://github.com/OWNER/forktopus-pilot/pull/9"},
{"closedAt":"2026-10-31T15:00:00Z","headRefName":"pilot/closed-pr","isCrossRepository":false,"mergedAt":"2026-10-31T15:00:00Z","number":5,"state":"MERGED","url":"https://github.com/OWNER/forktopus-pilot/pull/5"},
` + strings.TrimPrefix(string(fixture(t, "github/pr-closed.json")), "[")
	g, _ = ghWith(map[string]reply{open: stdout([]byte("[]")), closed: stdout([]byte(merged))})
	wantLast(t, "влитый позже закрытого, форк не считается", g.PullRequests(ctx, query("pilot/closed-pr")),
		5, true, "https://github.com/OWNER/forktopus-pilot/pull/5", "2026-10-31T15:00:00Z")

	// Ошибка второго запроса сводку не портит.
	g, _ = ghWith(map[string]reply{open: stdout([]byte("[]")), closed: stderr(1, "HTTP 502")})
	if got := g.PullRequests(ctx, query("pilot/closed-pr")); got.Availability != model.AvailabilityOK || got.Last != nil || got.Error != nil {
		t.Errorf("ошибка закрытых: %+v", got)
	}

	// Есть открытые — закрытые не запрашиваются.
	g, s = ghWith(map[string]reply{"pr list": stdout(fixture(t, "github/pr-open.json"))})
	if got := g.PullRequests(ctx, query("pilot/pr-open")); got.Last != nil || len(s.calls) != 1 {
		t.Errorf("при открытых: %+v, вызовов %d", got, len(s.calls))
	}
}

func TestGitLabLastClosed(t *testing.T) {
	ctx := context.Background()
	openKey := "mr list -R https://gitlab.com/OWNER/forktopus-pilot --source-branch pilot/closed-pr --per-page"
	allKey := "mr list -R https://gitlab.com/OWNER/forktopus-pilot --source-branch pilot/closed-pr --all"

	g, s := glWith(map[string]reply{openKey: stdout([]byte("[]")), allKey: stdout(fixture(t, "gitlab/mr-closed.json"))})
	wantLast(t, "закрыт без слияния", g.PullRequests(ctx, glQuery("pilot/closed-pr")),
		3, false, "https://gitlab.com/OWNER/forktopus-pilot/-/merge_requests/3", "2026-10-31T14:02:20.028Z")
	if len(s.calls) != 2 {
		t.Errorf("вызовы %v", s.calls)
	}

	all := `[{"iid":7,"state":"opened","source_branch":"pilot/closed-pr","source_project_id":88339103,"target_project_id":88339103,"web_url":"u7","merged_at":null,"closed_at":null},
{"iid":6,"state":"merged","source_branch":"pilot/closed-pr","source_project_id":1,"target_project_id":88339103,"web_url":"u6","merged_at":"2026-11-01T10:00:00Z","closed_at":null},
{"iid":5,"state":"merged","source_branch":"pilot/closed-pr","source_project_id":88339103,"target_project_id":88339103,"web_url":"u5","merged_at":"2026-10-31T15:00:00.5Z","closed_at":null},
` + strings.TrimPrefix(string(fixture(t, "gitlab/mr-closed.json")), "[")
	g, _ = glWith(map[string]reply{openKey: stdout([]byte("[]")), allKey: stdout([]byte(all))})
	wantLast(t, "влитый; открытый и из форка не считаются", g.PullRequests(ctx, glQuery("pilot/closed-pr")),
		5, true, "u5", "2026-10-31T15:00:00.5Z")
}

func TestGiteaLastClosed(t *testing.T) {
	ctx := context.Background()
	open := "pulls list"
	closed := "api -l home -i /repos/OWNER/forktopus-pilot/pulls?state=closed"

	g, _ := gtWith(map[string]reply{open: stdout([]byte("[]")), closed: apiReply("HTTP/2.0 200 OK", string(fixture(t, "gitea/pulls-closed.json")))})
	wantLast(t, "закрыт без слияния", g.PullRequests(ctx, gtQuery("pilot/closed-pr")),
		3, false, "https://git.example.org/OWNER/forktopus-pilot/pulls/3", "2026-10-31T17:07:36+03:00")

	list := `[{"number":8,"merged":true,"merged_at":"2026-11-01T10:00:00+03:00","closed_at":"2026-11-01T10:00:00+03:00","html_url":"u8","head":{"ref":"pilot/closed-pr","repo":{"id":40}},"base":{"ref":"main","repo":{"id":25}}},
{"number":7,"merged":false,"closed_at":"2026-11-01T09:00:00+03:00","html_url":"u7","head":{"ref":"other","repo":{"id":25}},"base":{"ref":"main","repo":{"id":25}}},
{"number":6,"merged":true,"merged_at":"2026-10-31T20:00:00+03:00","closed_at":"2026-10-31T20:00:00+03:00","html_url":"u6","head":{"ref":"pilot/closed-pr","repo":null},"base":{"ref":"main","repo":{"id":25}}},
{"number":5,"merged":true,"merged_at":"2026-10-31T19:00:00+03:00","closed_at":"2026-10-31T19:00:00+03:00","html_url":"u5","head":{"ref":"pilot/closed-pr","repo":{"id":25}},"base":{"ref":"main","repo":{"id":25}}},
` + strings.TrimPrefix(string(fixture(t, "gitea/pulls-closed.json")), "[")
	g, _ = gtWith(map[string]reply{open: stdout([]byte("[]")), closed: apiReply("HTTP/2.0 200 OK", list)})
	wantLast(t, "влитый; форк, удалённый форк и чужая ветка не считаются", g.PullRequests(ctx, gtQuery("pilot/closed-pr")),
		5, true, "u5", "2026-10-31T19:00:00+03:00")

	g, _ = gtWith(map[string]reply{open: stdout([]byte("[]")), closed: apiReply("HTTP/2.0 404 Not Found", `{"message":"not found"}`)})
	if got := g.PullRequests(ctx, gtQuery("pilot/closed-pr")); got.Availability != model.AvailabilityOK || got.Last != nil {
		t.Errorf("ошибка закрытых: %+v", got)
	}
}

func TestLatestPR(t *testing.T) {
	at := func(s string) *time.Time { return parseTime(s) }
	if latestPR(nil) != nil {
		t.Error("пусто — nil")
	}
	got := latestPR([]model.PRRef{{Number: 1}, {Number: 2, ClosedAt: at("2026-01-01T00:00:00Z")}, {Number: 3, ClosedAt: at("2026-01-02T00:00:00+03:00")}})
	if got.Number != 3 {
		t.Errorf("последний по времени: %+v", got)
	}
	if parseTime("") != nil || parseTime("вчера") != nil {
		t.Error("неразобранное время — nil")
	}
}
