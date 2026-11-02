package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/forktopus/forktopus/internal/buildinfo"
	"github.com/forktopus/forktopus/internal/fsutil"
	"github.com/forktopus/forktopus/internal/launch"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/updates"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const EventAppUpdate = "app:update"

func init() { application.RegisterEvent[model.AppUpdate](EventAppUpdate) }

// updater создаётся лениво, без сетевых запросов при старте приложения.
func (s *AppService) updater() *updates.Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.updates == nil {
		format, reason := buildinfo.Distribution()
		dir, err := fsutil.AppDir(os.UserCacheDir)
		if err == nil {
			dir = filepath.Join(dir, "updates")
		} else {
			dir = ""
		}
		s.updates = updates.New(s.life, updates.Options{Version: buildinfo.Version(), Directory: dir, Format: format, Reason: reason, Source: updates.NewGitHubSource(), OnChange: func(st model.AppUpdate) { emit(EventAppUpdate, st) }})
	}
	return s.updates
}

// GetAppUpdate возвращает состояние, включая прогресс после открытия настроек.
func (s *AppService) GetAppUpdate() model.AppUpdate { return s.updater().State() }

// CheckAppUpdate проверяет пакет встроенного типа; UI не выбирает формат или источник.
func (s *AppService) CheckAppUpdate() model.AppUpdate { return s.updater().Check() }

// DownloadAppUpdate запускает скачивание снимка выбранного релиза.
func (s *AppService) DownloadAppUpdate() model.AppUpdate { return s.updater().Download() }

// CancelAppUpdate отменяет активное скачивание и удаляет только его .part.
func (s *AppService) CancelAppUpdate() model.AppUpdate { return s.updater().Cancel() }

// OpenAppUpdateFolder проверяет файл повторно и открывает папку без запуска пакета.
func (s *AppService) OpenAppUpdateFolder(ctx context.Context) error {
	ctx, done := s.bind(ctx)
	defer done()
	dir, err := s.updater().ReadyFolder(ctx)
	if err != nil {
		return model.AsError(err)
	}
	if err := launch.Folder(dir); err != nil {
		return model.AsError(err)
	}
	return nil
}
