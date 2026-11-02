// Package build хранит в бинарнике версию из того же config.yml, что и упаковка.
package build

import (
	_ "embed"
	"regexp"
)

// Встраивание выполняется компилятором для каждой ОС и не требует генератора
// или второго номера версии: release bump меняет исходный config.yml.
//
//go:embed config.yml
var config string

// Version возвращает версию продукта, а не версию формата конфигурации Wails.
func Version() string {
	m := regexp.MustCompile(`(?m)^  version: "([^"]+)"`).FindStringSubmatch(config)
	if len(m) != 2 {
		return "unknown"
	}
	return m[1]
}
