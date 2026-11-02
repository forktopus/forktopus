package updates

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/model"
)

type fakeSource struct {
	formats chan string
	release Release
	check   func(context.Context) (*Release, error)
	open    func(context.Context) (io.ReadCloser, error)
	calls   atomic.Int32
}

func (f *fakeSource) Check(ctx context.Context, current, format string) (*Release, error) {
	if f.formats != nil {
		f.formats <- format
	}
	if f.check != nil {
		return f.check(ctx)
	}
	r := f.release
	return &r, nil
}
func (f *fakeSource) Open(ctx context.Context, r Release) (io.ReadCloser, error) {
	f.calls.Add(1)
	return f.open(ctx)
}

func testService(t *testing.T, data string) (*Service, *fakeSource) {
	t.Helper()
	f := &fakeSource{release: Release{Version: "0.4.0", Filename: AssetName("0.4.0", "deb"), Size: int64(len(data)), SHA256: sha256.Sum256([]byte(data))}}
	f.open = func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(data)), nil }
	s := New(context.Background(), Options{Version: "0.3.1", Format: "deb", Source: f, Directory: t.TempDir()})
	t.Cleanup(s.Close)
	s.Check()
	waitStatus(t, s, "available")
	return s, f
}
func waitStatus(t *testing.T, s *Service, want string) model.AppUpdate {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st := s.State()
		if st.Status == want {
			return st
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("ожидали %s: %+v", want, s.State())
	return model.AppUpdate{}
}
func noParts(t *testing.T, dir string) {
	t.Helper()
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".part") {
			t.Errorf("оставлен %s", p)
		}
		return nil
	})
}

func TestDownloadAndRevalidate(t *testing.T) {
	s, f := testService(t, "байты пакета")
	keep := filepath.Join(s.dir, "user-file")
	os.WriteFile(keep, []byte("оставить"), 0o600)
	s.Download()
	ready := waitStatus(t, s, "ready")
	if ready.CanDownload || ready.Downloaded != f.release.Size {
		t.Fatalf("%+v", ready)
	}
	dir, err := s.ReadyFolder(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(s.ready) != f.release.Filename || filepath.Dir(s.ready) != dir {
		t.Fatal("неверное имя пакета")
	}
	if b, err := os.ReadFile(keep); err != nil || string(b) != "оставить" {
		t.Fatal("чужой файл изменён")
	}
	noParts(t, s.dir)
	// Повторная проверка ловит изменение при том же размере, затем разрешает retry.
	os.WriteFile(s.ready, []byte(strings.Repeat("x", int(f.release.Size))), 0o600)
	if _, err = s.ReadyFolder(context.Background()); err == nil || !s.State().CanDownload {
		t.Fatal("подмена выдана как готовый пакет")
	}
	s.Download()
	waitStatus(t, s, "ready")
	os.Remove(s.ready)
	if _, err = s.ReadyFolder(context.Background()); err == nil {
		t.Fatal("удалённый файл принят")
	}
}

func TestDownloadFailures(t *testing.T) {
	for _, scenario := range []string{"short", "extra", "hash", "network", "folder", "permission"} {
		t.Run(scenario, func(t *testing.T) {
			s, f := testService(t, "correct")
			switch scenario {
			case "short":
				f.open = func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("bad")), nil }
			case "extra":
				f.open = func(context.Context) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("too-long-payload")), nil
				}
			case "hash":
				f.open = func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("changed")), nil }
			case "network":
				f.open = func(context.Context) (io.ReadCloser, error) { return nil, errors.New("нет сети") }
			case "folder":
				p := filepath.Join(s.dir, "file")
				os.WriteFile(p, []byte("x"), 0o600)
				s.dir = p
			case "permission":
				if os.Geteuid() == 0 || runtime.GOOS == "windows" {
					t.Skip("Unix-права недоступны для этого окружения")
				}
				os.Chmod(s.dir, 0o500)
				t.Cleanup(func() { os.Chmod(s.dir, 0o700) })
			}
			s.Download()
			st := waitStatus(t, s, "error")
			if !st.CanDownload || s.ready != "" {
				t.Fatalf("%+v", st)
			}
			if scenario != "folder" {
				noParts(t, s.dir)
			}
		})
	}
}

// Блокирующий reader позволяет проверить отмену реального io.Copy и двойной
// клик, не подменяя внутреннее состояние сервиса.
type cancelReader struct {
	ctx     context.Context
	once    sync.Once
	started chan struct{}
}

func (r *cancelReader) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-r.ctx.Done()
	return 0, r.ctx.Err()
}
func (r *cancelReader) Close() error { return nil }
func TestCancelConcurrencyAndShutdown(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		s, f := testService(t, "correct")
		started := make(chan struct{})
		f.open = func(ctx context.Context) (io.ReadCloser, error) {
			return &cancelReader{ctx: ctx, started: started}, nil
		}
		s.Download()
		<-started
		var clicks sync.WaitGroup
		for range 20 {
			clicks.Go(func() { s.Download(); s.Check() })
		}
		clicks.Wait()
		if f.calls.Load() != 1 {
			t.Fatalf("создано загрузок %d", f.calls.Load())
		}
		if shutdown {
			s.Close()
		} else {
			s.Cancel()
		}
		waitStatus(t, s, "available")
		noParts(t, s.dir)
		if !shutdown {
			f.open = func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("correct")), nil }
			s.Download()
			waitStatus(t, s, "ready")
		}
	}
}

type fullDisk struct{}

func (fullDisk) Write([]byte) (int, error) { return 0, syscall.ENOSPC }
func TestWriterDiskFullAndBound(t *testing.T) {
	w := packageWriter{ctx: context.Background(), out: fullDisk{}, size: 10, progress: func(int64, string) {}}
	if _, err := w.Write([]byte("123")); !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("%v", err)
	}
	var b strings.Builder
	w.out = &b
	w.size = 2
	if _, err := w.Write([]byte("123")); err == nil || b.Len() != 0 {
		t.Fatal("лишние байты записаны на диск")
	}
}

func TestCheckStatesAndImmutableSnapshot(t *testing.T) {
	s, f := testService(t, "correct")
	f.release.Version = "0.5.0"
	f.release.Filename = "other"
	s.Download()
	waitStatus(t, s, "ready")
	if s.State().Version != "0.4.0" {
		t.Fatal("снимок изменился вслед за latest")
	}
	f.check = func(context.Context) (*Release, error) { return nil, errors.New("проверочная ошибка") }
	s.Check()
	st := waitStatus(t, s, "error")
	if st.CanDownload || st.Version != "" {
		t.Fatal("сохранился старый пакет после неуспешной проверки")
	}
	f.check = func(context.Context) (*Release, error) { return nil, nil }
	s.Check()
	waitStatus(t, s, "current")
}

func TestUnsupportedAndCancelledVerification(t *testing.T) {
	s, f := testService(t, "correct")
	f.check = func(context.Context) (*Release, error) { return nil, ErrNoPackage }
	s.Check()
	waitStatus(t, s, "unsupported")
	f.check = nil
	s.Check()
	waitStatus(t, s, "available")
	s.Download()
	waitStatus(t, s, "ready")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ReadyFolder(ctx); err == nil || s.State().Status != "ready" {
		t.Fatal("отмена проверки потеряла готовый пакет")
	}
	s = New(context.Background(), Options{Version: "0.3.1", Source: f, Directory: t.TempDir()})
	defer s.Close()
	s.Check()
	if s.State().CanDownload || s.candidate != nil {
		t.Fatal("неверный формат сохранил доступ к пакету")
	}
}

func TestFixedPackageFormat(t *testing.T) {
	for _, format := range []string{"deb", "appimage", "nsis", "exe", "macos"} {
		t.Run(format, func(t *testing.T) {
			source := &fakeSource{formats: make(chan string, 2), release: Release{Version: "0.4.0", Filename: AssetName("0.4.0", format), Size: 1}}
			s := New(context.Background(), Options{Version: "0.3.1", Format: format, Directory: t.TempDir(), Source: source})
			defer s.Close()
			for range 2 {
				s.Check()
				waitStatus(t, s, "available")
				if got := <-source.formats; got != format {
					t.Fatalf("запрошен %s вместо %s", got, format)
				}
				if s.State().Format != format || s.State().Filename != AssetName("0.4.0", format) {
					t.Fatal("тип поставки изменился")
				}
			}
		})
	}
}

func TestUnmarkedBuildDoesNotCheckNetwork(t *testing.T) {
	for _, format := range []string{"", "unknown"} {
		source := &fakeSource{check: func(context.Context) (*Release, error) {
			t.Error("сетевая проверка без метки")
			return nil, nil
		}}
		s := New(context.Background(), Options{Version: "0.3.1", Format: format, Directory: t.TempDir(), Source: source})
		st := s.Check()
		if st.Status != "unsupported" || st.Format != "" || st.Message == "" {
			t.Fatalf("%+v", st)
		}
		s.Download()
		s.Close()
		if source.calls.Load() != 0 {
			t.Fatal("скачивание без метки")
		}
	}
}
