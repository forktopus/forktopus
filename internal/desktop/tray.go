// Package desktop — интеграция с рабочим столом (D-063): значок в трее,
// системные уведомления, автозапуск. Окно и приложение Wails создаёт main.
package desktop

import (
	_ "embed"
	"log/slog"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/forktopus/forktopus/internal/app"
)

// Значок трея (64×64, scripts/make-icon.sh).
//
//go:embed trayicon.png
var trayIcon []byte

// SetupTray добавляет значок в трей: меню, сводка в подсказке, крестик окна
// прячет его в трей (D-063). Если трея в системе нет или в настройках выбран
// выход, крестик закрывает приложение как обычно. Возвращает выход из
// приложения, который не перехватывается крестиком.
func SetupTray(a *application.App, w *application.WebviewWindow, service *app.AppService) (quit func()) {
	var quitting atomic.Bool
	quit = func() {
		quitting.Store(true)
		a.Quit()
	}
	if !trayAvailable() {
		slog.Info("трея в системе нет: крестик закрывает приложение")
		return quit
	}

	show := func() {
		w.Show()
		w.Focus()
	}
	// Сводка — первым неактивным пунктом меню: подсказку значка Wails на Linux
	// не меняет, а в GNOME её и не видно. В Windows и macOS — ещё и подсказка.
	newMenu := func(summary string) *application.Menu {
		menu := application.NewMenu()
		if summary != "" {
			menu.Add(summary).SetEnabled(false)
			menu.AddSeparator()
		}
		menu.Add("Показать Forktopus").OnClick(func(*application.Context) { show() })
		menu.Add("Проверить сейчас").OnClick(func(*application.Context) { service.CheckNow() })
		menu.AddSeparator()
		menu.Add("Выйти").OnClick(func(*application.Context) { quit() })
		return menu
	}

	tray := a.SystemTray.New()
	tray.SetIcon(trayIcon).SetMenu(newMenu("")).OnClick(show)
	tray.SetTooltip("Forktopus")
	service.TraySummary = func(text string) {
		tray.SetMenu(newMenu(text))
		tray.SetTooltip("Forktopus · " + text)
	}

	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if quitting.Load() || service.GetUI().QuitOnClose {
			return
		}
		e.Cancel()
		w.Hide()
	})
	return quit
}
