package scan

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// countingRunner считает вызовы Git, чтобы видеть стоимость скана на репозиторий.
type countingRunner struct {
	runner.Runner
	calls atomic.Int64
}

func (c *countingRunner) Run(ctx context.Context, cmd runner.Command) (runner.Result, error) {
	c.calls.Add(1)
	return c.Runner.Run(ctx, cmd)
}

// TestLoad — нагрузочный прогон (11.1) на репозиториях из
// scripts/create-load-repos.sh. Запуск:
//
//	FORKTOPUS_LOAD_ROOT=.local/load-repos go test ./internal/scan -run TestLoad -v
func TestLoad(t *testing.T) {
	root := os.Getenv("FORKTOPUS_LOAD_ROOT")
	if root == "" {
		t.Skip("FORKTOPUS_LOAD_ROOT не задан")
	}
	ctx := context.Background()
	r := &countingRunner{Runner: runner.Exec{}}
	roots := []model.Root{{ID: "load", Path: root}}

	var res model.ScanResult
	for i := range 4 {
		r.calls.Store(0)
		start := time.Now()
		var err error
		res, err = Run(ctx, r, roots, Options{})
		if err != nil {
			t.Fatal(err)
		}
		failed := 0
		for _, repo := range res.Repositories {
			if repo.Error != nil {
				failed++
			}
		}
		t.Logf("скан %d: %d репозиториев за %v, вызовов git %d, ошибок %d, параллельно %d",
			i+1, len(res.Repositories), time.Since(start).Round(time.Millisecond), r.calls.Load(), failed, Concurrency)
	}

	// Обновление статусов всех репозиториев без обхода каталогов.
	r.calls.Store(0)
	start := time.Now()
	jobs := make(chan model.Repository)
	var wg sync.WaitGroup
	for range Concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repo := range jobs {
				Refresh(ctx, r, repo, Options{})
			}
		}()
	}
	for _, repo := range res.Repositories {
		jobs <- repo
	}
	close(jobs)
	wg.Wait()
	t.Logf("обновление статусов: %d репозиториев за %v, вызовов git %d",
		len(res.Repositories), time.Since(start).Round(time.Millisecond), r.calls.Load())
}
