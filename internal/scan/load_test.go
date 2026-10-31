package scan

import (
	"context"
	"os"
	"slices"
	"strconv"
	"strings"
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

// TestLoad — нагрузочный прогон (11.1, 11.2) на репозиториях из
// scripts/create-load-repos.sh: время скана и обновления статусов всех
// репозиториев при разной параллельности (медиана трёх прогонов). Запуск:
//
//	FORKTOPUS_LOAD_ROOT=.local/load-repos go test ./internal/scan -run TestLoad -v
//
// FORKTOPUS_LOAD_LEVELS — уровни через запятую, по умолчанию текущий Concurrency.
func TestLoad(t *testing.T) {
	root := os.Getenv("FORKTOPUS_LOAD_ROOT")
	if root == "" {
		t.Skip("FORKTOPUS_LOAD_ROOT не задан")
	}
	levels := []int{Concurrency}
	if s := os.Getenv("FORKTOPUS_LOAD_LEVELS"); s != "" {
		levels = nil
		for _, f := range strings.Split(s, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(f))
			if err != nil || n < 1 {
				t.Fatalf("FORKTOPUS_LOAD_LEVELS: %q", f)
			}
			levels = append(levels, n)
		}
	}
	defer func(prev int) { Concurrency = prev }(Concurrency)

	ctx := context.Background()
	r := &countingRunner{Runner: runner.Exec{}}
	roots := []model.Root{{ID: "load", Path: root}}
	// Прогрев кэша ФС, чтобы первый уровень не проигрывал из-за диска.
	res, err := Run(ctx, r, roots, Options{})
	if err != nil {
		t.Fatal(err)
	}

	median := func(f func() time.Duration) time.Duration {
		d := []time.Duration{f(), f(), f()}
		slices.Sort(d)
		return d[1].Round(time.Millisecond)
	}
	for _, n := range levels {
		Concurrency = n
		r.calls.Store(0)
		scanTime := median(func() time.Duration {
			start := time.Now()
			got, err := Run(ctx, r, roots, Options{})
			if err != nil {
				t.Fatal(err)
			}
			for _, repo := range got.Repositories {
				if repo.Error != nil {
					t.Errorf("%s: %v", repo.Path, repo.Error)
				}
			}
			res = got
			return time.Since(start)
		})
		scanCalls := r.calls.Load() / 3
		r.calls.Store(0)
		refreshTime := median(func() time.Duration { return refreshAll(ctx, r, res.Repositories, n) })
		t.Logf("параллельно %2d: скан %d репозиториев %v (git %d), обновление статусов %v (git %d)",
			n, len(res.Repositories), scanTime, scanCalls, refreshTime, r.calls.Load()/3)
	}
}

// refreshAll перечитывает статус всех репозиториев без обхода каталогов.
func refreshAll(ctx context.Context, r runner.Runner, repos []model.Repository, workers int) time.Duration {
	start := time.Now()
	jobs := make(chan model.Repository)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repo := range jobs {
				Refresh(ctx, r, repo, Options{})
			}
		}()
	}
	for _, repo := range repos {
		jobs <- repo
	}
	close(jobs)
	wg.Wait()
	return time.Since(start)
}
