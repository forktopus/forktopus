package app

import (
	"slices"

	"github.com/forktopus/forktopus/internal/model"
)

// GetNoAutoCheck — пути репозиториев без автоматической проверки (D-064).
func (s *AppService) GetNoAutoCheck() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.cfg.NoAutoCheck)
}

// SetAutoCheck включает или выключает автоматическую проверку репозитория:
// выключенный фон не перечитывает (status, fetch, PR/CI), уведомлений по нему
// нет. Возвращает новый список путей без проверки.
func (s *AppService) SetAutoCheck(path string, on bool) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := slices.DeleteFunc(slices.Clone(s.cfg.NoAutoCheck), func(p string) bool { return p == path })
	if !on {
		list = append(list, path)
		slices.Sort(list)
	}
	if len(list) == 0 {
		list = nil
	}
	next := s.cfg
	next.NoAutoCheck = list
	if err := s.store.Save(next); err != nil {
		return nil, model.AsError(err)
	}
	s.cfg = next
	if !on {
		s.tracker.Forget(model.PathID(path))
	}
	return slices.Clone(list), nil
}

// autoCheckLocked — проверяется ли репозиторий автоматически; под s.mu.
func (s *AppService) autoCheckLocked(path string) bool {
	return !slices.Contains(s.cfg.NoAutoCheck, path)
}
