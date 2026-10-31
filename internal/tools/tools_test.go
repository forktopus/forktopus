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

func TestCheckCLI(t *testing.T) {
	ctx := context.Background()
	for out, want := range map[string]string{
		"gh version 2.103.3 (2026-10-26)\n":               "2.103.3",
		"glab 1.126.0 (4dd0d32d1)\n":                      "1.126.0",
		"Version: \x1b[1m0.16.0\x1b[0m\tgolang: 1.26.6\n": "0.16.0",
	} {
		d := CheckCLI(ctx, fakeRunner{res: runner.Result{Stdout: []byte(out)}}, "x")
		if d.Availability != model.AvailabilityOK || d.Version != want {
			t.Errorf("%q: %+v", out, d)
		}
	}
	missing := CheckCLI(ctx, fakeRunner{err: model.Errorf(model.CodeCLIMissing, "нет")}, "tea")
	if missing.Availability != model.AvailabilityUnavailable || missing.Error.Code != model.CodeCLIMissing || missing.Name != "tea" {
		t.Errorf("нет tea: %+v", missing)
	}
}
