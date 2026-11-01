package providers

import (
	"context"
	"reflect"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// Ответы CLI записаны с пилота (docs/providers.md), имена и хосты заменены.
const (
	ghHosts = `{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"OWNER","tokenSource":"keyring","scopes":"repo","gitProtocol":"ssh"}],
"GHE.corp":[{"state":"error","active":true,"host":"ghe.corp","login":"OWNER","tokenSource":"keyring"}]}}`
	glabHosts = `{"hosts":[{"host":"gitlab.com","state":"success","user":"OWNER","token_source":"keyring","git_protocol":"ssh","api_protocol":"https","api_endpoint":"https://gitlab.com/api/v4/","graphql_endpoint":"https://gitlab.com/api/graphql/"},
{"host":"code.corp","state":"error","api_endpoint":"https://code.corp/gitlab/api/v4/"}]}`
	teaLogins = `[{"name":"work","url":"https://git.work.example","ssh_host":"git.work.example","user":"OWNER","default":"false"},
{"name":"home","url":"https://git.home.example/","ssh_host":"SSH.home.example","user":"OWNER","default":"true"}]`
	// gt 0.2.0: `gt auth status --json host,user,active`.
	gtAccounts = `[{"active":true,"host":"https://git.home.example","user":"OWNER"},{"active":false,"host":"https://git.home.example/","user":"bot"},{"active":true,"host":"","user":"x"}]`
)

func TestParseLogins(t *testing.T) {
	gh := []Login{
		{Provider: model.ProviderGitHub, Name: "ghe.corp", URL: "https://ghe.corp"},
		{Provider: model.ProviderGitHub, Name: "github.com", URL: "https://github.com"},
	}
	if got := parseGHLogins([]byte(ghHosts)); !reflect.DeepEqual(got, gh) {
		t.Errorf("gh:\n got %+v\nwant %+v", got, gh)
	}
	glab := []Login{
		{Provider: model.ProviderGitLab, Name: "gitlab.com", URL: "https://gitlab.com"},
		{Provider: model.ProviderGitLab, Name: "code.corp", URL: "https://code.corp/gitlab"},
	}
	if got := parseGlabLogins([]byte(glabHosts)); !reflect.DeepEqual(got, glab) {
		t.Errorf("glab:\n got %+v\nwant %+v", got, glab)
	}
	tea := []Login{
		{Provider: model.ProviderGitea, Name: "work", URL: "https://git.work.example", SSHHost: "git.work.example", Tool: "tea"},
		{Provider: model.ProviderGitea, Name: "home", URL: "https://git.home.example", SSHHost: "ssh.home.example", Default: true, Tool: "tea"},
	}
	if got := parseTeaLogins([]byte(teaLogins)); !reflect.DeepEqual(got, tea) {
		t.Errorf("tea:\n got %+v\nwant %+v", got, tea)
	}
	gt := []Login{
		{Provider: model.ProviderGitea, Name: "OWNER", URL: "https://git.home.example", Default: true, Tool: "gt"},
		{Provider: model.ProviderGitea, Name: "bot", URL: "https://git.home.example", Tool: "gt"},
	}
	if got := parseGtLogins([]byte(gtAccounts)); !reflect.DeepEqual(got, gt) {
		t.Errorf("gt:\n got %+v\nwant %+v", got, gt)
	}

	// Ошибка вместо JSON (glab пишет её в stdout) и пустой вывод — логинов нет.
	for _, out := range []string{`{"error":{"message":"401 Unauthorized"}}`, "", "not json"} {
		if l := append(append(parseGHLogins([]byte(out)), parseGlabLogins([]byte(out))...), append(parseTeaLogins([]byte(out)), parseGtLogins([]byte(out))...)...); len(l) != 0 {
			t.Errorf("%q: %+v", out, l)
		}
	}
}

// cliRunner отвечает по имени CLI; CLI без ответа «не установлен».
type cliRunner map[string]runner.Result

func (c cliRunner) Run(_ context.Context, cmd runner.Command) (runner.Result, error) {
	if res, ok := c[cmd.Name]; ok {
		return res, nil
	}
	return runner.Result{}, model.Errorf(model.CodeCLIMissing, "%s не найден в PATH", cmd.Name)
}

func TestDiscoverLogins(t *testing.T) {
	r := cliRunner{
		// gh выходит с 1, если один из логинов не прошёл проверку, — хосты всё равно нужны.
		"gh":  {Stdout: []byte(ghHosts), ExitCode: 1},
		"gt":  {Stdout: []byte(gtAccounts)},
		"tea": {Stdout: []byte(teaLogins)},
	}
	got := DiscoverLogins(context.Background(), r)
	var names []string
	for _, l := range got {
		names = append(names, string(l.Provider)+":"+l.Name)
	}
	want := []string{"github:ghe.corp", "github:github.com", "gitea:OWNER", "gitea:bot", "gitea:work", "gitea:home"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("логины: %v, ожидались %v", names, want)
	}
}
