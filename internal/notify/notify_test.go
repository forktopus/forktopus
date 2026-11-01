package notify

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/model"
)

func ptr(n int) *int { return &n }

// repo — репозиторий на ветке с отставанием behind, итогом CI ci ("" — CI
// недоступен) и числом открытых PR open (-1 — PR недоступны).
func repo(branch string, behind int, ci string, open int) model.Repository {
	r := model.Repository{ID: "r1", Path: "/src/app", Branch: &model.BranchStatus{Name: branch, Behind: ptr(behind)}}
	if ci != "" {
		r.CI = model.CISummary{Availability: model.AvailabilityOK, LastResult: ci}
	} else {
		r.CI = model.CISummary{Availability: model.AvailabilityError}
	}
	if open >= 0 {
		r.PR = model.PRSummary{Availability: model.AvailabilityOK, Open: ptr(open)}
	}
	return r
}

func bodies(events []Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = string(e.Kind) + ": " + e.Body
	}
	return out
}

func TestObserveTransitions(t *testing.T) { // 18.3
	tr := NewTracker()
	steps := []struct {
		name string
		repo model.Repository
		want []string
	}{
		{"первое чтение — точка отсчёта", repo("main", 0, "failure", 0), nil},
		{"без изменений", repo("main", 0, "failure", 0), nil},
		{"CI починился", repo("main", 0, "success", 0), []string{"ci: CI на main снова проходит"}},
		{"сеть отвалилась — состояние не теряется", repo("main", 0, "", -1), nil},
		{"CI упал, ветка отстала, открыт PR", repo("main", 2, "failure", 1), []string{
			"behind: Ветка main отстаёт от upstream на 2 коммита — можно сделать Pull",
			"ci: CI упал на main",
			"pr: Открыт PR/MR по main",
		}},
		{"отставание уменьшилось (pull) — молчим", repo("main", 0, "failure", 1), nil},
		{"другая ветка — новая точка отсчёта", repo("feature", 5, "success", 0), nil},
		{"отстала сильнее", repo("feature", 6, "success", 0), []string{"behind: Ветка feature отстаёт от upstream на 6 коммитов — можно сделать Pull"}},
	}
	for _, s := range steps {
		got := bodies(tr.Observe(s.repo))
		if strings.Join(got, "\n") != strings.Join(s.want, "\n") {
			t.Errorf("%s:\n получено %q\n ожидалось %q", s.name, got, s.want)
		}
	}
}

func TestObservePRClosed(t *testing.T) { // 18.3
	tr := NewTracker()
	tr.Observe(repo("main", 0, "success", 1))
	merged := repo("main", 0, "success", 0)
	merged.PR.Last = &model.PRRef{Number: 42, Merged: true}
	if got := bodies(tr.Observe(merged)); len(got) != 1 || got[0] != "pr: PR/MR #42 по main влит" {
		t.Fatalf("влитый PR: %q", got)
	}

	tr.Observe(repo("main", 0, "success", 1))
	closed := repo("main", 0, "success", 0)
	closed.PR.Last = &model.PRRef{Number: 43}
	if got := bodies(tr.Observe(closed)); len(got) != 1 || got[0] != "pr: PR/MR #43 по main закрыт без слияния" {
		t.Fatalf("закрытый PR: %q", got)
	}
}

func TestObserveWithoutStatusAndForget(t *testing.T) { // 18.3
	tr := NewTracker()
	if got := tr.Observe(model.Repository{ID: "r1"}); got != nil {
		t.Fatalf("репозиторий без статуса: %v", got)
	}
	tr.Observe(repo("main", 0, "success", 0))
	tr.Forget("r1")
	if got := tr.Observe(repo("main", 3, "failure", 0)); got != nil {
		t.Fatalf("после Forget — точка отсчёта, а получено %v", bodies(got))
	}
}

func TestCommitsPlural(t *testing.T) {
	for n, want := range map[int]string{1: "коммит", 2: "коммита", 5: "коммитов", 11: "коммитов", 12: "коммитов", 21: "коммит", 22: "коммита", 111: "коммитов"} {
		if got := commits(n); got != want {
			t.Errorf("%d: %s, ожидалось %s", n, got, want)
		}
	}
}

func TestMessages(t *testing.T) { // 18.3
	ev := func(i int) Event {
		return Event{Kind: KindCI, RepoID: "r", Title: "app", Body: "CI упал на b" + string(rune('0'+i))}
	}
	three := Messages([]Event{ev(1), ev(2), ev(3)})
	if len(three) != 3 || three[0].RepoID != "r" {
		t.Fatalf("до трёх — по одному: %+v", three)
	}

	var many []Event
	for i := range 7 {
		many = append(many, ev(i))
	}
	got := Messages(many)
	if len(got) != 1 || got[0].RepoID != "" || got[0].Title != "Forktopus: изменений — 7" {
		t.Fatalf("сводка: %+v", got)
	}
	if lines := strings.Split(got[0].Body, "\n"); len(lines) != 6 || lines[5] != "и ещё 2" {
		t.Fatalf("тело сводки: %q", got[0].Body)
	}
}

func TestBatcher(t *testing.T) { // 18.3
	var mu sync.Mutex
	var sent [][]Message
	b := NewBatcher(50*time.Millisecond, func(m []Message) {
		mu.Lock()
		sent = append(sent, m)
		mu.Unlock()
	})
	e := Event{Kind: KindBehind, RepoID: "r", Title: "app", Body: "x"}
	b.Add(nil)
	b.Add([]Event{e})
	b.Add([]Event{e, e, e})
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 || len(sent[0]) != 1 || sent[0][0].RepoID != "" {
		t.Fatalf("события подряд — одна сводка: %+v", sent)
	}
}
