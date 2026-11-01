// Package settings хранит пользовательские настройки Forktopus в JSON-файле
// в конфигурационном каталоге ОС. Секреты и токены здесь не хранятся.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/forktopus/forktopus/internal/fsutil"
	"github.com/forktopus/forktopus/internal/model"
)

// FormatVersion — версия формата файла настроек.
const FormatVersion = 1

// Settings — содержимое файла настроек.
type Settings struct {
	Version int          `json:"version"`
	Roots   []model.Root `json:"roots"`
	// Remotes — явно выбранный remote для репозитория: ID репозитория → имя remote.
	// В .git/config ничего не записывается.
	Remotes map[string]string `json:"remotes,omitempty"`
	// HostProviders — тип провайдера для self-hosted хостов: host[:port] или host → провайдер.
	// Главнее известных хостов и логинов CLI (D-042).
	HostProviders map[string]model.Provider `json:"hostProviders,omitempty"`
	// ScanSkipDirs — имена каталогов, в которые сканер не заходит; пусто —
	// scan.DefaultSkipDirs.
	ScanSkipDirs []string `json:"scanSkipDirs,omitempty"`
	// ScanSkipPaths — абсолютные пути каталогов, в которые сканер не заходит
	// (окно проблемных каталогов, D-067).
	ScanSkipPaths []string `json:"scanSkipPaths,omitempty"`
	// NoAutoCheck — пути репозиториев, которые фоновая проверка не трогает и
	// по которым нет уведомлений (D-064).
	NoAutoCheck []string `json:"noAutoCheck,omitempty"`
	UI          UI       `json:"ui"`
}

// UI — состояние интерфейса, которое переживает перезапуск.
type UI struct {
	Theme   string            `json:"theme,omitempty"` // light | dark; пусто — как в системе
	Mode    string            `json:"mode,omitempty"`  // registry | explorer | radar
	Filters map[string]string `json:"filters,omitempty"`
	// FetchPrune — Fetch с --prune: удалять remote-tracking ветки, которых нет на сервере (D-057).
	FetchPrune bool `json:"fetchPrune,omitempty"`
	// Zoom — масштаб интерфейса, 1.0–2.0; 0 — 1.0 (D-061).
	Zoom float64 `json:"zoom,omitempty"`
	// BackgroundMinutes — как часто перечитывать статусы в фоне, минуты; 0 — 5 (D-063).
	BackgroundMinutes int `json:"backgroundMinutes,omitempty"`
	// AutoFetch — fetch перед фоновой проверкой; по умолчанию выключен (D-063).
	AutoFetch bool `json:"autoFetch,omitempty"`
	// QuitOnClose — крестик закрывает приложение, а не прячет окно в трей (D-063).
	QuitOnClose bool `json:"quitOnClose,omitempty"`
	// Notify — какие системные уведомления показывать; nil — все (D-063).
	Notify *Notify `json:"notify,omitempty"`
}

// Notify — виды уведомлений: CI упал или снова проходит, ветка отстала,
// PR/MR открыт, влит или закрыт.
type Notify struct {
	CI     bool `json:"ci"`
	Behind bool `json:"behind"`
	PR     bool `json:"pr"`
}

// Notifications — включённые виды уведомлений; по умолчанию все.
func (u UI) Notifications() Notify {
	if u.Notify == nil {
		return Notify{CI: true, Behind: true, PR: true}
	}
	return *u.Notify
}

// Интервал фоновой проверки, минуты.
const (
	DefaultBackgroundMinutes = 5
	MinBackgroundMinutes     = 1
	MaxBackgroundMinutes     = 60
)

// BackgroundInterval — интервал фоновой проверки в допустимых пределах.
func (u UI) BackgroundInterval() time.Duration {
	m := u.BackgroundMinutes
	if m == 0 {
		m = DefaultBackgroundMinutes
	}
	return time.Duration(min(max(m, MinBackgroundMinutes), MaxBackgroundMinutes)) * time.Minute
}

// Масштаб интерфейса. Ниже 1.0 Wails на Linux не опускается (setZoom поднимает до 1).
const (
	MinZoom = 1.0
	MaxZoom = 2.0
)

// ZoomLevel — масштаб для окна: в допустимых пределах, по умолчанию 1.0.
func (u UI) ZoomLevel() float64 {
	return min(max(u.Zoom, MinZoom), MaxZoom)
}

// Default возвращает настройки первого запуска.
func Default() Settings {
	return Settings{Version: FormatVersion, Roots: []model.Root{}}
}

// DefaultPath — путь к файлу настроек в конфигурационном каталоге ОС.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "forktopus", "config.json"), nil
}

// Store читает и атомарно записывает файл настроек.
type Store struct {
	path string
	mu   sync.Mutex
	// broken — файл не удалось разобрать; запись запрещена, чтобы не затереть
	// данные пользователя, пока он сам не разберётся с файлом.
	broken bool
}

// NewStore создаёт хранилище для файла path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path — путь к файлу настроек.
func (s *Store) Path() string { return s.path }

// Load читает настройки. Отсутствующий файл — не ошибка: возвращаются значения
// по умолчанию. При повреждённом файле возвращаются значения по умолчанию
// вместе с ошибкой, а последующие Save отклоняются.
func (s *Store) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		s.broken = false
		return Default(), nil
	}
	if err != nil {
		return Default(), model.Errorf(model.CodeOther, "не удалось прочитать настройки %s: %v", s.path, err)
	}

	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		s.broken = true
		return Default(), model.Errorf(model.CodeOther, "файл настроек %s повреждён и не будет перезаписан: %v", s.path, err)
	}
	if cfg.Roots == nil {
		cfg.Roots = []model.Root{}
	}
	s.broken = false
	return cfg, nil
}

// Save атомарно записывает настройки: временный файл в том же каталоге,
// затем rename. Каталог создаётся с правами 0700, файл — 0600.
func (s *Store) Save(cfg Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.broken {
		return model.Errorf(model.CodeOther, "файл настроек %s повреждён; исправьте или удалите его вручную", s.path)
	}
	cfg.Version = FormatVersion

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.path, append(data, '\n'), 0o600)
}
