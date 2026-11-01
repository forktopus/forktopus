package providers

import (
	"slices"
	"strings"
	"time"

	"github.com/forktopus/forktopus/internal/model"
	"github.com/forktopus/forktopus/internal/runner"
)

// cliTimeout — таймаут одного запроса CLI к серверу.
const cliTimeout = 20 * time.Second

// now подменяется в тестах.
var now = time.Now

// AvailabilityOf — как UI показывает ошибку: нет CLI или авторизации —
// «недоступно», нет поддержки — «не поддерживается», прочее — ошибка источника.
func AvailabilityOf(code model.ErrorCode) model.Availability {
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
	return model.PRSummary{Availability: AvailabilityOf(e.Code), Error: e, LastAttemptAt: &at}
}

func ciFailed(e *model.Error) model.CISummary {
	at := now()
	return model.CISummary{Availability: AvailabilityOf(e.Code), Error: e, LastAttemptAt: &at}
}

func prOK(open, draft int, url string) model.PRSummary {
	at := now()
	return model.PRSummary{Availability: model.AvailabilityOK, Open: &open, Draft: &draft, URL: url, LastSuccessAt: &at, LastAttemptAt: &at}
}

func ciOK(active int, last, url string) model.CISummary {
	at := now()
	return model.CISummary{Availability: model.AvailabilityOK, Active: &active, LastResult: last, URL: url, LastSuccessAt: &at, LastAttemptAt: &at}
}

// latestPR — последний по времени закрытия из закрытых/влитых PR ветки;
// без времени — в конце. nil — таких нет.
func latestPR(refs []model.PRRef) *model.PRRef {
	var best *model.PRRef
	for i := range refs {
		r := &refs[i]
		if best == nil || r.ClosedAt != nil && (best.ClosedAt == nil || r.ClosedAt.After(*best.ClosedAt)) {
			best = r
		}
	}
	return best
}

// parseTime — время RFC 3339 из ответа CLI; пустое или неразобранное — nil.
func parseTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

// withLast — сводка без открытых PR дополняется последним закрытым (best effort:
// ошибка этого запроса сводку не портит).
func withLast(sum model.PRSummary, last func() *model.PRRef) model.PRSummary {
	if sum.Open != nil && *sum.Open == 0 {
		sum.Last = last()
	}
	return sum
}

// noBranch — ни ветки, ни коммита (detached HEAD без прочитанного SHA).
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
			return &model.Error{Code: m.code, Message: cli + ": " + strings.TrimPrefix(lineAt(out, i), "Error: "), ExitCode: res.ExitCode}
		}
	}
	msg := strings.TrimPrefix(firstLine(out), "Error: ")
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

// runState — запуск workflow в формате Actions (GitHub, Gitea).
type runState struct {
	key        string // workflow + событие: одна проверка коммита
	sha        string
	done       bool
	conclusion string
}

// runOK — завершения, которые GitHub не считает провалом.
var runOK = []string{"success", "neutral", "skipped"}

// lastRunResult — итог последнего коммита ветки (D-043); runs — новые первыми.
// По каждой проверке (workflow + событие) берётся её последний запуск на этом
// коммите: повторный успешный перекрывает упавший, незавершённый итога не даёт.
// Провал любой проверки, в том числе cancelled и timed_out, — failure.
func lastRunResult(runs []runState) string {
	if len(runs) == 0 {
		return ""
	}
	head := runs[0].sha
	seen := map[string]bool{}
	result := ""
	for _, r := range runs {
		if r.sha != head || seen[r.key] {
			continue
		}
		seen[r.key] = true
		switch {
		case !r.done:
		case !slices.Contains(runOK, r.conclusion):
			return "failure"
		default:
			result = "success"
		}
	}
	return result
}
