package updates

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/forktopus/forktopus/internal/model"
)

// Options задаются приложением, а не renderer. Каталог и источник не могут
// быть заменены аргументами UI; тесты используют временную папку и fixture.
type Options struct {
	Version, Directory, Format, Reason string
	Source                             Source
	OnChange                           func(model.AppUpdate)
}

// Service хранит операцию в Go: закрытие и повторное открытие настроек не
// прерывают скачивание. Все изменяющие действия сериализованы busy под mu.
type Service struct {
	mu        sync.Mutex
	state     model.AppUpdate
	candidate *Release
	ready     string
	source    Source
	dir       string
	life      context.Context
	cancel    context.CancelFunc
	busy      bool
	closed    bool
	wg        sync.WaitGroup
	onChange  func(model.AppUpdate)
}

// New не обращается к сети; проверка начинается только действием пользователя.
func New(life context.Context, o Options) *Service {
	st := model.AppUpdate{Revision: 1, Status: "idle", CurrentVersion: o.Version, Format: o.Format, Message: o.Reason, Instruction: instruction(o.Format)}
	if o.Format == "" || AssetName(o.Version, o.Format) == "" {
		st.Status = "unsupported"
		st.Format = ""
		if st.Message == "" {
			st.Message = "Тип поставки не указан. Проверка обновлений недоступна."
		}
	}
	return &Service{state: st, source: o.Source, dir: o.Directory, life: life, onChange: o.OnChange}
}

// State возвращает копию снимка без внутренних путей и загрузочных URL.
func (s *Service) State() model.AppUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot()
}

func (s *Service) snapshot() model.AppUpdate {
	st := s.state
	return st
}

// publish вызывается с mu; callback снаружи блокировки. Revision сохраняет
// порядок снимков даже при задержке доставки ответа относительно события.
func (s *Service) publish() model.AppUpdate { s.state.Revision++; return s.snapshot() }
func (s *Service) emit(st model.AppUpdate) {
	if s.onChange != nil {
		s.onChange(st)
	}
}

// Check запускает проверку только пакета встроенного типа поставки.
// Формат задаётся один раз при создании сервиса; UI его не передаёт.
func (s *Service) Check() model.AppUpdate {
	s.mu.Lock()
	if s.busy || s.closed || s.state.Format == "" {
		st := s.snapshot()
		s.mu.Unlock()
		return st
	}
	format := s.state.Format
	s.busy = true
	s.wg.Add(1)
	ctx, cancel := context.WithTimeout(s.life, 20*time.Second)
	s.cancel = cancel
	s.candidate = nil
	s.ready = ""
	s.state.Format = format
	s.state.Instruction = instruction(format)
	s.state.Status = "checking"
	s.state.Message = ""
	s.state.CanDownload = false
	s.state.Version = ""
	s.state.Filename = ""
	s.state.ReleaseURL = ""
	s.state.Size = 0
	s.state.Downloaded = 0
	st := s.publish()
	s.mu.Unlock()
	s.emit(st)
	go func() {
		defer s.wg.Done()
		defer cancel()
		release, err := s.source.Check(ctx, st.CurrentVersion, format)
		s.mu.Lock()
		s.busy = false
		s.cancel = nil
		s.state.CheckedAt = time.Now().UTC().Format(time.RFC3339)
		switch {
		case errors.Is(err, ErrNoPackage):
			s.state.Status = "unsupported"
			s.state.Message = err.Error()
		case err != nil:
			s.state.Status = "error"
			s.state.Message = message(err)
		case release == nil:
			s.state.Status = "current"
			s.state.Message = "Установлена актуальная стабильная версия."
		default:
			// Сохраняем собственную копию: Source не может изменить выбранный пакет.
			r := *release
			s.candidate = &r
			s.state.Status = "available"
			s.state.Version = r.Version
			s.state.Filename = r.Filename
			s.state.Size = r.Size
			s.state.ReleaseURL = r.Page
			s.state.CanDownload = true
		}
		next := s.publish()
		s.mu.Unlock()
		s.emit(next)
	}()
	return st
}

// Download не использует context JS-вызова: закрытие диалога не отменяет
// загрузку. Отмена пользователя и завершение приложения имеют свой context.
func (s *Service) Download() model.AppUpdate {
	s.mu.Lock()
	if s.busy || s.closed || s.candidate == nil || !s.state.CanDownload {
		st := s.snapshot()
		s.mu.Unlock()
		return st
	}
	r := *s.candidate
	ctx, cancel := context.WithTimeout(s.life, 30*time.Minute)
	s.cancel = cancel
	s.busy = true
	s.wg.Add(1)
	s.ready = ""
	s.state.Status = "downloading"
	s.state.Message = ""
	s.state.Downloaded = 0
	s.state.CanDownload = false
	st := s.publish()
	s.mu.Unlock()
	s.emit(st)
	go func() {
		defer s.wg.Done()
		defer cancel()
		path, err := s.download(ctx, r)
		s.mu.Lock()
		s.busy = false
		s.cancel = nil
		switch {
		case err == nil:
			s.ready = path
			s.state.Status = "ready"
			s.state.Message = "Размер и SHA-256 проверены. Откройте папку и установите пакет вручную."
		case errors.Is(err, context.Canceled):
			s.state.Status = "available"
			s.state.Message = "Скачивание отменено."
			s.state.CanDownload = true
			s.state.Downloaded = 0
		default:
			s.state.Status = "error"
			s.state.Message = message(err)
			s.state.CanDownload = true
		}
		next := s.publish()
		s.mu.Unlock()
		s.emit(next)
	}()
	return st
}

// Cancel прерывает скачивание или проверку файла, но не удаляет чужие файлы.
func (s *Service) Cancel() model.AppUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil && (s.state.Status == "downloading" || s.state.Status == "verifying") {
		s.cancel()
	}
	return s.snapshot()
}

// Close ждёт очистки .part, чтобы штатное завершение не оставляло обрывки.
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *Service) progress(written int64, status string) {
	s.mu.Lock()
	s.state.Downloaded = written
	s.state.Status = status
	st := s.publish()
	s.mu.Unlock()
	s.emit(st)
}

// packageWriter ограничивает объём до записи на диск и ограничивает частоту
// событий. io.Copy не может записать лишние байты при неверном Content-Length.
type packageWriter struct {
	ctx           context.Context
	out           io.Writer
	size, written int64
	last          time.Time
	progress      func(int64, string)
}

func (w *packageWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.size-w.written {
		return 0, errors.New("пакет превышает заявленный размер")
	}
	n, err := w.out.Write(p)
	w.written += int64(n)
	if time.Since(w.last) >= 150*time.Millisecond {
		w.last = time.Now()
		w.progress(w.written, "downloading")
	}
	return n, err
}

func (s *Service) download(ctx context.Context, r Release) (string, error) {
	if s.dir == "" {
		return "", errors.New("не удалось определить каталог кэша обновлений")
	}
	if r.Size <= 0 || r.Size > maxPackageSize || filepath.Base(r.Filename) != r.Filename || r.Filename == "." {
		return "", errors.New("некорректные метаданные пакета")
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", fmt.Errorf("не удалось создать папку обновлений: %w", err)
	}
	// Уникальный .part не затирает предыдущий пакет и пользовательские файлы.
	f, err := os.CreateTemp(s.dir, "."+r.Filename+"-*.part")
	if err != nil {
		return "", fmt.Errorf("не удалось создать временный файл: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	body, err := s.source.Open(ctx, r)
	if err != nil {
		return "", err
	}
	defer body.Close()
	hash := sha256.New()
	writer := &packageWriter{ctx: ctx, out: io.MultiWriter(f, hash), size: r.Size, progress: s.progress}
	if _, err = io.Copy(writer, body); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("не удалось скачать пакет; проверьте сеть и свободное место: %w", err)
	}
	s.progress(writer.written, "verifying")
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if writer.written != r.Size {
		return "", errors.New("пакет скачан не полностью: размер не совпадает")
	}
	if !slices.Equal(hash.Sum(nil), r.SHA256[:]) {
		return "", errors.New("SHA-256 пакета не совпадает. Скачайте файл повторно")
	}
	if err = f.Sync(); err != nil {
		return "", fmt.Errorf("не удалось сохранить пакет: %w", err)
	}
	if err = f.Close(); err != nil {
		return "", fmt.Errorf("не удалось закрыть пакет: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	// У каждой успешной загрузки своя папка, внутри — точное имя пакета.
	// Предыдущие файлы сохраняются, rename переносит только проверенный .part.
	finalDir, err := os.MkdirTemp(s.dir, "verified-*")
	if err != nil {
		return "", err
	}
	path := filepath.Join(finalDir, r.Filename)
	if err = os.Rename(f.Name(), path); err != nil {
		os.Remove(finalDir)
		return "", fmt.Errorf("не удалось сохранить проверенный пакет: %w", err)
	}
	return path, nil
}

// ReadyFolder повторно читает файл перед открытием папки: удалённый или
// изменённый пакет больше не показывается как готовый к установке.
func (s *Service) ReadyFolder(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.busy || s.closed || s.ready == "" || s.candidate == nil {
		s.mu.Unlock()
		return "", errors.New("сначала скачайте и проверьте пакет")
	}
	path, r := s.ready, *s.candidate
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.life, cancel)
	defer stop()
	defer cancel()
	s.cancel = cancel
	s.busy = true
	s.wg.Add(1)
	s.state.Status = "verifying"
	st := s.publish()
	s.mu.Unlock()
	s.emit(st)
	defer s.wg.Done()
	err := verify(ctx, path, r)
	s.mu.Lock()
	s.busy = false
	s.cancel = nil
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		s.state.Status = "ready"
		s.state.Message = "Проверка файла отменена."
	} else if err != nil {
		s.ready = ""
		s.state.Status = "error"
		s.state.Message = "Готовый файл удалён или изменён. Скачайте пакет повторно."
		s.state.CanDownload = true
	} else {
		s.state.Status = "ready"
	}
	next := s.publish()
	s.mu.Unlock()
	s.emit(next)
	if err != nil {
		return "", errors.New(next.Message)
	}
	return filepath.Dir(path), nil
}

func verify(ctx context.Context, path string, r Release) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != r.Size {
		return errors.New("размер или тип файла изменился")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	w := &packageWriter{ctx: ctx, out: h, size: r.Size, progress: func(int64, string) {}}
	n, err := io.Copy(w, f)
	if err != nil {
		return err
	}
	if n != r.Size || !slices.Equal(h.Sum(nil), r.SHA256[:]) {
		return errors.New("содержимое файла изменилось")
	}
	return ctx.Err()
}

func message(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "Превышено время ожидания. Проверьте сеть и повторите попытку."
	}
	if errors.Is(err, context.Canceled) {
		return "Операция отменена."
	}
	return err.Error()
}

func instruction(format string) string {
	switch format {
	case "deb":
		return "Закройте Forktopus, установите скачанный deb через центр приложений или пакетный менеджер и запустите приложение с панели. Настройки сохранятся."
	case "appimage":
		return "Закройте Forktopus, замените прежний AppImage, сохранив его имя и путь для ярлыка и автозапуска. Разрешите запуск файла и откройте его снова."
	case "nsis":
		return "Закройте Forktopus, запустите скачанный установщик и откройте установленное приложение. Настройки сохранятся."
	case "exe":
		return "Закройте Forktopus и замените прежний exe, сохранив его имя и путь для ярлыка и автозапуска. Запустите приложение снова."
	case "macos":
		return "Закройте Forktopus, распакуйте ZIP и замените прежний forktopus.app. Откройте приложение снова; инструкция первого запуска находится на странице релиза."
	}
	return ""
}
