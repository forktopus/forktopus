package runner

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// userinfoRe находит userinfo в URL со схемой: https://user:token@host.
// scp-форма git@host:org/repo не трогается — там нет секрета.
var userinfoRe = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/@\s]+@`)

// RedactURLs маскирует логины и токены в URL внутри произвольного текста.
// Применять ко всему, что попадает в логи и сообщения для UI.
func RedactURLs(s string) string {
	return userinfoRe.ReplaceAllString(s, "${1}***@")
}

// secretRes — токены в виде, в котором их печатают gh, glab, tea и HTTP:
// префиксы токенов GitHub / GitLab, заголовок Authorization и пары
// «token=…», «password: …».
var secretRes = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`\b(gh[pousr]_|github_pat_|glpat-)[A-Za-z0-9_-]+`), "${1}***"},
	{regexp.MustCompile(`(?i)\b(authorization:\s*(?:bearer|token|basic)\s+)\S+`), "${1}***"},
	{regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9._~+/=-]{8,}`), "${1}***"},
	{regexp.MustCompile(`(?i)\b([a-z_]*(?:token|password|passwd|secret))(\s*[=:]\s*)[^\s,;&"']+`), "${1}${2}***"},
}

// Redact маскирует URL с userinfo и похожие на токены строки. Для логов:
// туда попадает вывод CLI, формат которого мы не контролируем.
func Redact(s string) string {
	s = RedactURLs(s)
	for _, r := range secretRes {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}

// briefLimit — сколько символов ошибки попадает в лог.
const briefLimit = 200

// Brief — первая непустая строка текста, замаскированная и обрезанная до
// briefLimit символов: краткая ошибка для лога без полного вывода CLI.
func Brief(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			s = line
			break
		}
	}
	s = strings.TrimSpace(Redact(s))
	if utf8.RuneCountInString(s) > briefLimit {
		s = string([]rune(s)[:briefLimit]) + "…"
	}
	return s
}
