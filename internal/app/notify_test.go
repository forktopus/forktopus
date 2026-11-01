package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/gittest"
	"github.com/forktopus/forktopus/internal/notify"
	"github.com/forktopus/forktopus/internal/settings"
)

func TestNotifyBehindAfterBackgroundFetch(t *testing.T) { // 18.3
	repo, bare := gittest.Upstream(t)
	s, _, id := opService(t, repo) // сканирование — точка отсчёта
	got := make(chan []notify.Message, 4)
	s.batcher = notify.NewBatcher(10*time.Millisecond, func(m []notify.Message) { got <- m })
	ui := s.GetUI()
	ui.AutoFetch = true
	if err := s.SaveUI(ui); err != nil {
		t.Fatal(err)
	}

	gittest.PushFromOtherClone(t, bare, "a.txt", "a\n", "remote a")
	s.backgroundPass(context.Background())
	select {
	case m := <-got:
		if len(m) != 1 || m[0].RepoID != id || !strings.Contains(m[0].Body, "отстаёт от upstream на 1 коммит") {
			t.Fatalf("уведомление: %+v", m)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("нет уведомления об отставании")
	}

	// Вид выключен в настройках — уведомления нет.
	ui.Notify = &settings.Notify{CI: true, PR: true}
	if err := s.SaveUI(ui); err != nil {
		t.Fatal(err)
	}
	gittest.PushFromOtherClone(t, bare, "b.txt", "b\n", "remote b")
	s.backgroundPass(context.Background())
	select {
	case m := <-got:
		t.Fatalf("уведомление выключенного вида: %+v", m)
	case <-time.After(300 * time.Millisecond):
	}
}
