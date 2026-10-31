// Package app — сервис, методы которого вызывает UI через Wails bindings.
// Методы тонкие: проверяют ввод и вызывают ядро из соседних пакетов.
// Диалоги и события ОС (Wails) появляются только в этом пакете.
package app

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/forktopus/forktopus/internal/gitlocal"
	"github.com/forktopus/forktopus/internal/gitops"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/remote"
	"github.com/forktopus/forktopus/internal/repocache"
	"github.com/forktopus/forktopus/internal/runner"
	"github.com/forktopus/forktopus/internal/scan"
	"github.com/forktopus/forktopus/internal/settings"
)

// EventScanRepository — репозиторий, найденный во время сканирования
// (до окончания ScanRoots; parentId в нём ещё не заполнен).
const EventScanRepository = "scan:repository"

func init() {
	application.RegisterEvent[model.Repository](EventScanRepository)
}

// AppService — сервис Forktopus для фронтенда.
type AppService struct {
	store     *settings.Store
	runner    runner.Runner
	cachePath string // пусто — кэш между запусками отключён

	mu       sync.Mutex
	cfg      settings.Settings
	snapshot model.RepositorySnapshot

	scanMu sync.Mutex // одно сканирование за раз
	// locks — одна изменяющая операция на репозиторий; чтение статуса её ждёт.
	locks gitops.Locks
	opLog []model.OpRecord // под mu

	remote *remote.Fetcher
	// remoteByRepo — последнее серверное состояние по репозиторию (под mu);
	// в список попадает, только пока ветка та же.
	remoteByRepo map[string]model.RemoteStatus
}

// NewAppService создаёт сервис, читает настройки и кэш прошлого сканирования.
// Ошибка чтения настроек не фатальна: сервис работает со значениями по
// умолчанию, а запись будет отклонена с понятным сообщением (см. settings.Store).
func NewAppService(store *settings.Store, r runner.Runner, cachePath string) (*AppService, error) {
	cfg, err := store.Load()
	s := &AppService{store: store, runner: r, cachePath: cachePath, cfg: cfg,
		remote: remote.New(r, remote.Options{}), remoteByRepo: map[string]model.RemoteStatus{}}
	s.snapshot = model.RepositorySnapshot{Repositories: []model.Repository{}, Issues: []model.ScanIssue{}, Stale: true}
	if cachePath != "" {
		if cached, ok := repocache.Load(cachePath); ok {
			s.snapshot.Repositories = cached.Repositories
			s.snapshot.Issues = cached.Issues
			s.snapshot.ScannedAt = &cached.ScannedAt
		}
	}
	return s, err
}

// emit отправляет событие во фронтенд; без запущенного приложения (тесты) — ничего.
func emit(name string, data any) {
	if a := application.Get(); a != nil {
		a.Event.Emit(name, data)
	}
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
	s.snapshot.Repositories = slices.DeleteFunc(slices.Clone(s.snapshot.Repositories), func(r model.Repository) bool { return r.RootID == id })
	s.snapshot.Issues = slices.DeleteFunc(slices.Clone(s.snapshot.Issues), func(i model.ScanIssue) bool { return i.RootID == id })
	s.saveCacheLocked()
	return nil
}

// PickFolder открывает системный диалог выбора папки. Пустая строка —
// пользователь закрыл диалог.
func (s *AppService) PickFolder() (string, error) {
	a := application.Get()
	if a == nil {
		return "", model.Errorf(model.CodeUnsupported, "системный диалог доступен только в окне приложения")
	}
	path, err := a.Dialog.OpenFile().
		SetTitle("Выберите корневую папку").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(false).
		ShowHiddenFiles(true).
		PromptForSingleSelection()
	if err != nil {
		return "", model.Errorf(model.CodeOther, "диалог выбора папки: %v", err)
	}
	return path, nil
}

// ListRepositories возвращает известные репозитории. До первого сканирования
// в этом запуске — кэш прошлого запуска с пометкой Stale (D-027).
func (s *AppService) ListRepositories() model.RepositorySnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.snapshot
	snap.Repositories = slices.Clone(snap.Repositories)
	snap.Issues = slices.Clone(snap.Issues)
	return snap
}

// ScanRoots сканирует все корни. Найденные репозитории приходят событиями
// EventScanRepository по мере обнаружения, итог — в результате. Отмена вызова
// из JS прерывает сканирование; прежний список при этом сохраняется.
// Серверные данные (PR/CI) после сканирования запрашиваются в фоне и приходят
// событиями EventRemoteStatus; до тех пор показываются прежние той же ветки.
func (s *AppService) ScanRoots(ctx context.Context) (model.ScanResult, error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	s.mu.Lock()
	roots := slices.Clone(s.cfg.Roots)
	opts := s.scanOptionsLocked()
	opts.OnRepository = func(r model.Repository) {
		s.mu.Lock()
		r = s.withRemoteLocked(r, s.prevLocked(r.ID))
		s.mu.Unlock()
		emit(EventScanRepository, r)
	}
	opts.Lock = s.locks.Lock
	s.mu.Unlock()

	res, err := scan.Run(ctx, s.runner, roots, opts)
	if err != nil {
		return res, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Корень могли убрать, пока шло сканирование.
	known := map[string]bool{}
	for _, r := range s.cfg.Roots {
		known[r.ID] = true
	}
	res.Repositories = slices.DeleteFunc(res.Repositories, func(r model.Repository) bool { return !known[r.RootID] })
	res.Issues = slices.DeleteFunc(res.Issues, func(i model.ScanIssue) bool { return !known[i.RootID] })
	for i, r := range res.Repositories {
		res.Repositories[i] = s.withRemoteLocked(r, s.prevLocked(r.ID))
	}
	at := res.ScannedAt
	s.snapshot = model.RepositorySnapshot{Repositories: res.Repositories, Issues: res.Issues, ScannedAt: &at}
	s.saveCacheLocked()
	go s.refreshRemoteAll()
	return res, nil
}

// prevLocked — прежняя запись репозитория в списке или nil.
func (s *AppService) prevLocked(id string) *model.Repository {
	if i, err := s.findLocked(id); err == nil {
		r := s.snapshot.Repositories[i]
		return &r
	}
	return nil
}

func (s *AppService) scanOptionsLocked() scan.Options {
	return scan.Options{SkipDirs: s.cfg.ScanSkipDirs, Remotes: s.cfg.Remotes, HostProviders: s.cfg.HostProviders}
}

// findLocked возвращает индекс репозитория в снимке или ошибку INVALID_INPUT.
func (s *AppService) findLocked(id string) (int, error) {
	i := slices.IndexFunc(s.snapshot.Repositories, func(r model.Repository) bool { return r.ID == id })
	if i < 0 {
		return -1, model.Errorf(model.CodeInvalidInput, "репозиторий %q не найден", id)
	}
	return i, nil
}

// GetRepo возвращает репозиторий из текущего списка (без обращения к Git).
func (s *AppService) GetRepo(id string) (model.Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.findLocked(id)
	if err != nil {
		return model.Repository{}, err
	}
	return s.snapshot.Repositories[i], nil
}

// GetBranches читает локальные ветки репозитория: upstream, ahead/behind и
// ветки, занятые другими рабочими копиями.
func (s *AppService) GetBranches(ctx context.Context, id string) ([]model.BranchInfo, error) {
	repo, err := s.GetRepo(id)
	if err != nil {
		return nil, err
	}
	branches, err := gitlocal.Branches(ctx, s.runner, repo.Path)
	if err != nil {
		return nil, model.AsError(err)
	}
	return branches, nil
}

// RefreshRepo перечитывает локальный статус одного репозитория и обновляет
// его в списке и кэше. Если над репозиторием идёт операция, ждёт её конца.
func (s *AppService) RefreshRepo(ctx context.Context, id string) (model.Repository, error) {
	if _, err := s.GetRepo(id); err != nil {
		return model.Repository{}, err
	}
	unlock, err := s.locks.Lock(ctx, id)
	if err != nil {
		return model.Repository{}, err
	}
	defer unlock()

	s.mu.Lock()
	i, err := s.findLocked(id)
	if err != nil {
		s.mu.Unlock()
		return model.Repository{}, err
	}
	repo, opts := s.snapshot.Repositories[i], s.scanOptionsLocked()
	s.mu.Unlock()

	repo = scan.Refresh(ctx, s.runner, repo, opts)
	if err := ctx.Err(); err != nil {
		return model.Repository{}, model.Errorf(model.CodeOpCancelled, "обновление отменено")
	}
	return s.storeRepo(repo), nil
}

// SelectRemote запоминает remote, по которому определяется сервер
// репозитория. Пустое имя сбрасывает выбор. В .git/config ничего не пишется.
func (s *AppService) SelectRemote(repoID, remote string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.findLocked(repoID)
	if err != nil {
		return err
	}
	repo := s.snapshot.Repositories[i]
	if remote != "" && !slices.ContainsFunc(repo.Remotes, func(r model.RemoteInfo) bool { return r.Name == remote }) {
		return model.Errorf(model.CodeInvalidInput, "у репозитория нет remote %q", remote)
	}

	next := s.cfg
	next.Remotes = map[string]string{}
	for k, v := range s.cfg.Remotes {
		next.Remotes[k] = v
	}
	if remote == "" {
		delete(next.Remotes, repoID)
	} else {
		next.Remotes[repoID] = remote
	}
	if err := s.store.Save(next); err != nil {
		return model.AsError(err)
	}
	s.cfg = next
	repos := slices.Clone(s.snapshot.Repositories)
	repos[i].Remote = gitlocal.SelectRemote(repo.Remotes, remote)
	// Серверные данные относились к прежнему remote.
	delete(s.remoteByRepo, repoID)
	repos[i].PR, repos[i].CI = model.PRSummary{}, model.CISummary{}
	s.snapshot.Repositories = repos
	s.saveCacheLocked()
	go s.remoteStatus(context.Background(), repoID, false) //nolint:errcheck // итог приходит событием
	return nil
}

// saveCacheLocked пишет кэш; вызывается под s.mu. Ошибка кэша не мешает работе.
func (s *AppService) saveCacheLocked() {
	if s.cachePath == "" || s.snapshot.ScannedAt == nil {
		return
	}
	res := model.ScanResult{Repositories: s.snapshot.Repositories, Issues: s.snapshot.Issues, ScannedAt: *s.snapshot.ScannedAt}
	if err := repocache.Save(s.cachePath, res); err != nil {
		log.Printf("кэш репозиториев: %v", err)
	}
}

// GetUI возвращает сохранённое состояние интерфейса: режим, тему, фильтры.
func (s *AppService) GetUI() settings.UI {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.UI
}

// SaveUI сохраняет состояние интерфейса.
func (s *AppService) SaveUI(ui settings.UI) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	next.UI = ui
	if err := s.store.Save(next); err != nil {
		return model.AsError(err)
	}
	s.cfg = next
	return nil
}
