package providers

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// Login — учётная запись CLI на сервере. По логинам резолвер узнаёт тип
// self-hosted сервера и его base path (D-042).
type Login struct {
	Provider model.Provider
	// Name — имя логина для явного выбора (`tea -l`, у gt — пользователь
	// для `--user`); у gh и glab — хост.
	Name string
	// URL — адрес инстанса `https://host[:port][/base]`.
	URL string
	// SSHHost — хост SSH-remote, если он не совпадает с хостом URL (tea).
	SSHHost string
	Default bool
	// Tool — CLI логина Gitea: gt (основной) или tea (запасной); у gh и glab пусто.
	Tool string
}

// loginsTimeout — glab без сети отвечает через ~12 с (docs/providers.md).
const loginsTimeout = 20 * time.Second

// loginSources — команды, которые перечисляют логины, и разбор их вывода.
var loginSources = []struct {
	cmd   runner.Command
	parse func([]byte) []Login
}{
	{runner.Command{Name: "gh", Args: []string{"auth", "status", "--json", "hosts"}, Timeout: loginsTimeout}, parseGHLogins},
	{runner.Command{Name: "glab", Args: []string{"auth", "status", "--all", "--output", "json"}, Timeout: loginsTimeout}, parseGlabLogins},
	{runner.Command{Name: "gt", Args: []string{"auth", "status", "--json", "host,user,active"}, Timeout: loginsTimeout}, parseGtLogins},
	{runner.Command{Name: "tea", Args: []string{"logins", "list", "-o", "json"}, Timeout: loginsTimeout}, parseTeaLogins},
}

// DiscoverLogins опрашивает gh, glab, gt и tea параллельно. CLI, которого нет или
// который не ответил, логинов не даёт: его состояние покажет Diagnose адаптера.
// Код завершения не проверяется: gh и glab выходят с 1, если хоть один логин
// не прошёл проверку, но список хостов при этом печатают.
func DiscoverLogins(ctx context.Context, r runner.Runner) []Login {
	found := make([][]Login, len(loginSources))
	var wg sync.WaitGroup
	for i, src := range loginSources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res, err := r.Run(ctx, src.cmd); err == nil {
				found[i] = src.parse(res.Stdout)
			}
		}()
	}
	wg.Wait()
	var all []Login
	for _, l := range found {
		all = append(all, l...)
	}
	return all
}

// parseGHLogins разбирает `gh auth status --json hosts`:
// {"hosts":{"HOST":[{"host","login","state","active",…}]}}.
func parseGHLogins(out []byte) []Login {
	var v struct {
		Hosts map[string][]json.RawMessage `json:"hosts"`
	}
	if json.Unmarshal(out, &v) != nil {
		return nil
	}
	var logins []Login
	for host, accounts := range v.Hosts {
		if host = strings.ToLower(host); host == "" || len(accounts) == 0 {
			continue
		}
		logins = append(logins, Login{Provider: model.ProviderGitHub, Name: host, URL: "https://" + host})
	}
	slices.SortFunc(logins, func(a, b Login) int { return strings.Compare(a.Name, b.Name) })
	return logins
}

// parseGlabLogins разбирает `glab auth status --all --output json`:
// {"hosts":[{"host","api_endpoint":"https://HOST[/base]/api/v4/",…}]}.
func parseGlabLogins(out []byte) []Login {
	var v struct {
		Hosts []struct {
			Host        string `json:"host"`
			APIEndpoint string `json:"api_endpoint"`
		} `json:"hosts"`
	}
	if json.Unmarshal(out, &v) != nil {
		return nil
	}
	var logins []Login
	for _, h := range v.Hosts {
		host := strings.ToLower(h.Host)
		if host == "" {
			continue
		}
		base := strings.TrimSuffix(strings.TrimSuffix(h.APIEndpoint, "/"), "/api/v4")
		if _, err := url.Parse(base); err != nil || !strings.Contains(base, "://") {
			base = "https://" + host
		}
		logins = append(logins, Login{Provider: model.ProviderGitLab, Name: host, URL: base})
	}
	return logins
}

// parseTeaLogins разбирает `tea logins list -o json`:
// [{"name","url","ssh_host","user","default":"true"|"false"}] — все значения строки.
func parseTeaLogins(out []byte) []Login {
	var v []struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		SSHHost string `json:"ssh_host"`
		Default string `json:"default"`
	}
	if json.Unmarshal(out, &v) != nil {
		return nil
	}
	var logins []Login
	for _, l := range v {
		if l.Name == "" || l.URL == "" {
			continue
		}
		logins = append(logins, Login{
			Provider: model.ProviderGitea,
			Name:     l.Name,
			URL:      strings.TrimSuffix(l.URL, "/"),
			SSHHost:  strings.ToLower(l.SSHHost),
			Default:  l.Default == "true",
			Tool:     "tea",
		})
	}
	return logins
}

// parseGtLogins разбирает `gt auth status --json host,user,active`:
// [{"host":"https://HOST[/base]","user","active"}]. С --json gt выходит с 0,
// даже если учётка не работает: это покажет Diagnose.
func parseGtLogins(out []byte) []Login {
	var v []struct {
		Host   string `json:"host"`
		User   string `json:"user"`
		Active bool   `json:"active"`
	}
	if json.Unmarshal(out, &v) != nil {
		return nil
	}
	var logins []Login
	for _, l := range v {
		if l.Host == "" || l.User == "" {
			continue
		}
		logins = append(logins, Login{
			Provider: model.ProviderGitea,
			Name:     l.User,
			URL:      strings.TrimSuffix(l.Host, "/"),
			Default:  l.Active,
			Tool:     "gt",
		})
	}
	return logins
}
