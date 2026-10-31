package gitops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

const (
	// NetworkTimeout — fetch и pull ходят на сервер.
	NetworkTimeout = 2 * time.Minute
	// LocalTimeout — switch меняет только рабочую копию.
	LocalTimeout = 30 * time.Second
)

// Fetch обновляет remote-tracking refs выбранного remote. Без prune (D-033):
// явный --no-prune не зависит от fetch.prune в конфиге пользователя.
func Fetch(ctx context.Context, r runner.Runner, dir, remote string) error {
	if remote == "" || strings.HasPrefix(remote, "-") {
		return model.Errorf(model.CodeInvalidInput, "некорректное имя remote %q", remote)
	}
	head, before := fetchHead(ctx, r, dir)
	err := run(ctx, r, dir, NetworkTimeout, "fetch", "--no-prune", "--", remote)
	if err != nil && head != "" {
		restoreFetchHead(head, before)
	}
	return err
}

// fetchHead возвращает путь FETCH_HEAD и его прежнее состояние (nil — файла не было).
// Пусто — путь не определён; тогда восстанавливать нечего.
func fetchHead(ctx context.Context, r runner.Runner, dir string) (string, os.FileInfo) {
	res, err := r.Run(ctx, runner.Command{Name: "git", Args: []string{"rev-parse", "--git-path", "FETCH_HEAD"}, Dir: dir})
	if err != nil || !res.OK() {
		return "", nil
	}
	path := runner.TrimOutput(res.Stdout)
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	fi, _ := os.Stat(path)
	return path, fi
}

// restoreFetchHead: неудачный fetch всё равно создаёт или обрезает FETCH_HEAD,
// а по его mtime показывается время последнего fetch (06.4). Без восстановления
// сбой выглядел бы как свежий fetch.
func restoreFetchHead(path string, before os.FileInfo) {
	after, err := os.Stat(path)
	if err != nil {
		return
	}
	switch {
	case before != nil:
		_ = os.Chtimes(path, before.ModTime(), before.ModTime())
	case after.Size() == 0:
		_ = os.Remove(path) // создан этим fetch и пуст
	}
}

// Pull подтягивает upstream текущей ветки только fast-forward. --no-rebase и
// --no-autostash перекрывают pull.rebase и merge.autoStash из конфига: история
// не переписывается, изменения не прячутся.
func Pull(ctx context.Context, r runner.Runner, dir string) error {
	return run(ctx, r, dir, NetworkTimeout, "pull", "--ff-only", "--no-rebase", "--no-autostash")
}

// Switch переключает рабочую копию на существующую локальную ветку.
// --no-guess не даёт Git создать ветку из одноимённой remote-tracking.
func Switch(ctx context.Context, r runner.Runner, dir, branch string) error {
	if err := ValidateBranchName(ctx, r, dir, branch); err != nil {
		return err
	}
	return run(ctx, r, dir, LocalTimeout, "switch", "--no-guess", "--", branch)
}

// ValidateBranchName отклоняет имена, которые Git прочтёт как опцию или
// не примет как имя ветки.
func ValidateBranchName(ctx context.Context, r runner.Runner, dir, branch string) error {
	invalid := model.Errorf(model.CodeInvalidInput, "некорректное имя ветки %q", branch)
	// «@{-1}» и подобное check-ref-format раскрывает, а не проверяет.
	if branch == "" || strings.HasPrefix(branch, "-") || strings.Contains(branch, "@{") {
		return invalid
	}
	res, err := r.Run(ctx, runner.Command{Name: "git", Args: []string{"check-ref-format", "--branch", branch}, Dir: dir})
	if err != nil {
		return err
	}
	if !res.OK() || runner.TrimOutput(res.Stdout) != branch {
		return invalid
	}
	return nil
}

// HeadReachable — коммит HEAD входит в какую-либо локальную ветку или
// remote-tracking ref, то есть не потеряется после switch (D-034).
func HeadReachable(ctx context.Context, r runner.Runner, dir string) (bool, error) {
	res, err := r.Run(ctx, runner.Command{Name: "git", Args: []string{"for-each-ref", "--contains", "HEAD", "--count=1", "--format=%(refname)", "refs/heads/", "refs/remotes/"}, Dir: dir})
	if err != nil {
		return false, err
	}
	if !res.OK() {
		return false, gitError(res)
	}
	return strings.TrimSpace(string(res.Stdout)) != "", nil
}

func run(ctx context.Context, r runner.Runner, dir string, timeout time.Duration, args ...string) error {
	res, err := r.Run(ctx, runner.Command{Name: "git", Args: args, Dir: dir, Timeout: timeout})
	if err != nil {
		return err
	}
	if !res.OK() {
		return gitError(res)
	}
	return nil
}

// Фрагменты stderr Git (LC_ALL=C) и ssh, по которым видна причина сбоя.
var (
	authMarkers = []string{
		"Authentication failed", "could not read Username", "could not read Password",
		"terminal prompts disabled", "Permission denied (publickey", "Host key verification failed",
		"correct access rights", "Access denied", "HTTP Basic", "returned error: 401", "returned error: 403",
	}
	networkMarkers = []string{
		"Could not resolve host", "Could not resolve hostname", "Connection refused", "Connection timed out",
		"Network is unreachable", "Operation timed out", "Failed to connect", "Connection reset",
		"No route to host", "Temporary failure in name resolution", "SSL", "TLS",
	}
	divergedMarkers  = []string{"Not possible to fast-forward", "not possible to fast-forward"}
	overwriteMarkers = []string{"would be overwritten", "untracked working tree files would be"}
)

// gitError переводит неудачный запуск git в понятную ошибку с кодом.
func gitError(res runner.Result) *model.Error {
	stderr := runner.RedactURLs(runner.TrimOutput(res.Stderr))
	detail := summary(stderr)
	code, message := model.CodeOther, detail
	switch {
	case strings.Contains(stderr, "does not appear to be a git repository"):
		// Нет такого remote или локального пути; дальше git советует проверить права — это не про авторизацию.
	case containsAny(stderr, networkMarkers):
		// Раньше авторизации: при обрыве ssh git тоже советует проверить права доступа.
		code, message = model.CodeNetworkError, "сервер недоступен: "+lineWith(stderr, networkMarkers)
	case containsAny(stderr, authMarkers):
		code, message = model.CodeAuthRequired, "нет доступа к remote: выполните вход в терминале (например, git fetch) и повторите. Git: "+lineWith(stderr, authMarkers)
	case containsAny(stderr, divergedMarkers):
		code, message = model.CodeDiverged, "история разошлась: fast-forward невозможен, объедините ветки в терминале"
	case containsAny(stderr, overwriteMarkers):
		code, message = model.CodeDirtyWorktree, "локальные файлы были бы перезаписаны: "+detail
	}
	if message == "" {
		message = "git завершился с ошибкой"
	}
	return &model.Error{Code: code, Message: message, ExitCode: res.ExitCode}
}

func containsAny(s string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// lineWith — первая строка с одним из фрагментов: в ней конкретная причина
// (например, «ssh: Could not resolve hostname …»), а не общее «fatal: Could not read …».
func lineWith(s string, markers []string) string {
	for _, l := range strings.Split(s, "\n") {
		if containsAny(l, markers) {
			return strings.TrimSpace(l)
		}
	}
	return summary(s)
}

// summary — первая строка «fatal:»/«error:» (после неё git часто пишет
// общие советы), иначе последняя непустая строка.
func summary(s string) string {
	last := ""
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		for _, p := range []string{"fatal:", "error:", "ERROR:"} {
			if strings.HasPrefix(l, p) {
				return l
			}
		}
		last = l
	}
	return last
}
