package app

import (
	"path/filepath"
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

// GetSkipPaths возвращает пути каталогов, в которые сканер не заходит (D-067).
func (s *AppService) GetSkipPaths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.cfg.ScanSkipPaths)
}

// SetSkipPaths сохраняет пути каталогов, в которые сканер не заходит (D-067),
// и сразу убирает из снимка проблемы и репозитории внутри них (D-071) —
// предупреждение и исключённый репозиторий пропадают без пересканирования. Вернувшийся путь проверится со следующего
// сканирования. Возвращает оставшиеся проблемы сканирования.
func (s *AppService) SetSkipPaths(paths []string) ([]model.ScanIssue, error) {
	var clean []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			return nil, model.Errorf(model.CodeInvalidInput, "нужен абсолютный путь: %q", p)
		}
		if p = filepath.Clean(p); !slices.Contains(clean, p) {
			clean = append(clean, p)
		}
	}
	slices.Sort(clean)

	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	next.ScanSkipPaths = clean
	if err := s.store.Save(next); err != nil {
		return nil, model.AsError(err)
	}
	s.cfg = next
	skipped := func(path string) bool {
		return slices.ContainsFunc(clean, func(p string) bool { return within(path, p) })
	}
	s.snapshot.Issues = slices.DeleteFunc(slices.Clone(s.snapshot.Issues), func(i model.ScanIssue) bool { return skipped(i.Path) })
	s.snapshot.Repositories = slices.DeleteFunc(slices.Clone(s.snapshot.Repositories), func(r model.Repository) bool { return skipped(r.Path) })
	s.saveCacheLocked()
	return slices.Clone(s.snapshot.Issues), nil
}

// within — path совпадает с dir или лежит внутри него.
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
