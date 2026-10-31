package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileRotatesBySize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", FileName)
	f, err := Open(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for i := range 6 {
		fmt.Fprintf(f, "строка %02d — %s\n", i, strings.Repeat("x", 20))
	}

	cur, _ := os.ReadFile(path)
	old, _ := os.ReadFile(path + ".1")
	if len(cur) > 100 || len(old) > 100 || len(old) == 0 {
		t.Fatalf("ротация: текущий %d байт, .1 — %d байт", len(cur), len(old))
	}
	if !strings.Contains(string(cur), "строка 05") {
		t.Fatalf("последняя запись потеряна:\n%s", cur)
	}
	for _, p := range []string{path, path + ".1"} {
		if st, err := os.Stat(p); err != nil || st.Mode().Perm() != 0o600 {
			t.Fatalf("%s: права %v, ошибка %v", p, st.Mode().Perm(), err)
		}
	}
	if st, _ := os.Stat(filepath.Dir(path)); st.Mode().Perm() != 0o700 {
		t.Fatalf("каталог: права %v", st.Mode().Perm())
	}

	// Повторное открытие дописывает и учитывает размер.
	f.Close()
	f, err = Open(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if f.size != int64(len(cur)) {
		t.Fatalf("размер после открытия: %d, ожидалось %d", f.size, len(cur))
	}
}

func TestRecentKeepsWarnings(t *testing.T) {
	var sb strings.Builder
	log, recent := New(&sb)
	log.Info("обычная команда", "bin", "git")
	log.With("bin", "gh").Warn("команда завершилась с ошибкой", "error", "auth required")
	for i := range recentLimit + 5 {
		log.Error("ошибка", "n", i)
	}

	got := recent.Entries()
	if len(got) != recentLimit {
		t.Fatalf("запомнено %d, ожидалось %d", len(got), recentLimit)
	}
	if got[0].Message != fmt.Sprintf("ошибка n=%d", recentLimit+4) || got[0].Level != "ERROR" {
		t.Fatalf("новые должны быть первыми: %+v", got[0])
	}

	log2, recent := New(&sb)
	log2.With("bin", "gh").Warn("команда завершилась с ошибкой", "error", "auth required")
	if e := recent.Entries(); len(e) != 1 || e[0].Message != "команда завершилась с ошибкой bin=gh error=auth required" {
		t.Fatalf("запись: %+v", e)
	}
	if !strings.Contains(sb.String(), "level=INFO msg=\"обычная команда\" bin=git") {
		t.Fatalf("в файл пишутся все уровни от Info:\n%s", sb.String())
	}
}
