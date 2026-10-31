// Package tools диагностирует внешние утилиты: есть ли они и какой версии.
// Приложение ничего не устанавливает и не авторизует — только сообщает.
package tools

import (
	"context"
	"strings"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// CheckGit проверяет наличие git и его версию.
func CheckGit(ctx context.Context, r runner.Runner) model.ToolDiagnostic {
	diag := model.ToolDiagnostic{Name: "git"}
	res, err := r.Run(ctx, runner.Command{Name: "git", Args: []string{"--version"}})
	switch {
	case model.CodeOf(err) == model.CodeCLIMissing:
		diag.Availability = model.AvailabilityUnavailable
		diag.Error = model.Errorf(model.CodeCLIMissing, "git не найден в PATH: установите Git и перезапустите Forktopus")
	case err != nil:
		diag.Availability = model.AvailabilityError
		diag.Error = model.AsError(err)
	case !res.OK():
		diag.Availability = model.AvailabilityError
		diag.Error = &model.Error{Code: model.CodeOther, Message: runner.TrimOutput(res.Stderr), ExitCode: res.ExitCode}
	default:
		diag.Availability = model.AvailabilityOK
		diag.Version = strings.TrimPrefix(runner.TrimOutput(res.Stdout), "git version ")
	}
	return diag
}
