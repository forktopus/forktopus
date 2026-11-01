package runner

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"
)

// Logged оборачивает Runner: каждый запуск попадает в лог — бинарник,
// подкоманда, код завершения, длительность и краткая ошибка (Brief). Остальные
// аргументы (пути, URL, логины), окружение и вывод CLI не пишутся.
func Logged(r Runner, log *slog.Logger) Runner {
	return logged{r: r, log: log}
}

type logged struct {
	r   Runner
	log *slog.Logger
}

func (l logged) Run(ctx context.Context, c Command) (Result, error) {
	start := time.Now()
	res, err := l.r.Run(ctx, c)
	attrs := []any{slog.String("bin", c.Name), slog.String("cmd", Subcommand(c.Args))}
	ms := slog.Int64("ms", time.Since(start).Milliseconds())
	if err != nil {
		// Процесс не запустился или прерван — кода завершения нет.
		l.log.Warn("команда не выполнена", append(attrs, ms, slog.String("error", Brief(err.Error())))...)
		return res, err
	}
	attrs = append(attrs, slog.Int("exit", res.ExitCode), ms)
	switch {
	case res.ExitCode != 0 && strings.TrimSpace(string(res.Stderr)) != "":
		l.log.Warn("команда завершилась с ошибкой", append(attrs, slog.String("error", Brief(string(res.Stderr))))...)
	default:
		// Ненулевой код без stderr у git часто содержателен (symbolic-ref при detached HEAD).
		l.log.Info("команда", attrs...)
	}
	return res, err
}

var wordRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Subcommand — подкоманда без параметров: до двух первых слов до флага или
// аргумента-значения («status», «auth status», «pr list», «api»); проверка
// версии — «--version».
func Subcommand(args []string) string {
	if len(args) > 0 && args[0] == "--version" {
		return args[0]
	}
	var words []string
	for _, a := range args {
		if len(words) == 2 || !wordRe.MatchString(a) {
			break
		}
		words = append(words, a)
	}
	return strings.Join(words, " ")
}
