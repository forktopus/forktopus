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

// AppDir — каталог приложения внутри base (os.UserConfigDir или
// os.UserCacheDir). Переменная FORKTOPUS_DIR заменяет оба: для разработки —
// второй экземпляр со своими настройками рядом с установленным.
func AppDir(base func() (string, error)) (string, error) {
	if dir := os.Getenv("FORKTOPUS_DIR"); dir != "" {
		return dir, nil
	}
	dir, err := base()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "forktopus"), nil
}
