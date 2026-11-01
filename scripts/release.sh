#!/usr/bin/env bash
# Собирает файлы релиза с именами, как на странице Releases, и SHA256SUMS
# в bin/release/<версия>/:
#   linux   — forktopus_<v>_amd64.deb и Forktopus-<v>-x86_64.AppImage (только на Linux);
#   windows — forktopus-<v>-windows-amd64.exe (на любой ОС: сборка без CGO) и
#             установщик forktopus-<v>-windows-amd64-setup.exe (NSIS, нужен makensis);
#   macos   — Forktopus-<v>-macos-universal.zip с .app (Intel и Apple Silicon); на macOS —
#             нативно с ad-hoc подписью, на Linux — в Docker-образе wails-cross без
#             подписи (один раз: wails3 task setup:docker). Не проверялось.
#
#   bash scripts/release.sh [--os linux|windows|macos ...]
#
#   --os       что собирать: можно повторять или через запятую (--os linux,windows);
#              без флага — linux и windows
#
# Версия — текущая из build/config.yml; меняет её go run ./scripts/release bump.
# Файлы выпуска собирает CI на push в main, релиз — go run ./scripts/release
# (D-072, docs/release.md); этот скрипт — та же сборка локально. Ничего не
# коммитит и не публикует. Файлы других ОС той же версии в
# bin/release/<версия>/ остаются, SHA256SUMS считается по всем.
# AppImage при первой сборке скачивает linuxdeploy (кэш в build/linux/appimage/build).
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

usage() {
  sed -n '10,13p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

die() {
  echo "$*" >&2
  exit 1
}

oses=()
while (($#)); do
  case "$1" in
    --os) (($# >= 2)) || die "--os: нужна ОС"; IFS=, read -ra list <<< "$2"; oses+=("${list[@]}"); shift 2 ;;
    --os=*) IFS=, read -ra list <<< "${1#*=}"; oses+=("${list[@]}"); shift ;;
    -h | --help) usage ;;
    *) echo "Неизвестный аргумент: $1" >&2; usage 1 ;;
  esac
done
((${#oses[@]})) || oses=(linux windows)

want() {
  local o
  for o in "${oses[@]}"; do [[ "$o" == "$1" ]] && return 0; done
  return 1
}

for o in "${oses[@]}"; do
  case "$o" in
    linux | windows | macos) ;;
    *) die "--os: $o — допустимо linux, windows, macos" ;;
  esac
done
host="$(uname -s)"
want linux && [[ "$host" != Linux ]] && die "Linux-пакеты собираются только на Linux (GTK4 и WebKitGTK через CGO)"
if want macos && [[ "$host" != Darwin ]]; then
  # Не на Mac Wails собирает macOS в Docker: Zig и macOS SDK внутри образа.
  docker image inspect wails-cross > /dev/null 2>&1 ||
    die "Для macOS на $host нужен Docker-образ wails-cross (один раз, ~4 ГБ на диске): wails3 task setup:docker"
fi

want windows && ! command -v makensis > /dev/null &&
  die "Для установщика Windows нужен NSIS (makensis): sudo apt install nsis, brew install makensis или choco install nsis"

new="$(sed -n 's/^  version: "\([0-9.]*\)".*/\1/p' build/config.yml)"
[[ -n "$new" ]] || die "Не нашёл версию в build/config.yml"

out="bin/release/$new"
mkdir -p -- "$out"

if want linux; then
  wails3 build
  wails3 task linux:generate:deb
  wails3 task linux:create:appimage
  # Проверка: пакет собран с нужной версией (nfpm добавляет ревизию: 0.3.0-1).
  deb_version="$(dpkg-deb -f bin/forktopus.deb Version)"
  [[ "${deb_version%-*}" == "$new" ]] || die "В .deb версия $deb_version, ожидалась $new"
  cp bin/forktopus.deb "$out/forktopus_${new}_amd64.deb"
  cp bin/forktopus-x86_64.AppImage "$out/Forktopus-${new}-x86_64.AppImage"
fi

if want windows; then
  wails3 build GOOS=windows
  cp bin/forktopus.exe "$out/forktopus-${new}-windows-amd64.exe"
  # Установщик для текущего пользователя (без прав администратора): в
  # %LOCALAPPDATA%\Programs\Forktopus, ярлыки и запись в «Приложениях».
  # WebView2 bootstrapper ставит среду, если её нет.
  # Путь к exe — абсолютный: makensis на Windows не нашёл относительный. В Git
  # Bash он нужен в виде Windows (D:\...), makensis — не программа MSYS.
  exe="$PWD/bin/forktopus.exe"
  command -v cygpath > /dev/null && exe="$(cygpath -w "$exe")"
  wails3 generate webview2bootstrapper -dir build/windows/nsis
  (cd build/windows/nsis && makensis -V2 -DWAILS_INSTALL_SCOPE=user -DREQUEST_EXECUTION_LEVEL=user \
    "-DARG_WAILS_AMD64_BINARY=$exe" project.nsi)
  cp bin/forktopus-amd64-installer.exe "$out/forktopus-${new}-windows-amd64-setup.exe"
fi

if want macos; then
  wails3 task darwin:package:universal
  zip_name="$out/Forktopus-${new}-macos-universal.zip"
  rm -f -- "$zip_name"
  if [[ "$host" == Darwin ]]; then
    ditto -c -k --keepParent bin/forktopus.app "$zip_name"
  else
    # Сборка в контейнере оставляет в корне пустой -.o от root (Wails после
    # сборки возвращает владельца только bin/); каталог наш — удалить можно.
    rm -f -- ./-.o
    # zip сохраняет права на запуск и симлинки внутри .app.
    (cd bin && zip -qry "../$zip_name" forktopus.app)
  fi
  # Сборка оставляет в bin/forktopus universal-бинарник для Mac (он уже внутри
  # .app). Go не перезаписывает такой файл, и следующий wails3 build падает.
  rm -f -- bin/forktopus
fi

# sha256sum есть в Linux и Git Bash на Windows, shasum — в macOS.
sums=(sha256sum)
command -v sha256sum > /dev/null || sums=(shasum -a 256)
(cd -- "$out" && rm -f SHA256SUMS && "${sums[@]}" -- * > SHA256SUMS)

echo
echo "Готово: $out"
ls -lh -- "$out"
echo
echo "Релиз — из файлов CI: go run ./scripts/release plan (docs/release.md)."
if want macos && [[ "$host" != Darwin ]]; then
  cat <<'EOF'

.app не подписан (на Linux подписать нечем). На Mac после распаковки:
  xattr -cr forktopus.app && codesign --force --deep --sign - forktopus.app
EOF
fi
