package desktop

import (
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/forktopus/forktopus/internal/app"
	"github.com/forktopus/forktopus/internal/notify"
)

// SetupNotifications показывает уведомления сервиса системными (D-063).
// Клик по уведомлению показывает окно и открывает репозиторий; по сводке —
// просто показывает окно.
func SetupNotifications(ns *notifications.NotificationService, w *application.WebviewWindow, service *app.AppService) {
	var seq atomic.Uint64
	service.Notifier = func(msgs []notify.Message) {
		for _, m := range msgs {
			opts := notifications.NotificationOptions{ID: fmt.Sprintf("forktopus-%d", seq.Add(1)), Title: m.Title, Body: m.Body}
			if m.RepoID != "" {
				opts.Data = map[string]any{"repoId": m.RepoID}
			}
			if err := ns.SendNotification(opts); err != nil {
				slog.Warn("уведомление не показано", "error", err)
			}
		}
	}
	ns.OnNotificationResponse(func(r notifications.NotificationResult) {
		if r.Error != nil {
			slog.Warn("ответ на уведомление", "error", r.Error)
			return
		}
		w.Show()
		w.Focus()
		if id, _ := r.Response.UserInfo["repoId"].(string); id != "" {
			service.Reveal(id)
		}
	})
}
