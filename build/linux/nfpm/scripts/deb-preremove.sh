#!/bin/sh
# При обновлении профиль остаётся загруженным до замены в postinst.
# При удалении выгружаем его, пока файл ещё доступен; отсутствие профиля допустимо.
case "${1:-}" in
  remove|deconfigure)
    if command -v apparmor_parser >/dev/null 2>&1 &&
       [ -f /etc/apparmor.d/forktopus ]; then
      apparmor_parser -R /etc/apparmor.d/forktopus || true
    fi
    ;;
esac
exit 0
