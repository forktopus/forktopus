// Command release — выпуск Forktopus (D-072): bump меняет версию в build/ и
// коммитит её; plan только читает Git и GitHub; prepare ставит локальный тег и
// собирает комплект из артефактов успешного CI коммита; publish отправляет тег,
// загружает файлы и сверяет их SHA-256 через GitHub API. Порядок — в
// docs/release.md.
//
//	go run ./scripts/release bump --version 0.3.0
//	go run ./scripts/release plan [--version 0.3.0]
//	go run ./scripts/release prepare --version 0.3.0 --notes bin/release/notes-0.3.0.md
//	go run ./scripts/release publish --version 0.3.0
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const usage = `Команды:
  release bump --version X.Y.Z
  release plan [--version X.Y.Z]
  release prepare --version X.Y.Z --notes <файл>
  release publish --version X.Y.Z`

// command запускает программу в dir: stdout возвращается строкой, stderr виден в терминале.
func command(dir string) func(name string, args ...string) (string, error) {
	return func(name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args[:min(2, len(args))], " "), err)
		}
		return strings.TrimSpace(out.String()), nil
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if isReleaseError(err) {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Fprintln(os.Stderr, "Выпуск остановлен:", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fail("%s", usage)
	}
	name := args[0]
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	version := fs.String("version", "", "версия X.Y.Z")
	notes := fs.String("notes", "", "файл с описанием релиза (prepare)")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 0 || (*notes != "" && name != "prepare") {
		return fail("%s", usage)
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	exe := command(dir)
	git := func(args ...string) (string, error) { return exe("git", args...) }
	if top, err := git("rev-parse", "--show-toplevel"); err == nil {
		dir, exe = top, command(top)
	}
	t := &Tool{Dir: dir, Git: func(args ...string) (string, error) { return exe("git", args...) }, GH: ghClient{run: exe}}
	var res map[string]any
	switch name {
	case "bump":
		res, err = t.Bump(*version)
	case "plan":
		res, err = t.Plan(*version)
	case "prepare":
		res, err = t.Prepare(*version, *notes)
	case "publish":
		res, err = t.Publish(*version)
	default:
		return fail("%s", usage)
	}
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
