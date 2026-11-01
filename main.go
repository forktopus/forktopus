package main

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/forktopus/forktopus/internal/app"
	"github.com/forktopus/forktopus/internal/applog"
	"github.com/forktopus/forktopus/internal/desktop"
	"github.com/forktopus/forktopus/internal/repocache"
	"github.com/forktopus/forktopus/internal/runner"
	"github.com/forktopus/forktopus/internal/settings"
)

// Собранный фронтенд (frontend/dist) встраивается в бинарник.
//
//go:embed all:frontend/dist
var assets embed.FS

// setupLog направляет лог (и стандартный log) в файл в каталоге кэша;
// если файл недоступен — в stderr (path тогда пустой).
func setupLog() (logger *slog.Logger, path string, recent *applog.Recent) {
	path, err := applog.DefaultPath()
	if err == nil {
		var f *applog.File
		if f, err = applog.Open(path, applog.MaxSize); err == nil {
			logger, recent = applog.New(f)
		}
	}
	if logger == nil {
		path = ""
		logger, recent = applog.New(os.Stderr)
		logger.Warn("лог в файл отключён", "error", err)
	}
	slog.SetDefault(logger)
	logger.Info("запуск", "os", runtime.GOOS, "arch", runtime.GOARCH)
	return logger, path, recent
}

func main() {
	logger, logPath, recent := setupLog()
	configPath, err := settings.DefaultPath()
	if err != nil {
		log.Fatalf("не удалось определить каталог настроек: %v", err)
	}
	cachePath, err := repocache.DefaultPath()
	if err != nil {
		log.Printf("кэш репозиториев отключён: %v", err)
	}
	service, err := app.NewAppService(settings.NewStore(configPath), runner.Logged(runner.Exec{}, logger), cachePath)
	if err != nil {
		// Приложение продолжает работу со значениями по умолчанию.
		log.Printf("настройки: %v", err)
	}
	app.AttachLog(service, logPath, recent)

	var window *application.WebviewWindow
	wailsApp := application.New(application.Options{
		Name:        "forktopus",
		Description: "Все ваши Git-репозитории на одном экране",
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// Один экземпляр на файл настроек: повторный запуск (ярлык, автозапуск)
		// показывает уже открытое окно, а не второй процесс с тем же кэшем.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: instanceID(configPath),
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if window != nil {
					window.Show()
					window.Focus()
				}
			},
		},
	})

	window = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Forktopus",
		Width:  1000,
		Height: 618,
		// На Linux/X11 (GTK4, mutter) default size иногда не применяется и окно
		// открывается 1×1; минимальный размер это предотвращает.
		MinWidth:  1000,
		MinHeight: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
		// Сохранённый масштаб — сразу, без мигания при старте; дальше его меняет фронтенд.
		Zoom: service.GetUI().ZoomLevel(),
	})

	// SIGINT/SIGTERM (выход из сессии, kill): команды отменяются сразу, а не
	// после остановки сервера, и приложение завершается штатно — иначе git и
	// ssh в своей сессии пережили бы его (11.4). Закрытие окна идёт через
	// ServiceShutdown без этого.
	quit := desktop.SetupTray(wailsApp, window, service)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signals
		slog.Info("получен сигнал завершения", "signal", sig.String())
		_ = service.ServiceShutdown()
		quit()
	}()

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// instanceID — имя экземпляра для SingleInstance: своё у каждого файла
// настроек (запуск с другим XDG_CONFIG_HOME — отдельное приложение).
func instanceID(configPath string) string {
	sum := sha256.Sum256([]byte(configPath))
	return fmt.Sprintf("io.github.forktopus.c%x", sum[:6])
}
