package providers

import (
	"testing"

	"github.com/forktopus/forktopus/internal/gitlocal"
	"github.com/forktopus/forktopus/internal/model"
)

// remote — RemoteInfo так, как его собирает сканер.
func remote(raw string) model.RemoteInfo {
	a := gitlocal.ParseRemoteURL(raw)
	return model.RemoteInfo{Name: "origin", FetchURL: a.URL, Host: a.Host, Path: a.Path}
}

var testLogins = []Login{
	{Provider: model.ProviderGitHub, Name: "github.com", URL: "https://github.com"},
	{Provider: model.ProviderGitLab, Name: "gitlab.com", URL: "https://gitlab.com"},
	{Provider: model.ProviderGitLab, Name: "code.corp", URL: "https://code.corp/gitlab"},
	{Provider: model.ProviderGitea, Name: "home", URL: "https://git.home.example", SSHHost: "ssh.home.example"},
	{Provider: model.ProviderGitea, Name: "home-bot", URL: "https://git.home.example"},
	{Provider: model.ProviderGitea, Name: "port", URL: "https://tea.example:3000"},
	{Provider: model.ProviderGitea, Name: "sub", URL: "https://mixed.example/gitea"},
	{Provider: model.ProviderGitLab, Name: "mixed.example", URL: "https://mixed.example/gitlab"},
	{Provider: model.ProviderGitea, Name: "both-tea", URL: "https://both.example"},
	{Provider: model.ProviderGitLab, Name: "both.example", URL: "https://both.example"},
	{Provider: model.ProviderGitea, Name: "def-b", URL: "https://pick.example", Default: true},
	{Provider: model.ProviderGitea, Name: "def-a", URL: "https://pick.example"},
}

func TestResolve(t *testing.T) {
	overrides := map[string]model.Provider{
		"both.example":     model.ProviderGitLab,
		"ghe.corp:8443":    model.ProviderGitHub,
		"plain.example":    model.ProviderOther,
		"gitea.com":        model.ProviderGitea,
		"forced.example":   model.ProviderGitea,
		"ssh-only.example": model.ProviderGitLab,
	}
	res := Resolver{Overrides: overrides, Logins: testLogins}

	cases := []struct {
		name   string
		remote string
		want   Target
		code   model.ErrorCode
	}{
		{"github ssh", "git@github.com:org/repo.git",
			Target{Provider: model.ProviderGitHub, Host: "github.com", BaseURL: "https://github.com", Project: "org/repo", Login: "github.com"}, ""},
		{"github https", "https://github.com/org/repo",
			Target{Provider: model.ProviderGitHub, Host: "github.com", BaseURL: "https://github.com", Project: "org/repo", Login: "github.com"}, ""},
		{"github без owner", "https://github.com/repo", Target{}, model.CodeRepoUnresolved},
		{"github с лишним сегментом", "https://github.com/org/repo/extra", Target{}, model.CodeRepoUnresolved},
		{"gitlab подгруппы", "git@gitlab.com:group/sub/repo.git",
			Target{Provider: model.ProviderGitLab, Host: "gitlab.com", BaseURL: "https://gitlab.com", Project: "group/sub/repo", Login: "gitlab.com"}, ""},
		{"gitlab self-hosted с подпутём по https", "https://code.corp/gitlab/team/repo.git",
			Target{Provider: model.ProviderGitLab, Host: "code.corp", BaseURL: "https://code.corp/gitlab", Project: "team/repo", Login: "code.corp"}, ""},
		{"gitlab self-hosted с подпутём по ssh", "ssh://git@code.corp:2222/team/repo.git",
			Target{Provider: model.ProviderGitLab, Host: "code.corp:2222", BaseURL: "https://code.corp/gitlab", Project: "team/repo", Login: "code.corp"}, ""},
		{"gitea по логину tea, логин по имени", "git@git.home.example:me/repo.git",
			Target{Provider: model.ProviderGitea, Host: "git.home.example", BaseURL: "https://git.home.example", Project: "me/repo", Login: "home"}, ""},
		{"gitea по ssh_host логина", "git@ssh.home.example:me/repo.git",
			Target{Provider: model.ProviderGitea, Host: "ssh.home.example", BaseURL: "https://git.home.example", Project: "me/repo", Login: "home"}, ""},
		{"gitea логин по умолчанию главнее имени", "https://pick.example/me/repo",
			Target{Provider: model.ProviderGitea, Host: "pick.example", BaseURL: "https://pick.example", Project: "me/repo", Login: "def-b"}, ""},
		{"https с портом совпадает с логином", "https://tea.example:3000/me/repo.git",
			Target{Provider: model.ProviderGitea, Host: "tea.example:3000", BaseURL: "https://tea.example:3000", Project: "me/repo", Login: "port"}, ""},
		{"https на другом порту — другой инстанс", "https://tea.example/me/repo.git", Target{}, model.CodeUnsupported},
		{"ssh с портом — порт ssh, не веб", "ssh://git@tea.example:22/me/repo.git",
			Target{Provider: model.ProviderGitea, Host: "tea.example:22", BaseURL: "https://tea.example:3000", Project: "me/repo", Login: "port"}, ""},
		{"два инстанса на одном хосте, выбор по base path", "https://mixed.example/gitea/me/repo",
			Target{Provider: model.ProviderGitea, Host: "mixed.example", BaseURL: "https://mixed.example/gitea", Project: "me/repo", Login: "sub"}, ""},
		{"два CLI на одном хосте без base path по ssh", "git@mixed.example:me/repo.git", Target{}, model.CodeUnsupported},
		{"логины двух CLI, решают настройки", "git@both.example:group/repo.git",
			Target{Provider: model.ProviderGitLab, Host: "both.example", BaseURL: "https://both.example", Project: "group/repo", Login: "both.example"}, ""},
		{"github enterprise из настроек без логина", "https://ghe.corp:8443/org/repo",
			Target{Provider: model.ProviderGitHub, Host: "ghe.corp:8443", Project: "org/repo"}, ""},
		{"настройка по имени хоста для ssh с портом", "ssh://git@ssh-only.example:2222/g/repo",
			Target{Provider: model.ProviderGitLab, Host: "ssh-only.example:2222", Project: "g/repo"}, ""},
		{"настройки главнее известного хоста", "https://gitea.com/me/repo",
			Target{Provider: model.ProviderGitea, Host: "gitea.com", Project: "me/repo"}, ""},
		{"в настройках тип other", "https://plain.example/me/repo", Target{}, model.CodeUnsupported},
		{"неизвестный self-hosted (A30)", "git@unknown.example:me/repo.git", Target{}, model.CodeUnsupported},
		{"локальный remote", "/srv/git/repo.git", Target{}, model.CodeUnsupported},
		{"gitea с подгруппой", "git@forced.example:a/b/repo.git", Target{}, model.CodeRepoUnresolved},
	}
	for _, c := range cases {
		got, err := res.Resolve(remote(c.remote))
		if code := codeOf(err); code != c.code {
			t.Errorf("%s: код %q, ожидался %q (%+v)", c.name, code, c.code, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
}

func codeOf(e *model.Error) model.ErrorCode {
	if e == nil {
		return ""
	}
	return e.Code
}

func TestResolveWithoutLogins(t *testing.T) {
	// Известные хосты определяются и без логинов: «нет авторизации» скажет адаптер.
	got, err := Resolver{}.Resolve(remote("git@gitlab.com:group/repo.git"))
	if err != nil || got != (Target{Provider: model.ProviderGitLab, Host: "gitlab.com", Project: "group/repo"}) {
		t.Errorf("gitlab.com без логинов: %+v %v", got, err)
	}
}
