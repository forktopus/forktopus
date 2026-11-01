// Package scan находит Git-репозитории в корневых папках пользователя.
// Ядро не знает о Wails: частичные результаты отдаются через callback.
package scan

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/forktopus/forktopus/internal/model"
)

// DefaultSkipDirs — каталоги, в которые сканер не заходит (D-024): в них
// десятки тысяч файлов и чужие .git зависимостей. Скрытые каталоги обходятся.
var DefaultSkipDirs = []string{"node_modules", "vendor", ".venv", "venv", "__pycache__", "dist", "build", "target", ".cache"}

// walk обходит root и вызывает found для каждого каталога, где есть .git
// (каталог или файл). По симлинкам не ходит (A23), внутрь .git и каталогов из
// skip (по имени) и skipPaths (по пути) не заходит, bare-репозитории
// пропускает (D-026). Ошибка чтения каталога передаётся в issue, обход
// продолжается (A04).
func walk(ctx context.Context, root string, skip, skipPaths map[string]bool, found func(dir string), issue func(path string, err error)) error {
	var visit func(dir string) error
	visit = func(dir string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if skipPaths[dir] {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			issue(dir, err)
			return nil
		}
		if isBare(entries) {
			return nil
		}
		for _, e := range entries {
			if e.Name() == ".git" {
				found(dir)
				break
			}
		}
		for _, e := range entries {
			// DirEntry.IsDir для симлинка на каталог — false: по ссылкам не ходим.
			if !e.IsDir() || e.Name() == ".git" || skip[e.Name()] {
				continue
			}
			if err := visit(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(filepath.Clean(root))
}

// issueReason — причина ошибки чтения каталога для окна проблемных каталогов.
func issueReason(err error) model.IssueReason {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return model.IssueNoAccess
	case errors.Is(err, fs.ErrNotExist):
		return model.IssueNotFound
	}
	return model.IssueOther
}

// isBare — признаки git dir без рабочей копии: HEAD, objects/ и refs/ в одном каталоге.
func isBare(entries []os.DirEntry) bool {
	var head, objects, refs bool
	for _, e := range entries {
		switch e.Name() {
		case "HEAD":
			head = !e.IsDir()
		case "objects":
			objects = e.IsDir()
		case "refs":
			refs = e.IsDir()
		}
	}
	return head && objects && refs
}
