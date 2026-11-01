package notify

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// MaxSingle — сколько событий подряд приходят отдельными уведомлениями;
// больше — одно сводное, чтобы не засыпать рабочий стол.
const MaxSingle = 3

// maxSummaryLines — сколько событий перечисляется в сводке.
const maxSummaryLines = 5

// Message — одно уведомление: событие или сводка (RepoID пустой).
type Message struct {
	RepoID string
	Title  string
	Body   string
}

// Batcher собирает события, пришедшие почти одновременно (фоновая проверка
// читает репозитории параллельно), и отдаёт их send, когда delay проходит
// без новых событий.
type Batcher struct {
	delay time.Duration
	send  func([]Message)

	mu      sync.Mutex
	pending []Event
	timer   *time.Timer
}

func NewBatcher(delay time.Duration, send func([]Message)) *Batcher {
	return &Batcher{delay: delay, send: send}
}

// Add добавляет события в текущую пачку.
func (b *Batcher) Add(events []Event) {
	if len(events) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = append(b.pending, events...)
	if b.timer != nil {
		b.timer.Stop()
	}
	b.timer = time.AfterFunc(b.delay, b.flush)
}

func (b *Batcher) flush() {
	b.mu.Lock()
	events := b.pending
	b.pending, b.timer = nil, nil
	b.mu.Unlock()
	if len(events) > 0 {
		b.send(Messages(events))
	}
}

// Messages превращает пачку событий в уведомления: до MaxSingle — по одному,
// больше — одна сводка с первыми событиями.
func Messages(events []Event) []Message {
	if len(events) <= MaxSingle {
		out := make([]Message, len(events))
		for i, e := range events {
			out[i] = Message{RepoID: e.RepoID, Title: e.Title, Body: e.Body}
		}
		return out
	}
	lines := make([]string, 0, maxSummaryLines+1)
	for _, e := range events[:min(len(events), maxSummaryLines)] {
		lines = append(lines, e.Title+": "+e.Body)
	}
	if rest := len(events) - maxSummaryLines; rest > 0 {
		lines = append(lines, fmt.Sprintf("и ещё %d", rest))
	}
	return []Message{{Title: fmt.Sprintf("Forktopus: изменений — %d", len(events)), Body: strings.Join(lines, "\n")}}
}
