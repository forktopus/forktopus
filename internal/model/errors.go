package model

import (
	"errors"
	"fmt"
)

// ErrorCode — машинный код ошибки, общий для Go и UI.
// Контракт описан в idea/plan/04-GIT-AND-PROVIDERS.md §6.
type ErrorCode string

const (
	CodeCLIMissing     ErrorCode = "CLI_MISSING"     // нет исполняемого файла
	CodeAuthRequired   ErrorCode = "AUTH_REQUIRED"   // нет авторизации / доступа к инстансу
	CodeAccountBlocked ErrorCode = "ACCOUNT_BLOCKED" // сервер запретил вход или аккаунт не активирован
	CodeAccessDenied   ErrorCode = "ACCESS_DENIED"   // вход есть, но прав на этот раздел репозитория нет (HTTP 403, D-068)
	CodeUnsupported    ErrorCode = "UNSUPPORTED"     // команды/формата нет в этой версии CLI
	CodeNetworkError   ErrorCode = "NETWORK_ERROR"   // сервер недоступен, DNS, TLS
	CodeRateLimit      ErrorCode = "RATE_LIMIT"      // сервис ограничил частоту запросов
	CodeRepoUnresolved ErrorCode = "REPO_UNRESOLVED" // remote не сопоставлен с проектом
	CodeGitConflict    ErrorCode = "GIT_CONFLICT"    // конфликт или незавершённая операция Git
	CodeDirtyWorktree  ErrorCode = "DIRTY_WORKTREE"  // локальные изменения блокируют операцию
	CodeNoUpstream     ErrorCode = "NO_UPSTREAM"     // у ветки нет отслеживаемой ветки
	CodeOpCancelled    ErrorCode = "OP_CANCELLED"    // операция отменена
	CodeTimeout        ErrorCode = "TIMEOUT"         // команда не уложилась в отведённое время
	CodeInvalidInput   ErrorCode = "INVALID_INPUT"   // некорректный аргумент вызова
	CodeOther          ErrorCode = "OTHER"           // прочее: см. сообщение и код завершения

	// Коды безопасных операций (D-037).
	CodeRepoBusy            ErrorCode = "REPO_BUSY"            // над репозиторием уже идёт операция
	CodeStateChanged        ErrorCode = "STATE_CHANGED"        // состояние изменилось с момента показа в UI
	CodeNoRemote            ErrorCode = "NO_REMOTE"            // нет remote или он не выбран
	CodeDiverged            ErrorCode = "DIVERGED"             // история разошлась, fast-forward невозможен
	CodeBranchInWorktree    ErrorCode = "BRANCH_IN_WORKTREE"   // ветка checked out в другой рабочей копии
	CodeDetachedUnreachable ErrorCode = "DETACHED_UNREACHABLE" // коммит detached HEAD не входит ни в одну ветку
)

// Error — типизированная ошибка ядра. Сериализуется в JSON и в таком виде
// приходит во фронтенд как cause отклонённого вызова Wails.
// Message должен быть безопасным для показа: без токенов и userinfo в URL.
type Error struct {
	Code     ErrorCode `json:"code"`
	Message  string    `json:"message"`
	ExitCode int       `json:"exitCode,omitempty"`
}

func (e *Error) Error() string {
	return string(e.Code) + ": " + e.Message
}

// Errorf создаёт ошибку с кодом и форматированным сообщением.
func Errorf(code ErrorCode, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// CodeOf возвращает код ошибки ядра из цепочки err, иначе CodeOther.
func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		if e == nil { // типизированный nil: (*Error)(nil), переданный как error
			return ""
		}
		return e.Code
	}
	return CodeOther
}

// AsError приводит err к *Error, оборачивая чужие ошибки в CodeOther.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: CodeOther, Message: err.Error()}
}
