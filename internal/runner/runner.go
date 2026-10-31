// Package runner запускает внешние утилиты (git, gh, glab, tea) без shell.
package runner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/forktopus/forktopus/internal/model"
)

// DefaultTimeout — таймаут команды, если в Command он не задан.
const DefaultTimeout = 30 * time.Second

// Command — один запуск внешней утилиты.
type Command struct {
	Name    string        // имя бинарника, ищется в PATH
	Args    []string      // аргументы передаются как есть, без интерпретации shell
	Dir     string        // рабочий каталог; глобальный cwd процесса не меняется
	Timeout time.Duration // 0 — DefaultTimeout
	Env     []string      // дополнительные переменные KEY=VALUE поверх окружения процесса
}

// Result — итог запуска. Ненулевой код завершения — не ошибка запуска:
// для git он часто содержательный (например, symbolic-ref при detached HEAD).
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Duration time.Duration
}

// OK — команда завершилась с кодом 0.
func (r Result) OK() bool { return r.ExitCode == 0 }

// Runner запускает команды. Интерфейс нужен, чтобы подменять CLI в тестах.
type Runner interface {
	Run(ctx context.Context, cmd Command) (Result, error)
}

// Exec — Runner на базе os/exec.
type Exec struct{}

// nonInteractiveEnv не даёт утилитам ждать ввода от пользователя и делает
// вывод пригодным для разбора.
var nonInteractiveEnv = []string{
	"GIT_TERMINAL_PROMPT=0", // git не спрашивает логин/пароль
	"GIT_OPTIONAL_LOCKS=0",  // git status не берёт index.lock, не мешая пользователю
	"GH_PROMPT_DISABLED=1",
	"NO_COLOR=1",
	"LC_ALL=C", // стабильные англоязычные сообщения; пути в -z выводе не экранируются
}

// Run выполняет команду. Ошибка возвращается, только если процесс не удалось
// запустить или он был прерван: CLI_MISSING, TIMEOUT, OP_CANCELLED.
func (Exec) Run(ctx context.Context, c Command) (Result, error) {
	path, err := exec.LookPath(c.Name)
	if err != nil {
		return Result{}, model.Errorf(model.CodeCLIMissing, "%s не найден в PATH", c.Name)
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = append(append(os.Environ(), nonInteractiveEnv...), c.Env...)
	// Дочерние процессы (например, ssh от git) могут держать pipe после
	// завершения основного; не ждём их бесконечно.
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Duration: time.Since(start)}

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return res, model.Errorf(model.CodeTimeout, "%s: превышено время ожидания %s", c.Name, timeout)
	case errors.Is(ctx.Err(), context.Canceled):
		return res, model.Errorf(model.CodeOpCancelled, "%s: операция отменена", c.Name)
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	if runErr != nil {
		return res, model.Errorf(model.CodeOther, "%s: %s", c.Name, RedactURLs(runErr.Error()))
	}
	return res, nil
}

// TrimOutput возвращает вывод без завершающих переводов строк.
func TrimOutput(b []byte) string {
	return strings.TrimRight(string(b), "\r\n")
}
