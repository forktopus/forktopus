// Package fsutil — небольшие помощники для работы с файлами.
package fsutil

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic записывает файл через временный файл в том же каталоге,
// fsync и rename: читатель видит либо старое, либо новое содержимое целиком.
// Каталог создаётся с правами 0700.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // после успешного rename файла уже нет

	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
