package gitlocal

import (
	"net/url"
	"strings"

	"github.com/forktopus/forktopus/internal/model"
)

// RemoteAddress — разобранный адрес remote.
type RemoteAddress struct {
	// URL — адрес для показа: без пароля и токена в userinfo.
	URL string
	// Host — host[:port] сервера; пусто для локальных путей и неразобранных адресов.
	Host string
	// Path — путь проекта на сервере без ведущего «/» и суффикса .git;
	// для self-hosted сохраняется base path (`base/owner/repo`).
	Path string
}

// ServerID — серверный идентификатор проекта: host[:port]/path.
func (a RemoteAddress) ServerID() string {
	if a.Host == "" {
		return ""
	}
	return a.Host + "/" + a.Path
}

// ParseRemoteURL разбирает адрес remote в формах, которые понимает Git:
// URL со схемой (ssh://, https://, git://, file://), scp-подобную
// `[user@]host:path` и локальный путь. Порт и base path сохраняются,
// userinfo в URL со схемой отбрасывается целиком (там бывает токен).
func ParseRemoteURL(raw string) RemoteAddress {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "://"); i > 0 && !strings.ContainsAny(raw[:i], "/:@") {
		u, err := url.Parse(raw)
		if err != nil {
			return RemoteAddress{URL: stripUserinfo(raw, i)}
		}
		u.User = nil
		addr := RemoteAddress{URL: u.String()}
		if u.Scheme != "file" && u.Host != "" {
			addr.Host = strings.ToLower(u.Host)
			addr.Path = cleanPath(u.Path)
		}
		return addr
	}
	if host, path, ok := splitSCP(raw); ok {
		return RemoteAddress{URL: raw, Host: strings.ToLower(host), Path: cleanPath(path)}
	}
	// Локальный путь или адрес, который мы не умеем разбирать.
	return RemoteAddress{URL: raw}
}

// splitSCP разбирает `[user@]host:path`. По правилам Git это scp-форма, только
// если двоеточие встречается раньше первого «/»; `./a:b` и `/a:b` — пути.
func splitSCP(raw string) (host, path string, ok bool) {
	colon := strings.Index(raw, ":")
	if colon <= 0 {
		return "", "", false
	}
	if slash := strings.Index(raw, "/"); slash >= 0 && slash < colon {
		return "", "", false
	}
	host = raw[:colon]
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	// Буква диска Windows (`C:\repo`) — тоже путь.
	if host == "" || len(raw[:colon]) == 1 {
		return "", "", false
	}
	return host, raw[colon+1:], true
}

// stripUserinfo убирает userinfo из URL, который не разобрал net/url.
func stripUserinfo(raw string, schemeEnd int) string {
	rest := raw[schemeEnd+3:]
	end := strings.IndexAny(rest, "/?#")
	if end < 0 {
		end = len(rest)
	}
	if at := strings.LastIndex(rest[:end], "@"); at >= 0 {
		return raw[:schemeEnd+3] + rest[at+1:]
	}
	return raw
}

func cleanPath(p string) string {
	p = strings.Trim(p, "/")
	p = strings.TrimSuffix(p, ".git")
	return strings.TrimSuffix(p, "/")
}

// knownHosts — публичные инстансы, тип которых известен без настроек.
var knownHosts = map[string]model.Provider{
	"github.com": model.ProviderGitHub,
	"gitlab.com": model.ProviderGitLab,
	"gitea.com":  model.ProviderGitea,
}

// DetectProvider определяет провайдера по хосту: известные публичные
// инстансы, затем настройки пользователя (ключ host[:port]), иначе other.
func DetectProvider(host string, overrides map[string]model.Provider) model.Provider {
	if host == "" {
		return model.ProviderOther
	}
	if p, ok := overrides[host]; ok && p != "" {
		return p
	}
	name := host
	if i := strings.LastIndex(host, ":"); i >= 0 && !strings.Contains(host[i:], "]") {
		name = host[:i]
	}
	if p, ok := knownHosts[name]; ok {
		return p
	}
	return model.ProviderOther
}
