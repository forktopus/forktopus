package runner

import "regexp"

// userinfoRe находит userinfo в URL со схемой: https://user:token@host.
// scp-форма git@host:org/repo не трогается — там нет секрета.
var userinfoRe = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/@\s]+@`)

// RedactURLs маскирует логины и токены в URL внутри произвольного текста.
// Применять ко всему, что попадает в логи и сообщения для UI.
func RedactURLs(s string) string {
	return userinfoRe.ReplaceAllString(s, "${1}***@")
}
