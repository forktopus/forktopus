package providers

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
)

func TestListLinks(t *testing.T) {
	for _, tt := range []struct {
		provider                                   model.Provider
		base, project, branch, sha, prPath, ciPath string
	}{
		{model.ProviderGitHub, "https://github.com", "owner/repo", "feature/ветка&one", "", "/owner/repo/pulls", "/owner/repo/actions"},
		{model.ProviderGitLab, "https://web.example:8443/gitlab/", "group/sub/repo", "feature/x", "", "/gitlab/group/sub/repo/-/merge_requests", "/gitlab/group/sub/repo/-/pipelines"},
		{model.ProviderGitea, "https://web.example/gitea", "owner/repo", "main", "", "/gitea/owner/repo/pulls", "/gitea/owner/repo/actions"},
		{model.ProviderGitHub, "https://github.com", "owner/repo", "", "abcdef0123456789", "/owner/repo/pulls", "/owner/repo/actions"},
		{model.ProviderGitLab, "https://gitlab.com", "owner/repo", "", "abcdef0123456789", "/owner/repo/-/merge_requests", "/owner/repo/-/commit/abcdef0123456789/pipelines"},
	} {
		t.Run(string(tt.provider)+tt.branch+tt.sha, func(t *testing.T) {
			q := Query{Target: Target{Provider: tt.provider, Host: "ssh.other.example:2222", BaseURL: tt.base, Project: tt.project}, Branch: tt.branch, SHA: tt.sha}
			pr, _ := url.Parse(listURL(q, false))
			ci, _ := url.Parse(listURL(q, true))
			if pr.Path != tt.prPath || ci.Path != tt.ciPath || strings.Contains(pr.String(), "ssh.other") {
				t.Fatalf("неверные ссылки: %v %v", pr, ci)
			}
			if tt.branch != "" && tt.provider == model.ProviderGitHub && pr.Query().Get("q") != "is:pr is:open head:"+tt.branch {
				t.Fatalf("потерян фильтр: %v", pr)
			}
			if tt.branch != "" && tt.provider == model.ProviderGitLab && (pr.Query().Get("source_branch") != tt.branch || ci.Query().Get("ref") != tt.branch) {
				t.Fatal("потерян фильтр GitLab")
			}
		})
	}
}

func TestNoGuessedListAddress(t *testing.T) {
	for _, base := range []string{"", "ssh://host/base", "https://host/base?token=secret", "https://host/base#fragment", "https://%invalid"} {
		if got := listURL(Query{Target: Target{Provider: model.ProviderGitea, BaseURL: base, Host: "ssh.host:2222", Project: "o/r"}}, false); got != "" {
			t.Fatalf("адрес угадан: %s", got)
		}
	}
	q := Query{Target: Target{Provider: model.ProviderGitLab, BaseURL: "https://user:secret@web.example/base", Project: "group/Мой проект"}}
	if got := listURL(q, false); strings.Contains(got, "secret") || !strings.Contains(got, "%20") {
		t.Fatalf("небезопасный адрес: %s", got)
	}
}

// Проверяем настоящие адаптеры и нулевые ответы: список не исчезает только
// потому, что нет одного PR или одного запуска, на который можно сослаться.
func TestZeroCountHasListLink(t *testing.T) {
	ctx := context.Background()
	gh, _ := ghWith(map[string]reply{"pr list": stdout([]byte("[]")), "run list": stdout([]byte("[]"))})
	pr := gh.PullRequests(ctx, query("main"))
	ci := gh.CIRuns(ctx, query("main"))
	if pr.ListURL == "" || ci.ListURL == "" || pr.URL != "" || ci.URL != "" {
		t.Fatalf("gh: %+v %+v", pr, ci)
	}
	gl, _ := glWith(map[string]reply{"mr list": stdout([]byte("[]")), "ci list": stdout([]byte("[]"))})
	pr = gl.PullRequests(ctx, glQuery("main"))
	ci = gl.CIRuns(ctx, glQuery("main"))
	if pr.ListURL == "" || ci.ListURL == "" || pr.URL != "" || ci.URL != "" {
		t.Fatalf("gl: %+v %+v", pr, ci)
	}
	gt, _ := gtWith(map[string]reply{"pulls list": stdout([]byte("[]")), "api": apiReply("HTTP/1.1 200 OK", `{"workflow_runs":[]}`)})
	pr = gt.PullRequests(ctx, gtQuery("main"))
	ci = gt.CIRuns(ctx, gtQuery("main"))
	if pr.ListURL == "" || ci.ListURL == "" {
		t.Fatalf("gitea: %+v %+v", pr, ci)
	}
}
