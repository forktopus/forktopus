package providers

import (
	"cmp"
	"net"
	"net/url"
	"slices"
	"strings"

	"github.com/forktopus/forktopus/internal/gitlocal"
	"github.com/forktopus/forktopus/internal/model"
)

// Resolver сопоставляет remote с проектом на сервере (D-042). Тип сервера:
// настройки пользователя → известный публичный хост → CLI, у которого есть
// логин на этот хост. Base path и имя логина берутся из логина CLI.
type Resolver struct {
	// Overrides — тип провайдера из настроек: host[:port] или host → провайдер.
	Overrides map[string]model.Provider
	// Logins — логины gh, glab, gt и tea (DiscoverLogins).
	Logins []Login
}

// candidate — логин, подходящий к remote, и путь проекта относительно его base path.
type candidate struct {
	login   Login
	project string
	baseLen int // длина отрезанного base path: чем длиннее, тем точнее совпадение
}

// Resolve находит проект remote. Ошибки: UNSUPPORTED — для сервера нет
// адаптера (PR/CI = not_supported, A30); REPO_UNRESOLVED — путь remote не похож
// на проект этого провайдера.
func (r Resolver) Resolve(remote model.RemoteInfo) (Target, *model.Error) {
	if remote.Host == "" {
		return Target{}, model.Errorf(model.CodeUnsupported, "remote %s — локальный путь, серверных данных нет", remote.Name)
	}
	cands := r.matching(remote)
	provider, err := r.provider(remote.Host, cands)
	if err != nil {
		return Target{}, err
	}

	t := Target{Provider: provider, Host: remote.Host, Project: remote.Path}
	if c, ok := best(cands, provider); ok {
		t.BaseURL, t.Login, t.Project, t.Tool = c.login.URL, c.login.Name, c.project, c.login.Tool
	}
	if !validProject(provider, t.Project) {
		want := "owner/repo"
		if provider == model.ProviderGitLab {
			want = "group/…/repo"
		}
		return Target{}, model.Errorf(model.CodeRepoUnresolved, "путь %q на %s не похож на проект (ожидается %s)", t.Project, remote.Host, want)
	}
	return t, nil
}

// provider выбирает тип сервера.
func (r Resolver) provider(host string, cands []candidate) (model.Provider, *model.Error) {
	p, ok := r.Overrides[host]
	if !ok {
		p, ok = r.Overrides[hostname(host)]
	}
	if !ok {
		if known := gitlocal.DetectProvider(host, nil); known != model.ProviderOther {
			return known, nil
		}
		var found []model.Provider
		for _, c := range cands {
			if !slices.Contains(found, c.login.Provider) {
				found = append(found, c.login.Provider)
			}
		}
		switch len(found) {
		case 0:
			return "", model.Errorf(model.CodeUnsupported, "для сервера %s нет адаптера: тип не указан в настройках, и ни у gh, glab, gt, tea нет логина на этот хост", host)
		case 1:
			return found[0], nil
		}
		slices.Sort(found)
		names := make([]string, len(found))
		for i, f := range found {
			names[i] = string(f)
		}
		return "", model.Errorf(model.CodeUnsupported, "на сервер %s есть логины нескольких провайдеров (%s) — укажите тип сервера в настройках", host, strings.Join(names, ", "))
	}
	switch p {
	case model.ProviderGitHub, model.ProviderGitLab, model.ProviderGitea:
		return p, nil
	}
	return "", model.Errorf(model.CodeUnsupported, "для сервера %s в настройках указан тип без адаптера: %q", host, p)
}

// matching отбирает логины, которые подходят к remote. У HTTP(S)-remote должен
// совпасть host[:port] и base path; у SSH-remote порт — порт SSH, поэтому
// сравнивается только имя хоста (или ssh_host логина), а base path в пути
// обычно отсутствует.
func (r Resolver) matching(remote model.RemoteInfo) []candidate {
	web := isWebURL(remote.FetchURL)
	var out []candidate
	for _, l := range r.Logins {
		u, err := url.Parse(l.URL)
		if err != nil || u.Host == "" {
			continue
		}
		lhost := strings.ToLower(u.Host)
		if web {
			if lhost != remote.Host {
				continue
			}
		} else if hostname(lhost) != hostname(remote.Host) && (l.SSHHost == "" || hostname(l.SSHHost) != hostname(remote.Host)) {
			continue
		}
		c := candidate{login: l, project: remote.Path}
		if base := strings.Trim(u.Path, "/"); base != "" {
			if rest, ok := strings.CutPrefix(remote.Path, base+"/"); ok {
				c.project, c.baseLen = rest, len(base)
			} else if web {
				continue // другой инстанс на том же хосте
			}
		}
		out = append(out, c)
	}
	return out
}

// best — лучший логин провайдера: самый длинный base path, затем логин gt
// раньше tea (tea — запасной, D-074), затем логин по умолчанию, затем по
// имени — чтобы выбор не зависел от порядка вывода CLI.
func best(cands []candidate, p model.Provider) (candidate, bool) {
	cands = slices.DeleteFunc(slices.Clone(cands), func(c candidate) bool { return c.login.Provider != p })
	if len(cands) == 0 {
		return candidate{}, false
	}
	slices.SortStableFunc(cands, func(a, b candidate) int {
		if c := cmp.Compare(b.baseLen, a.baseLen); c != 0 {
			return c
		}
		if (a.login.Tool == "tea") != (b.login.Tool == "tea") {
			if b.login.Tool == "tea" {
				return -1
			}
			return 1
		}
		if a.login.Default != b.login.Default {
			if a.login.Default {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.login.Name, b.login.Name)
	})
	return cands[0], true
}

// validProject: у GitHub и Gitea проект — ровно owner/repo, у GitLab — группы
// любой вложенности и проект.
func validProject(p model.Provider, project string) bool {
	parts := strings.Split(project, "/")
	if slices.Contains(parts, "") {
		return false
	}
	if p == model.ProviderGitLab {
		return len(parts) >= 2
	}
	return len(parts) == 2
}

func isWebURL(raw string) bool {
	raw = strings.ToLower(raw)
	return strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://")
}

// hostname — host без порта.
func hostname(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return strings.Trim(host, "[]")
}
