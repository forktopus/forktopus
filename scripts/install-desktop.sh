#!/usr/bin/env bash
# Регистрирует собранный bin/forktopus в меню рабочего стола Linux текущего
# пользователя: значок в hicolor и .desktop с StartupWMClass=forktopus.
# В GTK4 у окна нет собственного значка — панель задач берёт его из .desktop.
#   bash scripts/install-desktop.sh            — установить
#   bash scripts/install-desktop.sh --remove   — удалить установленные файлы
# Пишет только в ${XDG_DATA_HOME:-~/.local/share}; бинарник не копирует.
set -euo pipefail

PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
DATA="${XDG_DATA_HOME:-$HOME/.local/share}"
APPS="$DATA/applications"
ICONS="$DATA/icons/hicolor"
SIZES=(16 24 32 48 64 128 256 512)

refresh() {
  if command -v update-desktop-database >/dev/null; then update-desktop-database -q "$APPS" || true; fi
}

if [[ "${1:-}" == "--remove" ]]; then
  rm -f -- "$APPS/forktopus.desktop"
  for s in "${SIZES[@]}"; do rm -f -- "$ICONS/${s}x${s}/apps/forktopus.png"; done
  refresh
  echo "Удалено из $DATA"
  exit 0
fi

BIN="$PROJECT_ROOT/bin/forktopus"
ICON="$PROJECT_ROOT/build/appicon.png"
if [[ ! -x "$BIN" ]]; then
  echo "Нет $BIN — сначала собери: wails3 build" >&2
  exit 1
fi
command -v convert >/dev/null || { echo "Нужен ImageMagick (convert)" >&2; exit 1; }

for s in "${SIZES[@]}"; do
  mkdir -p -- "$ICONS/${s}x${s}/apps"
  convert "$ICON" -filter Lanczos -resize "${s}x${s}" -strip "$ICONS/${s}x${s}/apps/forktopus.png"
done

mkdir -p -- "$APPS"
# Exec из шаблона указывает на /usr/local/bin — подменяем на собранный бинарник.
sed -e "s|^Exec=.*|Exec=\"$BIN\"|" -e '/^# The Exec line/d' "$PROJECT_ROOT/build/linux/desktop" > "$APPS/forktopus.desktop"

refresh
echo "Установлено в $DATA (меню: Forktopus)"
