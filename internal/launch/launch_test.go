package launch

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNearestDir(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(repo, "f.txt")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, want string }{
		{repo, repo},
		{filepath.Join(repo, "gone", "deeper"), repo},
		{file, repo},
	} {
		got, err := NearestDir(tc.path)
		if err != nil || got != tc.want {
			t.Errorf("NearestDir(%q) = %q, %v; want %q", tc.path, got, err, tc.want)
		}
	}
	home, _ := os.UserHomeDir()
	if got, err := NearestDir(""); err != nil || got != home {
		t.Errorf("NearestDir(\"\") = %q, %v; want %q", got, err, home)
	}
}

func TestCleanEnv(t *testing.T) {
	plain := []string{"PATH=/usr/bin", "LD_LIBRARY_PATH=/x"}
	if got := cleanEnv(plain); !slices.Equal(got, plain) {
		t.Errorf("вне AppImage окружение изменилось: %v", got)
	}
	sep := string(filepath.ListSeparator)
	env := []string{
		"APPDIR=/tmp/.mount_x",
		"PATH=/tmp/.mount_x/usr/bin" + sep + "/usr/bin",
		"LD_LIBRARY_PATH=/tmp/.mount_x/usr/lib",
		"XDG_DATA_DIRS=/tmp/.mount_x/usr/share" + sep + "/usr/share",
		"GSETTINGS_SCHEMA_DIR=/tmp/.mount_x/usr/share/glib-2.0/schemas",
		"HOME=/home/u",
		"PATHX=/tmp/.mount_xy",
	}
	want := []string{"PATH=/usr/bin", "XDG_DATA_DIRS=/usr/share", "HOME=/home/u", "PATHX=/tmp/.mount_xy"}
	if got := cleanEnv(env); !slices.Equal(got, want) {
		t.Errorf("cleanEnv = %v; want %v", got, want)
	}
}
