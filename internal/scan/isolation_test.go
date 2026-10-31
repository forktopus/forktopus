package scan

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/forktopus/forktopus/internal/gittest"
	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// breakingRunner паникует на любой команде в одном каталоге — как сломанный
// разбор вывода или ошибка в коде чтения конкретного репозитория.
type breakingRunner struct {
	runner.Runner
	broken string
}

func (b breakingRunner) Run(ctx context.Context, c runner.Command) (runner.Result, error) {
	if c.Dir == b.broken {
		panic("сломанный разбор вывода")
	}
	return b.Runner.Run(ctx, c)
}

// Паника на одном репозитории — его ошибка; скан и остальные репозитории целы (11.4).
func TestScanIsolatesPanics(t *testing.T) {
	root := tempRoot(t)
	good := gittest.Init(t, mkdir(t, root, "good"))
	bad := gittest.Init(t, mkdir(t, root, "bad"))
	r := breakingRunner{Runner: runner.Exec{}, broken: bad}

	res, err := Run(context.Background(), r, []model.Root{{ID: "r", Path: root}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(t, root, res); len(got) != 2 {
		t.Fatalf("найдено %v", got)
	}
	if g := byPath(res, good); g.Error != nil || g.Branch == nil {
		t.Fatalf("исправный репозиторий пострадал: %+v", g)
	}
	b := byPath(res, filepath.Clean(bad))
	if b.ID != model.PathID(bad) || model.CodeOf(b.Error) != model.CodeOther {
		t.Fatalf("сломанный репозиторий: %+v", b)
	}

	// Обновление статуса — так же.
	repo := byPath(res, good)
	repo.Path = bad
	if got := Refresh(context.Background(), r, repo, Options{}); model.CodeOf(got.Error) != model.CodeOther || got.ID != repo.ID {
		t.Fatalf("Refresh: %+v", got)
	}
}
