#!/usr/bin/env bash
# Регистрирует локальный bin/forktopus отдельным ярлыком forktopus-dev.desktop.
# Имя GTK, значок и настройки отделены от установленного приложения из deb.
#   bash scripts/install-desktop.sh            — установить локальный ярлык
#   bash scripts/install-desktop.sh --remove   — удалить локальный ярлык
# Пишет только в ${XDG_DATA_HOME:-~/.local/share}; бинарник не копирует.
set -euo pipefail

PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
DATA="${XDG_DATA_HOME:-$HOME/.local/share}"
APPS="$DATA/applications"
ICONS="$DATA/icons/hicolor"
BIN="$PROJECT_ROOT/bin/forktopus"
DEV_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/forktopus-dev"
SIZES=(16 24 32 48 64 128 256 512)

refresh() {
  if command -v update-desktop-database >/dev/null; then update-desktop-database -q "$APPS" || true; fi
}

# Удаляем только прежний ярлык этого скрипта, указывающий на текущую сборку.
# Произвольный пользовательский forktopus.desktop не трогаем.
legacy="$APPS/forktopus.desktop"
if [[ -f "$legacy" ]] && grep -Fxq -- "Exec=\"$BIN\"" "$legacy"; then
  rm -f -- "$legacy"
  for s in "${SIZES[@]}"; do rm -f -- "$ICONS/${s}x${s}/apps/forktopus.png"; done
fi

if [[ "${1:-}" == "--remove" ]]; then
  rm -f -- "$APPS/forktopus-dev.desktop"
  for s in "${SIZES[@]}"; do rm -f -- "$ICONS/${s}x${s}/apps/forktopus-dev.png"; done
  refresh
  echo "Локальный ярлык удалён из $DATA"
  exit 0
fi

ICON="$PROJECT_ROOT/build/appicon.png"
if [[ ! -x "$BIN" ]]; then
  echo "Нет $BIN — сначала собери: wails3 build" >&2
  exit 1
fi
command -v convert >/dev/null || { echo "Нужен ImageMagick (convert)" >&2; exit 1; }

for s in "${SIZES[@]}"; do
  mkdir -p -- "$ICONS/${s}x${s}/apps"
  convert "$ICON" -filter Lanczos -resize "${s}x${s}" -strip "$ICONS/${s}x${s}/apps/forktopus-dev.png"
done

# Exec разбирается по правилам .desktop, а не оболочкой. Экранируем пути,
# включая знак %, чтобы он не превратился в код подстановки аргументов.
desktop_quote() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//\`/\\\`}"
  value="${value//\$/\\\$}"
  value="${value//%/%%}"
  # Второй слой экранирования нужен для самого строкового значения .desktop.
  value="${value//\\/\\\\}"
  printf '\"%s\"' "$value"
}

mkdir -p -- "$APPS"
{
  printf '%s\n' '[Desktop Entry]' 'Version=1.0' 'Type=Application'
  printf '%s\n' 'Name=Forktopus (локальная сборка)' 'Comment=Локальная сборка с отдельными настройками'
  printf 'Exec=env FORKTOPUS_DESKTOP_ID=forktopus-dev %s %s\n' "$(desktop_quote "FORKTOPUS_DIR=$DEV_DIR")" "$(desktop_quote "$BIN")"
  printf '%s\n' 'Icon=forktopus-dev' 'Terminal=false' 'Categories=Development;' 'StartupWMClass=forktopus-dev'
} > "$APPS/forktopus-dev.desktop"

refresh
echo "Установлено в $DATA (меню: Forktopus (локальная сборка))"
