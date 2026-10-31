package gitlocal

import (
	"testing"

	"github.com/forktopus/forktopus/internal/model"
)

func TestParseRemoteURL(t *testing.T) {
	cases := []struct {
		raw  string
		want RemoteAddress
	}{
		{"git@github.com:org/repo.git", RemoteAddress{"git@github.com:org/repo.git", "github.com", "org/repo"}},
		{"github.com:org/repo", RemoteAddress{"github.com:org/repo", "github.com", "org/repo"}},
		{"ssh://git@git.company:2222/team/repo.git", RemoteAddress{"ssh://git.company:2222/team/repo.git", "git.company:2222", "team/repo"}},
		{"https://git.company/base/owner/repo.git", RemoteAddress{"https://git.company/base/owner/repo.git", "git.company", "base/owner/repo"}},
		{"https://user:s3cret@GitHub.com/org/repo.git/", RemoteAddress{"https://GitHub.com/org/repo.git/", "github.com", "org/repo"}},
		{"https://ghp_token@github.com/org/repo", RemoteAddress{"https://github.com/org/repo", "github.com", "org/repo"}},
		{"git://example.org/group/sub/repo.git", RemoteAddress{"git://example.org/group/sub/repo.git", "example.org", "group/sub/repo"}},
		{"file:///srv/git/repo.git", RemoteAddress{"file:///srv/git/repo.git", "", ""}},
		{"/srv/repos/Мои проекты/demo.git", RemoteAddress{"/srv/repos/Мои проекты/demo.git", "", ""}},
		{"../relative/repo", RemoteAddress{"../relative/repo", "", ""}},
		{"./a:b", RemoteAddress{"./a:b", "", ""}},
		{`C:\repos\demo`, RemoteAddress{`C:\repos\demo`, "", ""}},
		{"persistent-https://tok@host.example/repo", RemoteAddress{"persistent-https://host.example/repo", "host.example", "repo"}},
	}
	for _, c := range cases {
		if got := ParseRemoteURL(c.raw); got != c.want {
			t.Errorf("ParseRemoteURL(%q)\n got %+v\nwant %+v", c.raw, got, c.want)
		}
	}
}

func TestServerID(t *testing.T) {
	if id := ParseRemoteURL("ssh://git@git.company:2222/team/repo.git").ServerID(); id != "git.company:2222/team/repo" {
		t.Errorf("ServerID: %q", id)
	}
	if id := ParseRemoteURL("/srv/demo.git").ServerID(); id != "" {
		t.Errorf("ServerID локального пути: %q", id)
	}
}

func TestDetectProvider(t *testing.T) {
	overrides := map[string]model.Provider{"git.home.test": model.ProviderGitea, "git.company:2222": model.ProviderGitLab}
	cases := map[string]model.Provider{
		"github.com":       model.ProviderGitHub,
		"gitlab.com":       model.ProviderGitLab,
		"gitea.com":        model.ProviderGitea,
		"github.com:443":   model.ProviderGitHub,
		"git.home.test":    model.ProviderGitea,
		"git.company:2222": model.ProviderGitLab,
		"git.company":      model.ProviderOther,
		"":                 model.ProviderOther,
	}
	for host, want := range cases {
		if got := DetectProvider(host, overrides); got != want {
			t.Errorf("DetectProvider(%q) = %q, want %q", host, got, want)
		}
	}
}
