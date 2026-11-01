// Package providers получает PR/MR и CI текущей ветки через CLI хостингов
// (gh, glab, tea) и выбирает, к какому проекту на каком сервере относится remote.
// Находки пилота по форматам CLI — docs/providers.md.
package providers

import (
	"context"

	"github.com/forktopus/forktopus/internal/model"
)

// Target — проект на сервере, к которому относится remote.
type Target struct {
	Provider model.Provider
	// Host — host[:port] из адреса remote.
	Host string
	// BaseURL — адрес инстанса из логина CLI (`https://host[:port][/base]`);
	// пусто — логин на этот хост не найден.
	BaseURL string
	// Project — путь проекта без base path: `owner/repo`, у GitLab — `group/sub/repo`.
	Project string
	// Login — имя логина CLI, которое передаётся явно (`tea -l`); пусто — CLI выбирает сам.
	Login string
}

// Query — запрос серверного состояния ветки.
type Query struct {
	// Dir — путь рабочей копии: tea работает только с cwd внутри git-репозитория.
	Dir    string
	Target Target
	// Branch — текущая локальная ветка; PR/MR отбираются, где она — источник.
	Branch string
	// SHA — текущий коммит при detached HEAD (Branch пуст): PR/MR, содержащие
	// его, и CI этого коммита.
	SHA string
}

// noRef — ни ветки, ни коммита: искать нечего.
func (q Query) noRef() bool { return q.Branch == "" && q.SHA == "" }

// RemoteProvider — адаптер CLI одного провайдера. Методы не возвращают
// ошибку отдельно: недоступность, отсутствие CLI и авторизации передаются
// через Availability и Error в результате, ноль — только при успешном ответе CLI.
type RemoteProvider interface {
	Provider() model.Provider
	// Diagnose проверяет CLI и авторизацию на сервере цели. dir — рабочая копия:
	// tea без git-репозитория в cwd не работает.
	Diagnose(ctx context.Context, dir string, t Target) model.ToolDiagnostic
	// PullRequests — открытые PR/MR, где ветка запроса — источник (при
	// detached HEAD — содержащие коммит запроса).
	PullRequests(ctx context.Context, q Query) model.PRSummary
	// CIRuns — CI-запуски ветки запроса (при detached HEAD — коммита).
	CIRuns(ctx context.Context, q Query) model.CISummary
}

var (
	_ RemoteProvider = GitHub{}
	_ RemoteProvider = GitLab{}
	_ RemoteProvider = Gitea{}
)
