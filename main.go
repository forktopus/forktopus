package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/forktopus/forktopus/internal/app"
	"github.com/forktopus/forktopus/internal/repocache"
	"github.com/forktopus/forktopus/internal/runner"
	"github.com/forktopus/forktopus/internal/settings"
)

// Собранный фронтенд (frontend/dist) встраивается в бинарник.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	configPath, err := settings.DefaultPath()
	if err != nil {
		log.Fatalf("не удалось определить каталог настроек: %v", err)
	}
	cachePath, err := repocache.DefaultPath()
	if err != nil {
		log.Printf("кэш репозиториев отключён: %v", err)
	}
	service, err := app.NewAppService(settings.NewStore(configPath), runner.Exec{}, cachePath)
	if err != nil {
		// Приложение продолжает работу со значениями по умолчанию.
		log.Printf("настройки: %v", err)
	}

	wailsApp := application.New(application.Options{
		Name:        "forktopus",
		Description: "Обзор состояния множества Git-репозиториев",
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
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
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
