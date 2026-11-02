package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forktopus/forktopus/internal/updates"
)

// Изменение имён выпуска обязано менять selector обновлений одновременно.
// Проверяется и реальный формат SHA256SUMS, создаваемый prepare.
func TestUpdatePackageContract(t *testing.T) {
	version := "0.10.0"
	names := packages(version)
	formats := []string{"deb", "appimage", "nsis", "exe", "macos"}
	sums := map[string]string{}
	for _, name := range names {
		sum := sha256.Sum256([]byte(name))
		sums[name] = hex.EncodeToString(sum[:])
	}
	body := sumsFile(names, sums)
	for _, format := range formats {
		name := updates.AssetName(version, format)
		expected, ok := sums[name]
		if !ok {
			t.Fatalf("selector %s отсутствует в выпуске", name)
		}
		got, err := updates.ParseChecksum(body, name)
		if err != nil || hex.EncodeToString(got[:]) != expected {
			t.Fatalf("не прочитана сумма %s: %v", name, err)
		}
	}
}

// Проверяется развёрнутый граф Task, а не наличие строки в YAML: метка
// должна дойти через вложенные задачи до компилятора или Docker-сборки.
func TestPackageBuildMarkers(t *testing.T) {
	if _, err := exec.LookPath("wails3"); err != nil {
		t.Skip("wails3 нужен для проверки задач упаковки")
	}
	root := filepath.Join("..", "..")
	for _, tt := range []struct {
		name, format string
		args         []string
	}{
		{"deb", "deb", []string{"linux:generate:deb"}},
		{"appimage", "appimage", []string{"linux:create:appimage"}},
		{"exe", "exe", []string{"windows:build", "PACKAGE_TYPE=exe"}},
		{"nsis", "nsis", []string{"windows:create:nsis:installer", "INSTALL_SCOPE=user"}},
		{"macos", "macos", []string{"darwin:package:universal"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"task", "-dry"}, tt.args...)
			cmd := exec.Command("wails3", args...)
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			text := string(out)
			native := "internal/buildinfo.PackageFormat=" + tt.format
			docker := `PACKAGE_TYPE="` + tt.format + `"`
			if err != nil || strings.Contains(text, "ERROR") || (!strings.Contains(text, native) && !strings.Contains(text, docker)) {
				t.Fatalf("не передана метка %s: %v\n%s", tt.format, err, out)
			}
			if tt.format == "macos" && (!strings.Contains(text, "amd64") || !strings.Contains(text, "arm64")) {
				t.Fatal("не проверены обе архитектуры universal")
			}
		})
	}
}
