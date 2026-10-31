// Package model содержит общие типы ядра Forktopus.
//
// Соглашение о данных: указатель nil (поле отсутствует в JSON) означает
// «нет достоверных данных», а 0 — «достоверно пусто». UI не должен
// показывать отсутствующее значение как ноль.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"time"
)

// Availability — достоверность сведений из внешнего источника (CLI провайдера).
type Availability string

const (
	AvailabilityOK           Availability = "ok"            // данные получены
	AvailabilityUnavailable  Availability = "unavailable"   // источник недоступен: нет CLI, нет авторизации
	AvailabilityError        Availability = "error"         // источник ответил ошибкой
	AvailabilityNotSupported Availability = "not_supported" // для этого сервера/версии CLI нет поддержки
)

// Provider — тип Git-хостинга.
type Provider string

const (
	ProviderGitHub Provider = "github"
	ProviderGitLab Provider = "gitlab"
	ProviderGitea  Provider = "gitea"
	ProviderOther  Provider = "other"
)

// Root — корневая папка, выбранная пользователем для сканирования.
type Root struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// RemoteInfo — remote репозитория с разобранным адресом сервера.
type RemoteInfo struct {
	Name     string   `json:"name"`
	FetchURL string   `json:"fetchUrl"` // без userinfo
	PushURL  string   `json:"pushUrl,omitempty"`
	Host     string   `json:"host,omitempty"` // host[:port]
	Path     string   `json:"path,omitempty"` // namespace/repo, с base path для self-hosted
	Provider Provider `json:"provider"`
}

// BranchStatus — состояние текущей ветки.
type BranchStatus struct {
	Name     string `json:"name,omitempty"` // пусто при detached HEAD
	Detached bool   `json:"detached"`
	HeadSHA  string `json:"headSha,omitempty"`
	// Unborn — в репозитории ещё нет коммитов: ветка названа, но HEAD никуда не указывает.
	Unborn   bool   `json:"unborn,omitempty"`
	Upstream string `json:"upstream,omitempty"`
	// UpstreamGone — upstream настроен, но его remote-tracking ветки нет (удалена на сервере и подрезана fetch --prune).
	UpstreamGone bool `json:"upstreamGone,omitempty"`
	// Ahead/Behind считаются по локальным remote-tracking refs; nil — нет upstream.
	Ahead  *int `json:"ahead,omitempty"`
	Behind *int `json:"behind,omitempty"`
	// LastFetchAt — время последнего fetch (mtime FETCH_HEAD); nil — неизвестно: fetch не выполнялся
	// или репозиторий только склонирован (clone не пишет FETCH_HEAD).
	LastFetchAt *time.Time `json:"lastFetchAt,omitempty"`
}

// BranchInfo — локальная ветка репозитория.
type BranchInfo struct {
	Name     string     `json:"name"`
	Current  bool       `json:"current"` // checked out в этой рабочей копии
	HeadSHA  string     `json:"headSha"`
	CommitAt *time.Time `json:"commitAt,omitempty"` // дата коммита на вершине ветки
	Upstream string     `json:"upstream,omitempty"`
	// UpstreamGone, Ahead, Behind — как в BranchStatus.
	UpstreamGone bool `json:"upstreamGone,omitempty"`
	Ahead        *int `json:"ahead,omitempty"`
	Behind       *int `json:"behind,omitempty"`
	// WorktreePath — другая рабочая копия, где ветка сейчас checked out; переключиться на неё здесь нельзя (A15).
	WorktreePath string `json:"worktreePath,omitempty"`
}

// WorktreeStatus — локальные изменения рабочей копии.
type WorktreeStatus struct {
	Staged    int `json:"staged"`
	Unstaged  int `json:"unstaged"`
	Untracked int `json:"untracked"`
	Conflicts int `json:"conflicts"`
	// Operation — незавершённая операция Git: merge, rebase, am, cherry-pick, revert; пусто, если нет.
	Operation string `json:"operation,omitempty"`
}

// PRSummary — открытые PR/MR, где текущая ветка — источник.
type PRSummary struct {
	Availability  Availability `json:"availability"`
	Open          *int         `json:"open,omitempty"`
	Draft         *int         `json:"draft,omitempty"`
	URL           string       `json:"url,omitempty"`
	Error         *Error       `json:"error,omitempty"`
	LastSuccessAt *time.Time   `json:"lastSuccessAt,omitempty"`
	LastAttemptAt *time.Time   `json:"lastAttemptAt,omitempty"`
}

// CISummary — CI-запуски на текущей ветке.
type CISummary struct {
	Availability  Availability `json:"availability"`
	Active        *int         `json:"active,omitempty"` // queued / in progress
	LastResult    string       `json:"lastResult,omitempty"`
	URL           string       `json:"url,omitempty"`
	Error         *Error       `json:"error,omitempty"`
	LastSuccessAt *time.Time   `json:"lastSuccessAt,omitempty"`
	LastAttemptAt *time.Time   `json:"lastAttemptAt,omitempty"`
}

// Nesting — почему репозиторий лежит внутри рабочей копии другого.
type Nesting string

const (
	NestingSubmodule Nesting = "submodule" // сабмодуль родителя
	NestingNested    Nesting = "nested"    // самостоятельный клон внутри чужой рабочей копии
)

// Repository — найденная рабочая копия Git.
type Repository struct {
	ID     string `json:"id"`
	RootID string `json:"rootId"`
	Path   string `json:"path"`   // корень рабочей копии (git rev-parse --show-toplevel)
	GitDir string `json:"gitDir"` // может лежать вне Path (worktree, submodule)
	// ParentID — ближайший репозиторий, внутри рабочей копии которого лежит этот; пусто — верхний уровень.
	ParentID string       `json:"parentId,omitempty"`
	Nesting  Nesting      `json:"nesting,omitempty"`
	Remotes  []RemoteInfo `json:"remotes"`
	Remote   string       `json:"remote,omitempty"` // имя выбранного remote; пусто — нет или неоднозначно
	// Branch и Worktree: nil — статус ещё не прочитан (или прочитать не удалось, см. Error).
	Branch   *BranchStatus   `json:"branch,omitempty"`
	Worktree *WorktreeStatus `json:"worktree,omitempty"`
	PR       PRSummary       `json:"pr"`
	CI       CISummary       `json:"ci"`
	// Error — сбой чтения именно этого репозитория; остальные поля могут быть неполными.
	Error *Error `json:"error,omitempty"`
	// UpdatedAt — когда локальный статус прочитан («обновлено …»).
	UpdatedAt time.Time `json:"updatedAt"`
}

// ScanIssue — каталог, который не удалось просмотреть при сканировании корня.
type ScanIssue struct {
	RootID string `json:"rootId"`
	Path   string `json:"path"`
	Error  *Error `json:"error"`
}

// ScanResult — итог сканирования корней.
type ScanResult struct {
	Repositories []Repository `json:"repositories"`
	Issues       []ScanIssue  `json:"issues"`
	ScannedAt    time.Time    `json:"scannedAt"`
}

// RepositorySnapshot — известные репозитории. Stale — список взят из кэша
// прошлого запуска и ещё не подтверждён сканированием.
type RepositorySnapshot struct {
	Repositories []Repository `json:"repositories"`
	Issues       []ScanIssue  `json:"issues"`
	ScannedAt    *time.Time   `json:"scannedAt,omitempty"`
	Stale        bool         `json:"stale"`
}

// Operation — изменяющая операция над репозиторием.
type Operation string

const (
	OperationFetch  Operation = "fetch"
	OperationPull   Operation = "pull" // только fast-forward
	OperationSwitch Operation = "switch"
)

// OpExpect — состояние, которое видел пользователь, запуская операцию (D-032).
// Если ветка или HEAD уже другие, операция отменяется с STATE_CHANGED.
type OpExpect struct {
	Branch  string `json:"branch"` // пусто — detached HEAD
	HeadSHA string `json:"headSha"`
}

// OpRecord — запись журнала операций (для диагностики).
type OpRecord struct {
	RepoID     string    `json:"repoId"`
	Op         Operation `json:"op"`
	Target     string    `json:"target,omitempty"` // remote для fetch, ветка для switch
	At         time.Time `json:"at"`
	DurationMs int64     `json:"durationMs"`
	// Error — nil при успехе; иначе причина: блокировка по предусловиям или сбой Git.
	Error *Error `json:"error,omitempty"`
}

// OpResult — итог операции и свежий статус репозитория.
type OpResult struct {
	OpRecord
	Repo Repository `json:"repo"`
}

// RemoteStatus — серверное состояние ветки репозитория: PR/MR и CI.
type RemoteStatus struct {
	RepoID string `json:"repoId"`
	// Branch — ветка, для которой получены данные; пусто — detached HEAD.
	Branch string `json:"branch"`
	// Provider — тип сервера по резолверу (с учётом логинов CLI); у выбранного
	// remote в списке репозиториев он заменяет определённый сканером.
	Provider Provider  `json:"provider"`
	PR       PRSummary `json:"pr"`
	CI       CISummary `json:"ci"`
}

// ToolDiagnostic — результат проверки внешней утилиты (git, gh, glab, tea).
type ToolDiagnostic struct {
	Name string `json:"name"`
	// Host — сервер, для которого проверена авторизация; пусто — проверена только утилита.
	Host         string       `json:"host,omitempty"`
	Availability Availability `json:"availability"`
	Path         string       `json:"path,omitempty"`
	Version      string       `json:"version,omitempty"`
	Error        *Error       `json:"error,omitempty"`
}

// LogEntry — предупреждение или ошибка из технического лога (уже замаскированные).
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// Diagnostics — сведения для отчёта диагностики. Пути — с «~» вместо
// домашнего каталога, тексты пропущены через маскировку секретов.
type Diagnostics struct {
	OS         string     `json:"os"`
	ConfigPath string     `json:"configPath"`
	LogPath    string     `json:"logPath,omitempty"`
	Errors     []LogEntry `json:"errors"`
	// Report — всё одним текстом для копирования: ОС, пути, утилиты, корни,
	// последние ошибки и операции.
	Report string `json:"report"`
}

// PathID — устойчивый идентификатор для пути файловой системы.
func PathID(path string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	return hex.EncodeToString(sum[:8])
}
