package main

import "github.com/godbus/dbus/v5"

// trayAvailable — есть ли в сессии хост значков StatusNotifierItem: в KDE
// он встроен, в GNOME появляется с расширением AppIndicator. Без него значок
// не виден, и прятать окно в трей нельзя — его было бы не вернуть.
func trayAvailable() bool {
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return false
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return false
	}
	if err := conn.Hello(); err != nil {
		return false
	}
	var has bool
	err = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.kde.StatusNotifierWatcher").Store(&has)
	return err == nil && has
}
