package runner

import "testing"

func TestMergePath(t *testing.T) {
	got := mergePath([]string{`C:\Windows`, `C:\Program Files\GitHub CLI\`, ""}, `c:\windows;C:\tools;C:\Program Files\GitHub CLI;`, ";")
	if want := `C:\Windows;C:\Program Files\GitHub CLI\;C:\tools`; got != want {
		t.Errorf("mergePath: %q, ждали %q", got, want)
	}
}
