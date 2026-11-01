//go:build windows

package runner

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

var (
	pathMu sync.Mutex
	pathAt time.Time
)

// refreshPath перечитывает PATH из реестра: процесс получает его один раз при
// запуске, и CLI, установленные, пока Forktopus работает в трее, иначе не
// находятся. Не чаще раза в 10 с; записи процесса, которых нет в реестре,
// остаются в конце.
func refreshPath() {
	pathMu.Lock()
	defer pathMu.Unlock()
	if time.Since(pathAt) < 10*time.Second {
		return
	}
	pathAt = time.Now()
	var fresh []string
	for _, k := range []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`},
		{registry.CURRENT_USER, `Environment`},
	} {
		key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := key.GetStringValue("Path")
		key.Close()
		if err != nil {
			continue
		}
		if x, err := registry.ExpandString(v); err == nil {
			v = x
		}
		fresh = append(fresh, filepath.SplitList(v)...)
	}
	if len(fresh) > 0 {
		os.Setenv("PATH", mergePath(fresh, os.Getenv("PATH"), string(filepath.ListSeparator)))
	}
}
