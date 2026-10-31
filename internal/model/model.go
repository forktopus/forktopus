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
	Upstream string `json:"upstream,omitempty"`
	// Ahead/Behind считаются по локальным remote-tracking refs; nil — нет upstream.
	Ahead  *int `json:"ahead,omitempty"`
	Behind *int `json:"behind,omitempty"`
	// LastFetchAt — время последнего fetch; nil — fetch не выполнялся.
	LastFetchAt *time.Time `json:"lastFetchAt,omitempty"`
}

// WorktreeStatus — локальные изменения рабочей копии.
type WorktreeStatus struct {
	Staged    int `json:"staged"`
	Unstaged  int `json:"unstaged"`
	Untracked int `json:"untracked"`
	Conflicts int `json:"conflicts"`
	// Operation — незавершённая операция Git: merge, rebase, cherry-pick, revert; пусто, если нет.
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

// Repository — найденная рабочая копия Git.
type Repository struct {
	ID       string         `json:"id"`
	RootID   string         `json:"rootId"`
	Path     string         `json:"path"`   // корень рабочей копии (git rev-parse --show-toplevel)
	GitDir   string         `json:"gitDir"` // может лежать вне Path (worktree, submodule)
	Remotes  []RemoteInfo   `json:"remotes"`
	Remote   string         `json:"remote,omitempty"` // имя выбранного remote; пусто — нет или неоднозначно
	Branch   BranchStatus   `json:"branch"`
	Worktree WorktreeStatus `json:"worktree"`
	PR       PRSummary      `json:"pr"`
	CI       CISummary      `json:"ci"`
	// Error — сбой чтения именно этого репозитория; остальные поля могут быть неполными.
	Error     *Error    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ToolDiagnostic — результат проверки внешней утилиты (git, gh, glab, tea).
type ToolDiagnostic struct {
	Name         string       `json:"name"`
	Availability Availability `json:"availability"`
	Path         string       `json:"path,omitempty"`
	Version      string       `json:"version,omitempty"`
	Error        *Error       `json:"error,omitempty"`
}

// PathID — устойчивый идентификатор для пути файловой системы.
func PathID(path string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	return hex.EncodeToString(sum[:8])
}
