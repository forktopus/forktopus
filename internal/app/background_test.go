package app

import (
	"context"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/gittest"
)

func TestBackgroundPassRereadsStatus(t *testing.T) { // 18.1
	repo := gittest.Clean(t)
	s, _, id := opService(t, repo)
	before, _ := s.GetRepo(id)

	gittest.WriteFile(t, repo, "new.txt", "new\n")
	s.backgroundPass(context.Background())

	r, _ := s.GetRepo(id)
	if r.Worktree == nil || r.Worktree.Untracked != 1 {
		t.Fatalf("фоновая проверка не увидела новый файл: %+v", r.Worktree)
	}
	snap := s.ListRepositories()
	if snap.ScannedAt == nil || !snap.ScannedAt.After(before.UpdatedAt) {
		t.Fatalf("время проверки не обновилось: %v", snap.ScannedAt)
	}
}

func TestBackgroundPassSkipsBusyRepo(t *testing.T) { // 18.1
	repo := gittest.Clean(t)
	s, rec, id := opService(t, repo)
	unlock, err := s.locks.TryLock(id)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	gittest.WriteFile(t, repo, "new.txt", "new\n")
	s.backgroundPass(context.Background())
	if rec.ran("status") {
		t.Fatal("репозиторий с идущей операцией перечитан в фоне")
	}
	if r, _ := s.GetRepo(id); r.Worktree.Untracked != 0 {
		t.Fatalf("статус занятого репозитория изменился: %+v", r.Worktree)
	}
}

func TestBackgroundFetchOnlyWhenEnabled(t *testing.T) { // 18.1
	repo, bare := gittest.Upstream(t)
	s, rec, id := opService(t, repo)
	gittest.PushFromOtherClone(t, bare, "remote.txt", "remote\n", "remote commit")

	s.backgroundPass(context.Background())
	if rec.ran("fetch") {
		t.Fatal("fetch без включённой настройки")
	}
	if r, _ := s.GetRepo(id); *r.Branch.Behind != 0 {
		t.Fatalf("без fetch отставание не видно, а получено %d", *r.Branch.Behind)
	}

	ui := s.GetUI()
	ui.AutoFetch = true
	if err := s.SaveUI(ui); err != nil {
		t.Fatal(err)
	}
	s.backgroundPass(context.Background())
	if !rec.ran("fetch") {
		t.Fatal("fetch не выполнен при включённой настройке")
	}
	if r, _ := s.GetRepo(id); *r.Branch.Behind != 1 {
		t.Fatalf("после фонового fetch отставание %d, ожидалось 1", *r.Branch.Behind)
	}
}

func TestBackgroundLoopCheckNowAndShutdown(t *testing.T) { // 18.1
	repo := gittest.Clean(t)
	s, rec, _ := opService(t, repo)
	done := make(chan struct{})
	go func() {
		s.backgroundLoop()
		close(done)
	}()

	// Интервал по умолчанию — 5 минут; проверка идёт только по CheckNow.
	s.CheckNow()
	deadline := time.Now().Add(10 * time.Second)
	for !rec.ran("status") {
		if time.Now().After(deadline) {
			t.Fatal("CheckNow не запустил проверку")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := s.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("фоновая проверка не остановилась при завершении")
	}
}
