//go:build unix

package runner

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/forktopus/forktopus/internal/model"
)

// alive — процесс существует и не зомби (зомби уже не работает, его лишь не забрал init).
func alive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return true // не Linux: без /proc считаем живым, если kill(0) прошёл
	}
	// pid (comm) S …: состояние — первое поле после закрывающей скобки.
	fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	return len(fields) > 0 && fields[0] != "Z"
}

// Отмена завершает и потомков команды, как ssh у git fetch (11.4).
func TestCancelKillsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()

	start := time.Now()
	_, err := Exec{}.Run(ctx, Command{Name: "sh", Args: []string{"-c", `sleep 30 & echo $! > "$0"; wait`, pidFile}})
	if model.CodeOf(err) != model.CodeOpCancelled {
		t.Fatalf("ожидался OP_CANCELLED, получено %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("отмена заняла %v", d)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for alive(pid) {
		if time.Now().After(deadline) {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("потомок %d пережил отмену", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
