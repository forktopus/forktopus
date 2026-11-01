//go:build !windows

package app

import (
	"os"
	"runtime"
	"strings"
)

// osName — название ОС (на Linux — из /etc/os-release) и архитектура.
func osName() string {
	name := runtime.GOOS
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for line := range strings.Lines(string(data)) {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "PRETTY_NAME="); ok {
				name = strings.Trim(v, `"`)
				break
			}
		}
	}
	return name + " (" + runtime.GOOS + "/" + runtime.GOARCH + ")"
}
