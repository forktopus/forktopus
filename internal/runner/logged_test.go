package runner

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
)

func TestRedactSecrets(t *testing.T) {
	cases := map[string]string{
		"fatal: unable to access 'https://user:s3cret@github.com/o/r.git/'": "fatal: unable to access 'https://***@github.com/o/r.git/'",
		"token ghp_AbCdEf0123456789xyz is invalid":                          "token ghp_*** is invalid",
		"github_pat_11ABC_def":                                              "github_pat_***",
		"glpat-AbCd_ef-123 expired":                                         "glpat-*** expired",
		"Authorization: Bearer eyJhbGciOi.payload.sig":                      "Authorization: Bearer ***",
		"Authorization: token abc":                                          "Authorization: token ***",
		"got Bearer abcdefgh12345 from cache":                               "got Bearer *** from cache",
		"oauth_token: gho_xyz password=hunter2; secret: x":                  "oauth_token: *** password=***; secret: ***",
		"GITLAB_TOKEN=abc123 glab":                                          "GITLAB_TOKEN=*** glab",
		"git@github.com:o/r.git — без секрета":                              "git@github.com:o/r.git — без секрета",
	}
	for in, want := range cases {
		if got := Redact(in); got != want {
			t.Errorf("Redact(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestBrief(t *testing.T) {
	if got := Brief("\n  error: https://u:p@h/x failed\nsecond line with glpat-zzz\n"); got != "error: https://***@h/x failed" {
		t.Fatalf("Brief: %q", got)
	}
	long := Brief(strings.Repeat("я", 500))
	if !strings.HasSuffix(long, "…") || len([]rune(long)) != briefLimit+1 {
		t.Fatalf("Brief не обрезал: %d символов", len([]rune(long)))
	}
}

func TestSubcommand(t *testing.T) {
	cases := map[string][]string{
		"status":      {"status", "--porcelain=v2", "-z"},
		"auth status": {"auth", "status", "--hostname", "github.com"},
		"pr list":     {"pr", "list", "-R", "https://user:tok@git.example.org/o/r"},
		"api":         {"api", "/repos/o/r/pulls", "-l", "login"},
		"--version":   {"--version"},
		"":            {"-c", "core.x=y"},
		"ci list":     {"ci", "list", "extra"},
	}
	for want, args := range cases {
		if got := Subcommand(args); got != want {
			t.Errorf("Subcommand(%q) = %q, ожидалось %q", args, got, want)
		}
	}
}

type fakeRunner struct {
	res Result
	err error
}

func (f fakeRunner) Run(context.Context, Command) (Result, error) { return f.res, f.err }

// Лог не содержит аргументов, кроме подкоманды, окружения и вывода CLI; ошибка — замаскирована.
func TestLoggedWritesNoSecrets(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	cmd := Command{
		Name: "glab",
		Args: []string{"api", "projects/https%3A%2F%2Fuser%3Atok%40host", "-R", "https://user:tok@git.example.org/o/r"},
		Dir:  "/srv/repos/private-project",
		Env:  []string{"GITLAB_TOKEN=glpat-secretvalue"},
	}
	ctx := context.Background()
	Logged(fakeRunner{res: Result{ExitCode: 1, Stdout: []byte(`{"token":"glpat-fromstdout"}`), Stderr: []byte("401 Unauthorized: token glpat-abcdef is invalid\nmore")}}, log).Run(ctx, cmd)
	Logged(fakeRunner{res: Result{}}, log).Run(ctx, Command{Name: "git", Args: []string{"status", "--porcelain=v2"}, Dir: "/srv/x"})
	Logged(fakeRunner{err: model.Errorf(model.CodeTimeout, "git: превышено время ожидания")}, log).Run(ctx, Command{Name: "git", Args: []string{"fetch", "https://u:p@h/r"}})

	out := buf.String()
	for _, secret := range []string{"user:tok", "glpat-secretvalue", "glpat-fromstdout", "glpat-abcdef", "private-project", "/srv/x", "GITLAB_TOKEN", "u:p@", "projects/", "more"} {
		if strings.Contains(out, secret) {
			t.Errorf("в логе %q:\n%s", secret, out)
		}
	}
	for _, want := range []string{"bin=glab cmd=api exit=1", "glpat-***", "bin=git cmd=status exit=0", "level=INFO", "cmd=fetch", "превышено время ожидания"} {
		if !strings.Contains(out, want) {
			t.Errorf("в логе нет %q:\n%s", want, out)
		}
	}
}
