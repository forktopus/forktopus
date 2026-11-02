package buildinfo

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// Версия встроенного config должна совпадать с упаковкой и после release bump.
func TestVersion(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "build", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^  version: "([^"]+)"`).FindSubmatch(b)
	if len(m) != 2 || Version() != string(m[1]) {
		t.Fatalf("несовпадение версии: %s", Version())
	}
}

func TestDistribution(t *testing.T) {
	for _, tt := range []struct {
		os, arch, format string
		production       bool
		want             string
	}{
		{"linux", "amd64", "deb", true, "deb"},
		{"linux", "amd64", "appimage", true, "appimage"},
		{"windows", "amd64", "nsis", true, "nsis"},
		{"windows", "amd64", "exe", true, "exe"},
		{"darwin", "amd64", "macos", true, "macos"},
		{"darwin", "arm64", "macos", true, "macos"},
		{"linux", "amd64", "", true, ""},
		{"windows", "amd64", "", true, ""},
		{"linux", "amd64", "nsis", true, ""},
		{"windows", "amd64", "deb", true, ""},
		{"darwin", "arm64", "appimage", true, ""},
		{"linux", "arm64", "deb", true, ""},
		{"windows", "arm64", "exe", true, ""},
		{"linux", "amd64", "deb", false, ""},
	} {
		t.Run(tt.os+"/"+tt.arch+"/"+tt.format, func(t *testing.T) {
			format, reason := distribution(tt.os, tt.arch, tt.production, tt.format)
			if format != tt.want || (reason == "") != (tt.want != "") {
				t.Fatalf("получено %q: %s", format, reason)
			}
		})
	}
}

// Окружение AppImage не должно переопределять встроенную метку другой поставки.
func TestDistributionIgnoresEnvironment(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("сценарий Linux amd64")
	}
	t.Setenv("APPIMAGE", "/tmp/other.AppImage")
	before := PackageFormat
	PackageFormat = "deb"
	defer func() { PackageFormat = before }()
	if Production {
		format, _ := Distribution()
		if format != "deb" {
			t.Fatalf("получено %s", format)
		}
	}
}
