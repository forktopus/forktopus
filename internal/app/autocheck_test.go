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

func TestNoAutoCheck(t *testing.T) { // 18.5
	repo, bare := gittest.Upstream(t)
	s, rec, id := opService(t, repo)
	got := make(chan []notify.Message, 4)
	s.batcher = notify.NewBatcher(10*time.Millisecond, func(m []notify.Message) { got <- m })
	ui := s.GetUI()
	ui.AutoFetch = true
	if err := s.SaveUI(ui); err != nil {
		t.Fatal(err)
	}
	r, _ := s.GetRepo(id)

	list, err := s.SetAutoCheck(r.Path, false)
	if err != nil || len(list) != 1 || list[0] != r.Path {
		t.Fatalf("список без проверки: %v, %v", list, err)
	}
	if cfg, _ := settings.NewStore(s.store.Path()).Load(); len(cfg.NoAutoCheck) != 1 {
		t.Fatalf("в настройках: %v", cfg.NoAutoCheck)
	}

	// Фон репозиторий не трогает: ни fetch, ни status, уведомлений нет.
	gittest.PushFromOtherClone(t, bare, "a.txt", "a\n", "remote a")
	rec.reset()
	s.backgroundPass(context.Background())
	if rec.ran("fetch") || rec.ran("status") {
		t.Fatal("фоновая проверка тронула репозиторий без автопроверки")
	}
	select {
	case m := <-got:
		t.Fatalf("уведомление по репозиторию без автопроверки: %+v", m)
	case <-time.After(300 * time.Millisecond):
	}

	// Включили снова: первое чтение — точка отсчёта, без уведомления о том,
	// что накопилось за время без проверки; следующее отставание — с уведомлением.
	if list, err := s.SetAutoCheck(r.Path, true); err != nil || len(list) != 0 {
		t.Fatalf("после включения: %v, %v", list, err)
	}
	s.backgroundPass(context.Background())
	if !rec.ran("fetch") {
		t.Fatal("после включения фон не сделал fetch")
	}
	select {
	case m := <-got:
		t.Fatalf("уведомление о накопленном за время без проверки: %+v", m)
	case <-time.After(300 * time.Millisecond):
	}
	gittest.PushFromOtherClone(t, bare, "b.txt", "b\n", "remote b")
	s.backgroundPass(context.Background())
	select {
	case m := <-got:
		if len(m) != 1 || !strings.Contains(m[0].Body, "на 2 коммита") {
			t.Fatalf("уведомление: %+v", m)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("нет уведомления после включения")
	}
}
