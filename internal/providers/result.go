package providers

import (
	"strings"
	"time"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// cliTimeout — таймаут одного запроса CLI к серверу.
const cliTimeout = 20 * time.Second

// now подменяется в тестах.
var now = time.Now

// availabilityOf — как UI показывает ошибку: нет CLI или авторизации —
// «недоступно», нет поддержки — «не поддерживается», прочее — ошибка источника.
func availabilityOf(code model.ErrorCode) model.Availability {
	switch code {
	case model.CodeCLIMissing, model.CodeAuthRequired:
		return model.AvailabilityUnavailable
	case model.CodeUnsupported:
		return model.AvailabilityNotSupported
	}
	return model.AvailabilityError
}

// prFailed — сводка PR без чисел: проблема не превращается в ноль.
func prFailed(e *model.Error) model.PRSummary {
	at := now()
	return model.PRSummary{Availability: availabilityOf(e.Code), Error: e, LastAttemptAt: &at}
}

func ciFailed(e *model.Error) model.CISummary {
	at := now()
	return model.CISummary{Availability: availabilityOf(e.Code), Error: e, LastAttemptAt: &at}
}

func prOK(open, draft int, url string) model.PRSummary {
	at := now()
	return model.PRSummary{Availability: model.AvailabilityOK, Open: &open, Draft: &draft, URL: url, LastSuccessAt: &at, LastAttemptAt: &at}
}

func ciOK(active int, last, url string) model.CISummary {
	at := now()
	return model.CISummary{Availability: model.AvailabilityOK, Active: &active, LastResult: last, URL: url, LastSuccessAt: &at, LastAttemptAt: &at}
}

// noBranch — при detached HEAD PR/CI не запрашиваются (idea 04 §7).
func noBranch() *model.Error {
	return model.Errorf(model.CodeInvalidInput, "нет текущей ветки (detached HEAD)")
}

// marker — признак ошибки в выводе CLI. Сравнение без учёта регистра.
type marker struct {
	code model.ErrorCode
	text string
}

// classify превращает неудачный запуск CLI в ошибку ядра: сначала ошибка
// запуска (нет CLI, таймаут, отмена), затем первый подходящий маркер в stderr.
// В сообщение попадает строка с маркером — без токенов в URL.
func classify(cli string, res runner.Result, err error, markers []marker) *model.Error {
	if err != nil {
		return model.AsError(err)
	}
	out := runner.RedactURLs(string(res.Stderr))
	lower := strings.ToLower(out)
	for _, m := range markers {
		if i := strings.Index(lower, strings.ToLower(m.text)); i >= 0 {
			return &model.Error{Code: m.code, Message: cli + ": " + lineAt(out, i), ExitCode: res.ExitCode}
		}
	}
	msg := firstLine(out)
	if msg == "" {
		msg = "завершился с ошибкой"
	}
	return &model.Error{Code: model.CodeOther, Message: cli + ": " + msg, ExitCode: res.ExitCode}
}

// lineAt — строка текста, в которой находится позиция i.
func lineAt(s string, i int) string {
	start := strings.LastIndexByte(s[:i], '\n') + 1
	end := strings.IndexByte(s[i:], '\n')
	if end < 0 {
		return strings.TrimSpace(s[start:])
	}
	return strings.TrimSpace(s[start : i+end])
}

func firstLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// badOutput — CLI ответил кодом 0, но вывод не разобрался.
func badOutput(cli string, err error) *model.Error {
	return model.Errorf(model.CodeOther, "%s: неожиданный формат ответа: %v", cli, err)
}
