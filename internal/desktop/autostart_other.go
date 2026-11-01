//go:build !linux

package desktop

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/forktopus/forktopus/internal/app"
)

// NewAutostart — автозапуск средствами Wails: Windows — ключ Run в реестре,
// macOS — SMAppService или LaunchAgent.
func NewAutostart(a *application.App) app.Autostart {
	return wailsAutostart{a.Autostart}
}

type wailsAutostart struct{ m *application.AutostartManager }

func (w wailsAutostart) Enabled() (bool, error) { return w.m.IsEnabled() }

func (w wailsAutostart) SetEnabled(on bool) error {
	if on {
		return w.m.EnableWithOptions(application.AutostartOptions{Arguments: []string{HiddenFlag}})
	}
	return w.m.Disable()
}
