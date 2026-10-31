// Package gitlocal читает локальное состояние Git-репозиториев командами git
// (без разбора .git вручную). Сетевых операций здесь нет.
package gitlocal

import (
	"context"
	"slices"
	"strings"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// Info — расположение рабочей копии.
type Info struct {
	TopLevel string // корень рабочей копии
	GitDir   string // абсолютный путь к git dir (у worktree и сабмодуля — вне TopLevel)
	// Superproject — корень родительского репозитория, если это сабмодуль.
	Superproject string
}

func git(ctx context.Context, r runner.Runner, dir string, args ...string) (string, error) {
	res, err := r.Run(ctx, runner.Command{Name: "git", Args: args, Dir: dir})
	if err != nil {
		return "", err
	}
	if !res.OK() {
		return "", &model.Error{Code: model.CodeOther, Message: runner.RedactURLs(runner.TrimOutput(res.Stderr)), ExitCode: res.ExitCode}
	}
	return string(res.Stdout), nil
}

// Inspect подтверждает, что dir — рабочая копия Git, и возвращает её
// расположение. Bare-репозиторий — ошибка UNSUPPORTED.
func Inspect(ctx context.Context, r runner.Runner, dir string) (Info, error) {
	// --show-superproject-working-tree последним: для обычного репозитория он
	// ничего не печатает.
	out, err := git(ctx, r, dir, "rev-parse", "--is-bare-repository", "--show-toplevel", "--absolute-git-dir", "--show-superproject-working-tree")
	if err != nil {
		return Info{}, err
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > 0 && lines[0] == "true" {
		return Info{}, model.Errorf(model.CodeUnsupported, "bare-репозиторий: %s", dir)
	}
	if len(lines) < 3 {
		return Info{}, model.Errorf(model.CodeOther, "неожиданный вывод git rev-parse в %s", dir)
	}
	info := Info{TopLevel: lines[1], GitDir: lines[2]}
	if len(lines) > 3 {
		info.Superproject = lines[3]
	}
	return info, nil
}

// Remotes возвращает remote репозитория с разобранными адресами.
// overrides — тип провайдера для self-hosted хостов из настроек.
func Remotes(ctx context.Context, r runner.Runner, dir string, overrides map[string]model.Provider) ([]model.RemoteInfo, error) {
	out, err := git(ctx, r, dir, "remote", "-v")
	if err != nil {
		return nil, err
	}
	return parseRemotes(out, overrides), nil
}

// parseRemotes разбирает вывод `git remote -v`: «имя\tадрес (fetch|push)».
// Адрес может содержать пробелы (локальный путь), поэтому суффикс снимается справа.
func parseRemotes(out string, overrides map[string]model.Provider) []model.RemoteInfo {
	remotes := []model.RemoteInfo{}
	index := map[string]int{}
	for _, line := range strings.Split(out, "\n") {
		name, rest, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		var kind string
		switch {
		case strings.HasSuffix(rest, " (fetch)"):
			kind, rest = "fetch", strings.TrimSuffix(rest, " (fetch)")
		case strings.HasSuffix(rest, " (push)"):
			kind, rest = "push", strings.TrimSuffix(rest, " (push)")
		default:
			continue
		}
		addr := ParseRemoteURL(rest)
		i, seen := index[name]
		if !seen {
			i = len(remotes)
			index[name] = i
			remotes = append(remotes, model.RemoteInfo{Name: name})
		}
		rm := &remotes[i]
		if kind == "fetch" {
			rm.FetchURL = addr.URL
			rm.Host = addr.Host
			rm.Path = addr.Path
			rm.Provider = DetectProvider(addr.Host, overrides)
		} else if addr.URL != rm.FetchURL {
			rm.PushURL = addr.URL
		}
	}
	// push может прийти раньше fetch только теоретически; PushURL, совпавший с fetch, не нужен
	for i := range remotes {
		if remotes[i].PushURL == remotes[i].FetchURL {
			remotes[i].PushURL = ""
		}
	}
	return remotes
}

// SelectRemote выбирает remote репозитория: явный выбор пользователя (если
// такой remote есть), иначе origin, иначе единственный. При неоднозначности —
// пустая строка: пользователь выбирает сам.
func SelectRemote(remotes []model.RemoteInfo, preferred string) string {
	has := func(name string) bool {
		return slices.ContainsFunc(remotes, func(r model.RemoteInfo) bool { return r.Name == name })
	}
	switch {
	case preferred != "" && has(preferred):
		return preferred
	case has("origin"):
		return "origin"
	case len(remotes) == 1:
		return remotes[0].Name
	}
	return ""
}
