//go:build !windows

package app

import (
	"bufio"
	"os"
	"runtime"
	"strings"
)

// osName — название ОС (на Linux — из /etc/os-release) и архитектура.
func osName() string {
	name := runtime.GOOS
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME="); ok {
				name = strings.Trim(v, `"`)
				break
			}
		}
	}
	return name + " (" + runtime.GOOS + "/" + runtime.GOARCH + ")"
}
