// Package app — сервис, методы которого вызывает UI через Wails bindings.
// Методы тонкие: проверяют ввод и вызывают ядро из соседних пакетов.
// Диалоги и события ОС (Wails) появляются только в этом пакете.
package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
	"github.com/forktopus/forktopus/internal/settings"
	"github.com/forktopus/forktopus/internal/tools"
)

// AppService — сервис Forktopus для фронтенда.
type AppService struct {
	store  *settings.Store
	runner runner.Runner

	mu  sync.Mutex
	cfg settings.Settings
}

// NewAppService создаёт сервис и читает настройки. Ошибка чтения настроек
// не фатальна: сервис работает со значениями по умолчанию, а запись будет
// отклонена с понятным сообщением (см. settings.Store).
func NewAppService(store *settings.Store, r runner.Runner) (*AppService, error) {
	cfg, err := store.Load()
	return &AppService{store: store, runner: r, cfg: cfg}, err
}

// ListRoots возвращает добавленные корневые папки.
func (s *AppService) ListRoots() []model.Root {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.cfg.Roots)
}

// AddRoot добавляет корневую папку. Повторное добавление той же папки
// (в том числе через симлинк) возвращает уже существующую запись.
func (s *AppService) AddRoot(path string) (model.Root, error) {
	if !filepath.IsAbs(path) {
		return model.Root{}, model.Errorf(model.CodeInvalidInput, "нужен абсолютный путь: %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return model.Root{}, model.Errorf(model.CodeInvalidInput, "папка недоступна: %v", err)
	}
	if !info.IsDir() {
		return model.Root{}, model.Errorf(model.CodeInvalidInput, "это не папка: %s", path)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return model.Root{}, model.Errorf(model.CodeInvalidInput, "не удалось определить путь: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	root := model.Root{ID: model.PathID(canonical), Path: canonical}
	for _, r := range s.cfg.Roots {
		if r.ID == root.ID {
			return r, nil
		}
	}

	next := s.cfg
	next.Roots = append(slices.Clone(s.cfg.Roots), root)
	if err := s.store.Save(next); err != nil {
		return model.Root{}, model.AsError(err)
	}
	s.cfg = next
	return root, nil
}

// RemoveRoot убирает корневую папку из списка. Файлы на диске не затрагиваются.
func (s *AppService) RemoveRoot(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.cfg.Roots, func(r model.Root) bool { return r.ID == id })
	if i < 0 {
		return model.Errorf(model.CodeInvalidInput, "корневая папка %q не найдена", id)
	}

	next := s.cfg
	next.Roots = slices.Delete(slices.Clone(s.cfg.Roots), i, i+1)
	if err := s.store.Save(next); err != nil {
		return model.AsError(err)
	}
	s.cfg = next
	return nil
}

// CheckTools проверяет внешние утилиты. Пока только git; серверные CLI
// добавятся вместе с адаптерами провайдеров.
func (s *AppService) CheckTools(ctx context.Context) []model.ToolDiagnostic {
	return []model.ToolDiagnostic{tools.CheckGit(ctx, s.runner)}
}
