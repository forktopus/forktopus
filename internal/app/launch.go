package app

import (
	"github.com/forktopus/forktopus/internal/launch"
	"github.com/forktopus/forktopus/internal/model"
)

// OpenFolder открывает каталог в файловом менеджере системы; если его нет —
// ближайший существующий выше по пути (D-066).
func (s *AppService) OpenFolder(path string) error {
	if err := launch.Folder(path); err != nil {
		return model.AsError(err)
	}
	return nil
}

// OpenTerminal запускает терминал системы в каталоге; пустой путь —
// домашний каталог (D-066).
func (s *AppService) OpenTerminal(path string) error {
	if err := launch.Terminal(path); err != nil {
		return model.AsError(err)
	}
	return nil
}
