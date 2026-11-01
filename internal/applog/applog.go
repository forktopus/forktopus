// Package applog — технический лог приложения в каталоге кэша ОС с ротацией
// по размеру и памятью о последних ошибках для отчёта диагностики. Секреты
// маскирует тот, кто пишет (runner.Logged, runner.Brief): сюда попадают уже
// безопасные строки.
package applog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/forktopus/forktopus/internal/fsutil"
)

const (
	// FileName — имя файла лога в каталоге кэша.
	FileName = "forktopus.log"
	// MaxSize — размер файла, после которого он переименовывается в .1 (старый .1 удаляется).
	MaxSize = 2 << 20
	// recentLimit — сколько последних предупреждений и ошибок помнить для отчёта.
	recentLimit = 20
)

// DefaultPath — путь к логу в каталоге кэша ОС (рядом с кэшем репозиториев).
func DefaultPath() (string, error) {
	dir, err := fsutil.AppDir(os.UserCacheDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// File — файл лога с ротацией по размеру. Права как у настроек: каталог 0700, файл 0600.
type File struct {
	mu   sync.Mutex
	path string
	max  int64
	f    *os.File
	size int64
}

// Open открывает (создаёт) файл лога для дописывания.
func Open(path string, max int64) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	w := &File{path: path, max: max}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *File) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f, w.size = f, st.Size()
	return nil
}

func (w *File) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size > 0 && w.size+int64(len(p)) > w.max {
		w.f.Close()
		// Ошибку переименования не возвращаем: лучше дописать в старый файл, чем потерять запись.
		_ = os.Rename(w.path, w.path+".1")
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// Close закрывает файл.
func (w *File) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

// Entry — запомненное предупреждение или ошибка.
type Entry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// Recent помнит последние записи уровня Warn и выше.
type Recent struct {
	mu      sync.Mutex
	entries []Entry
}

// Entries — последние предупреждения и ошибки, новые — первыми.
func (r *Recent) Entries() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := slices.Clone(r.entries)
	slices.Reverse(out)
	return out
}

func (r *Recent) add(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
	if len(r.entries) > recentLimit {
		r.entries = slices.Delete(r.entries, 0, len(r.entries)-recentLimit)
	}
}

// New — логгер в w (текстовый формат slog) и память о последних ошибках.
func New(w io.Writer) (*slog.Logger, *Recent) {
	recent := &Recent{}
	h := &recentHandler{Handler: slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}), recent: recent}
	return slog.New(h), recent
}

// recentHandler пишет запись дальше и копирует предупреждения в Recent:
// сообщение и атрибуты одной строкой, как в файле.
type recentHandler struct {
	slog.Handler
	recent *Recent
	attrs  []slog.Attr
}

func (h *recentHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		msg := r.Message
		add := func(a slog.Attr) bool {
			msg += " " + a.String()
			return true
		}
		for _, a := range h.attrs {
			add(a)
		}
		r.Attrs(add)
		h.recent.add(Entry{Time: r.Time, Level: r.Level.String(), Message: msg})
	}
	return h.Handler.Handle(ctx, r)
}

func (h *recentHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &recentHandler{Handler: h.Handler.WithAttrs(attrs), recent: h.recent, attrs: append(slices.Clone(h.attrs), attrs...)}
}

func (h *recentHandler) WithGroup(name string) slog.Handler {
	return &recentHandler{Handler: h.Handler.WithGroup(name), recent: h.recent, attrs: h.attrs}
}
