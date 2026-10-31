package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/model"
)

func TestRunSuccessAndExitCode(t *testing.T) {
	ctx := context.Background()
	res, err := Exec{}.Run(ctx, Command{Name: "git", Args: []string{"--version"}})
	if err != nil || !res.OK() || !strings.HasPrefix(string(res.Stdout), "git version") {
		t.Fatalf("git --version: res=%+v err=%v", res, err)
	}

	// Ненулевой код завершения — не ошибка запуска.
	res, err = Exec{}.Run(ctx, Command{Name: "git", Args: []string{"rev-parse", "--git-dir"}, Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if res.OK() || res.ExitCode == 0 || len(res.Stderr) == 0 {
		t.Fatalf("ожидался ненулевой код и stderr: %+v", res)
	}
}

func TestRunMissingBinary(t *testing.T) {
	_, err := Exec{}.Run(context.Background(), Command{Name: "forktopus-no-such-binary"})
	if model.CodeOf(err) != model.CodeCLIMissing {
		t.Fatalf("ожидался CLI_MISSING, получено %v", err)
	}
}

func TestRunTimeoutAndCancel(t *testing.T) {
	_, err := Exec{}.Run(context.Background(), Command{Name: "sleep", Args: []string{"5"}, Timeout: 100 * time.Millisecond})
	if model.CodeOf(err) != model.CodeTimeout {
		t.Fatalf("ожидался TIMEOUT, получено %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	_, err = Exec{}.Run(ctx, Command{Name: "sleep", Args: []string{"5"}})
	if model.CodeOf(err) != model.CodeOpCancelled {
		t.Fatalf("ожидался OP_CANCELLED, получено %v", err)
	}
}

func TestRunArgsAreNotInterpreted(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "каталог с пробелами")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	args := []string{
		"пробелы и кириллица",
		"--",
		"--not-a-flag",
		"перевод\nстроки",
		"$(echo injected); `id` | cat > x",
	}
	// printf печатает каждый аргумент отдельной записью, разделённой NUL.
	res, err := Exec{}.Run(context.Background(), Command{
		Name: "printf",
		Args: append([]string{`%s\0`}, args...),
		Dir:  dir,
	})
	if err != nil || !res.OK() {
		t.Fatalf("printf: res=%+v err=%v", res, err)
	}
	got := strings.Split(strings.TrimSuffix(string(res.Stdout), "\x00"), "\x00")
	if len(got) != len(args) {
		t.Fatalf("аргументы склеились или разделились: %q", got)
	}
	for i := range args {
		if got[i] != args[i] {
			t.Errorf("аргумент %d: %q, ожидалось %q", i, got[i], args[i])
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "x")); err == nil {
		t.Error("аргумент был выполнен как команда shell")
	}
}

func TestRunEnvAndDir(t *testing.T) {
	dir := t.TempDir()
	res, err := Exec{}.Run(context.Background(), Command{
		Name: "sh",
		Args: []string{"-c", `printf '%s|%s|%s' "$PWD" "$GIT_TERMINAL_PROMPT" "$FORKTOPUS_TEST"`},
		Dir:  dir,
		Env:  []string{"FORKTOPUS_TEST=1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := dir + "|0|1"
	if string(res.Stdout) != want {
		// На некоторых системах TempDir — симлинк; сравниваем по EvalSymlinks.
		real, _ := filepath.EvalSymlinks(dir)
		if string(res.Stdout) != real+"|0|1" {
			t.Errorf("получено %q, ожидалось %q", res.Stdout, want)
		}
	}
}

func TestRedactURLs(t *testing.T) {
	cases := map[string]string{
		"fatal: https://user:ghp_secret@github.com/o/r.git":  "fatal: https://***@github.com/o/r.git",
		"https://oauth2:token@git.example.com:8443/base/o/r": "https://***@git.example.com:8443/base/o/r",
		"ssh://git@host:2222/team/repo.git":                  "ssh://***@host:2222/team/repo.git",
		"git@github.com:owner/repo.git":                      "git@github.com:owner/repo.git",
		"https://github.com/owner/repo":                      "https://github.com/owner/repo",
		"два: http://a:b@x/1 и https://c@y/2":                "два: http://***@x/1 и https://***@y/2",
	}
	for in, want := range cases {
		if got := RedactURLs(in); got != want {
			t.Errorf("RedactURLs(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}
