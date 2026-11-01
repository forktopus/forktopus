package app

import "github.com/forktopus/forktopus/internal/model"

// Autostart — запуск при входе в систему (D-063). Реализацию задаёт main
// (internal/desktop); состояние читается из системы, а не из настроек:
// пользователь мог убрать запись сам.
type Autostart interface {
	Enabled() (bool, error)
	SetEnabled(on bool) error
}

// GetAutostart — включён ли запуск при входе; nil — не поддерживается.
func (s *AppService) GetAutostart() (*bool, error) {
	if s.Autostart == nil {
		return nil, nil
	}
	on, err := s.Autostart.Enabled()
	if err != nil {
		return nil, model.AsError(err)
	}
	return &on, nil
}

// SetAutostart включает или выключает запуск при входе (сразу в трей, без окна).
func (s *AppService) SetAutostart(on bool) error {
	if s.Autostart == nil {
		return nil
	}
	if err := s.Autostart.SetEnabled(on); err != nil {
		return model.AsError(err)
	}
	return nil
}
