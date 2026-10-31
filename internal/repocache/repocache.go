// Package repocache хранит между запусками последний результат сканирования
// (D-027) в каталоге кэша ОС. Файл можно удалить без потерь; повреждённый
// файл игнорируется.
package repocache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/forktopus/forktopus/internal/fsutil"
	"github.com/forktopus/forktopus/internal/model"
)

// FormatVersion — версия формата; кэш другой версии игнорируется.
const FormatVersion = 1

type file struct {
	Version      int                `json:"version"`
	ScannedAt    time.Time          `json:"scannedAt"`
	Repositories []model.Repository `json:"repositories"`
	Issues       []model.ScanIssue  `json:"issues"`
}

// DefaultPath — путь к файлу кэша в каталоге кэша ОС.
func DefaultPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "forktopus", "repositories.json"), nil
}

// Load читает кэш. ok = false — кэша нет, он повреждён или другой версии.
func Load(path string) (res model.ScanResult, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.ScanResult{}, false
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil || f.Version != FormatVersion || f.Repositories == nil {
		return model.ScanResult{}, false
	}
	if f.Issues == nil {
		f.Issues = []model.ScanIssue{}
	}
	return model.ScanResult{Repositories: f.Repositories, Issues: f.Issues, ScannedAt: f.ScannedAt}, true
}

// Save атомарно записывает кэш.
func Save(path string, res model.ScanResult) error {
	data, err := json.Marshal(file{Version: FormatVersion, ScannedAt: res.ScannedAt, Repositories: res.Repositories, Issues: res.Issues})
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, data, 0o600)
}
