// Package notify решает, о чём сообщить пользователю системным уведомлением
// (D-063): о смене состояния репозитория между двумя чтениями, а не о каждом
// опросе. CI текущей ветки упал или снова прошёл, ветка стала отставать от
// upstream сильнее, PR/MR по ветке открыт, влит или закрыт.
package notify

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/forktopus/forktopus/internal/model"
)

// Kind — вид события; по нему уведомления включаются и выключаются в настройках.
type Kind string

const (
	KindCI     Kind = "ci"
	KindBehind Kind = "behind"
	KindPR     Kind = "pr"
)

// Event — одно событие для уведомления.
type Event struct {
	Kind   Kind
	RepoID string
	Title  string // имя репозитория
	Body   string
}

// Tracker помнит последнее увиденное состояние каждого репозитория. Первое
// чтение репозитория, как и первое чтение после смены ветки, — точка отсчёта:
// событий по нему нет.
type Tracker struct {
	mu   sync.Mutex
	seen map[string]state
}

// state — то, что сравнивается между чтениями; known — значение уже было.
type state struct {
	ref         string // ветка или @SHA при detached HEAD
	behind      int
	behindKnown bool
	ci          string // последний итог CI: success / failure
	prOpen      int
	prKnown     bool
}

func NewTracker() *Tracker {
	return &Tracker{seen: map[string]state{}}
}

// Observe сравнивает свежее чтение с прежним и возвращает события. Данные,
// которые не удалось получить (нет статуса, CLI недоступен), прежние не
// затирают: сбой сети не превращается в «CI снова зелёный».
func (t *Tracker) Observe(repo model.Repository) []Event {
	if repo.Branch == nil {
		return nil
	}
	ref := repo.Branch.Name
	if repo.Branch.Detached {
		ref = "@" + repo.Branch.HeadSHA
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	prev, ok := t.seen[repo.ID]
	if !ok || prev.ref != ref {
		prev = state{ref: ref}
		ok = false
	}
	next := prev
	var events []Event
	add := func(kind Kind, format string, args ...any) {
		if ok {
			events = append(events, Event{Kind: kind, RepoID: repo.ID, Title: filepath.Base(repo.Path), Body: fmt.Sprintf(format, args...)})
		}
	}

	if b := repo.Branch.Behind; b != nil {
		if prev.behindKnown && *b > prev.behind {
			add(KindBehind, "Ветка %s отстаёт от upstream на %d %s — можно сделать Pull", ref, *b, commits(*b))
		}
		next.behind, next.behindKnown = *b, true
	}

	if ci := repo.CI; ci.Availability == model.AvailabilityOK && ci.LastResult != "" {
		switch {
		case prev.ci == "" || prev.ci == ci.LastResult:
		case ci.LastResult == "failure":
			add(KindCI, "CI упал на %s", ref)
		case prev.ci == "failure":
			add(KindCI, "CI на %s снова проходит", ref)
		}
		next.ci = ci.LastResult
	}

	if pr := repo.PR; pr.Availability == model.AvailabilityOK && pr.Open != nil {
		open := *pr.Open
		if prev.prKnown {
			switch {
			case open > prev.prOpen:
				add(KindPR, "Открыт PR/MR по %s", ref)
			case open == 0 && prev.prOpen > 0 && pr.Last != nil && pr.Last.Merged:
				add(KindPR, "PR/MR #%d по %s влит", pr.Last.Number, ref)
			case open == 0 && prev.prOpen > 0 && pr.Last != nil:
				add(KindPR, "PR/MR #%d по %s закрыт без слияния", pr.Last.Number, ref)
			case open == 0 && prev.prOpen > 0:
				add(KindPR, "PR/MR по %s закрыт", ref)
			}
		}
		next.prOpen, next.prKnown = open, true
	}

	t.seen[repo.ID] = next
	return events
}

// Forget убирает репозиторий (корень удалён): вернётся — будет новой точкой отсчёта.
func (t *Tracker) Forget(id string) {
	t.mu.Lock()
	delete(t.seen, id)
	t.mu.Unlock()
}

// commits — «коммит», «коммита», «коммитов».
func commits(n int) string {
	switch {
	case n%10 == 1 && n%100 != 11:
		return "коммит"
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
		return "коммита"
	default:
		return "коммитов"
	}
}
