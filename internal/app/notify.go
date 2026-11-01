package app

import (
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/notify"
)

// EventReveal — показать репозиторий в окне (клик по уведомлению); данные — id.
const EventReveal = "repo:reveal"

func init() {
	application.RegisterEvent[string](EventReveal)
}

// notifyDelay — события, пришедшие в пределах этого времени, уходят одной
// пачкой: больше notify.MaxSingle — одно сводное уведомление.
const notifyDelay = 2 * time.Second

// observeLocked сравнивает свежее состояние репозитория с прежним и ставит
// в очередь уведомления включённых в настройках видов; вызывается под s.mu.
func (s *AppService) observeLocked(repo model.Repository) {
	if !s.autoCheckLocked(repo.Path) {
		// Включат снова — первое чтение станет новой точкой отсчёта.
		s.tracker.Forget(repo.ID)
		return
	}
	events := s.tracker.Observe(repo)
	if len(events) == 0 {
		return
	}
	on := s.cfg.UI.Notifications()
	enabled := map[notify.Kind]bool{notify.KindCI: on.CI, notify.KindBehind: on.Behind, notify.KindPR: on.PR}
	kept := events[:0]
	for _, e := range events {
		if enabled[e.Kind] {
			kept = append(kept, e)
		}
	}
	s.batcher.Add(kept)
}

// Reveal просит UI показать репозиторий: выбрать его и открыть детали.
func (s *AppService) Reveal(id string) {
	emit(EventReveal, id)
}
