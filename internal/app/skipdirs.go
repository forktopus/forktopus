package app

import (
	"slices"
	"strings"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/scan"
)

// GetSkipDirs возвращает имена каталогов, в которые сканер не заходит, и
// значения по умолчанию.
func (s *AppService) GetSkipDirs() model.SkipDirs {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.skipDirsLocked()
}

// SetSkipDirs сохраняет список имён каталогов, в которые сканер не заходит
// (D-024). Пустой список или совпадающий с умолчанием сбрасывает настройку.
// Новый список действует со следующего сканирования.
func (s *AppService) SetSkipDirs(dirs []string) (model.SkipDirs, error) {
	clean, err := cleanSkipDirs(dirs)
	if err != nil {
		return model.SkipDirs{}, err
	}
	if sameSet(clean, scan.DefaultSkipDirs) {
		clean = nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	next.ScanSkipDirs = clean
	if err := s.store.Save(next); err != nil {
		return model.SkipDirs{}, model.AsError(err)
	}
	s.cfg = next
	return s.skipDirsLocked(), nil
}

func (s *AppService) skipDirsLocked() model.SkipDirs {
	dirs := s.cfg.ScanSkipDirs
	if len(dirs) == 0 {
		dirs = scan.DefaultSkipDirs
	}
	return model.SkipDirs{Dirs: slices.Clone(dirs), Defaults: slices.Clone(scan.DefaultSkipDirs)}
}

// cleanSkipDirs убирает пробелы, пустые строки и повторы; сканер сравнивает
// имя каталога целиком, поэтому путь или «..» — ошибка ввода.
func cleanSkipDirs(dirs []string) ([]string, error) {
	var out []string
	for _, d := range dirs {
		d = strings.TrimSpace(d)
		if d == "" || slices.Contains(out, d) {
			continue
		}
		if d == "." || d == ".." || strings.ContainsAny(d, `/\`) {
			return nil, model.Errorf(model.CodeInvalidInput, "нужно имя каталога, а не путь: %q", d)
		}
		out = append(out, d)
	}
	return out, nil
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
