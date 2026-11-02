#!/bin/sh
set -eu

# Профиль нужен при активном AppArmor; на системах без него установка работает
# как обычно. Ошибку загрузки не скрываем: иначе установленный ярлык снова падает.
profile=/etc/apparmor.d/forktopus
if [ -r /sys/module/apparmor/parameters/enabled ] &&
   grep -q '^Y' /sys/module/apparmor/parameters/enabled &&
   command -v apparmor_parser >/dev/null 2>&1 &&
   [ ! -e /etc/apparmor.d/disable/forktopus ]; then
  apparmor_parser -r -W "$profile"
fi

# Меню приложений и значок подхватываются без повторного входа в сеанс.
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database -q /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
fi
