package tools

import (
	"context"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

type fakeRunner struct {
	res runner.Result
	err error
}

func (f fakeRunner) Run(context.Context, runner.Command) (runner.Result, error) {
	return f.res, f.err
}

func TestCheckGit(t *testing.T) {
	ctx := context.Background()

	ok := CheckGit(ctx, fakeRunner{res: runner.Result{Stdout: []byte("git version 2.43.0\n")}})
	if ok.Availability != model.AvailabilityOK || ok.Version != "2.43.0" || ok.Error != nil {
		t.Errorf("git есть: %+v", ok)
	}

	missing := CheckGit(ctx, fakeRunner{err: model.Errorf(model.CodeCLIMissing, "git не найден")})
	if missing.Availability != model.AvailabilityUnavailable || model.CodeOf(missing.Error) != model.CodeCLIMissing {
		t.Errorf("git нет: %+v", missing)
	}

	failed := CheckGit(ctx, fakeRunner{res: runner.Result{ExitCode: 128, Stderr: []byte("fatal\n")}})
	if failed.Availability != model.AvailabilityError || failed.Error.ExitCode != 128 {
		t.Errorf("git упал: %+v", failed)
	}
}

func TestCheckGitReal(t *testing.T) {
	diag := CheckGit(context.Background(), runner.Exec{})
	if diag.Availability != model.AvailabilityOK || diag.Version == "" {
		t.Fatalf("реальный git: %+v", diag)
	}
}
