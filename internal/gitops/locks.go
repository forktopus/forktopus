// Package gitops — изменяющие операции над рабочей копией: fetch,
// fast-forward pull и switch на существующую ветку. Перед вызовом Git
// состояние проверяется по безопасной матрице (Check); никаких stash, reset,
// force и автоматических коммитов.
package gitops

import (
	"context"
	"sync"

	"github.com/forktopus/forktopus/internal/model"
)

// Locks — по одной блокировке на репозиторий. Изменяющая операция берёт её
// без ожидания (TryLock, D-031), чтение статуса — с ожиданием (Lock), чтобы
// не читать репозиторий посреди операции.
type Locks struct {
	mu sync.Mutex
	m  map[string]chan struct{}
}

func (l *Locks) slot(id string) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.m == nil {
		l.m = map[string]chan struct{}{}
	}
	c, ok := l.m[id]
	if !ok {
		c = make(chan struct{}, 1)
		l.m[id] = c
	}
	return c
}

// TryLock захватывает репозиторий или сразу возвращает REPO_BUSY.
func (l *Locks) TryLock(id string) (unlock func(), err error) {
	c := l.slot(id)
	select {
	case c <- struct{}{}:
		return func() { <-c }, nil
	default:
		return nil, model.Errorf(model.CodeRepoBusy, "над репозиторием уже выполняется операция, повторите после её завершения")
	}
}

// Lock ждёт освобождения репозитория; отмена ctx прерывает ожидание.
func (l *Locks) Lock(ctx context.Context, id string) (unlock func(), err error) {
	c := l.slot(id)
	select {
	case c <- struct{}{}:
		return func() { <-c }, nil
	case <-ctx.Done():
		return nil, model.Errorf(model.CodeOpCancelled, "ожидание операции над репозиторием отменено")
	}
}
