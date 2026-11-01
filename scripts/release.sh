#!/usr/bin/env bash
# Собирает файлы релиза с именами, как на странице Releases, и SHA256SUMS
# в bin/release/<версия>/:
#   linux   — forktopus_<v>_amd64.deb и Forktopus-<v>-x86_64.AppImage (только на Linux);
#   windows — forktopus-<v>-windows-amd64.exe (на любой ОС: сборка без CGO);
#   macos   — Forktopus-<v>-macos-universal.zip с .app (Intel и Apple Silicon); на macOS —
#             нативно с ad-hoc подписью, на Linux — в Docker-образе wails-cross без
#             подписи (один раз: wails3 task setup:docker). Не проверялось.
#
#   bash scripts/release.sh [--version X.Y.Z] [--os linux|windows|macos ...]
#
#   --version  сменить версию во всех файлах build/ и собрать её; без флага —
#              текущая из build/config.yml
#   --os       что собирать: можно повторять или через запятую (--os linux,windows);
#              без флага — linux и windows
# Без флагов: текущая версия, Linux и Windows.
#
# Версия хранится в build/config.yml и ещё в десяти файлах build/ (манифесты,
# plist, nfpm) — скрипт меняет её везде. Ничего не коммитит, не ставит тег и
# не публикует: что сделать дальше — в конце вывода. Файлы других ОС той же
# версии в bin/release/<версия>/ остаются, SHA256SUMS считается по всем.
# AppImage при первой сборке скачивает linuxdeploy (кэш в build/linux/appimage/build).
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

usage() {
  sed -n '10,16p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

die() {
  echo "$*" >&2
  exit 1
}

new=""
oses=()
while (($#)); do
  case "$1" in
    --version) (($# >= 2)) || die "--version: нужна версия"; new="$2"; shift 2 ;;
    --version=*) new="${1#*=}"; shift ;;
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

# Файлы с версией. В MSIX она четырёхзначная (0.2.0.0), в Info.dev.plist —
# с суффиксом -dev: замена с границей слова покрывает оба случая. Замена —
# perl, а не sed: у sed на macOS нет \b.
FILES=(
  build/config.yml
  build/linux/nfpm/nfpm.yaml
  build/windows/info.json
  build/windows/wails.exe.manifest
  build/windows/nsis/wails_tools.nsh
  build/windows/msix/app_manifest.xml
  build/windows/msix/template.xml
  build/darwin/Info.plist
  build/darwin/Info.dev.plist
  build/ios/Info.plist
  build/ios/Info.dev.plist
)

old="$(sed -n 's/^  version: "\([0-9.]*\)".*/\1/p' build/config.yml)"
[[ -n "$old" ]] || die "Не нашёл версию в build/config.yml"
new="${new:-$old}"
[[ "$new" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "Версия — три числа через точку: 0.3.0"

if [[ "$new" != "$old" ]]; then
  for f in "${FILES[@]}"; do
    grep -qwF -- "$old" "$f" || die "В $f нет версии $old — список FILES устарел"
  done
  OLD="$old" NEW="$new" perl -pi -e 's/\b\Q$ENV{OLD}\E\b/$ENV{NEW}/g' "${FILES[@]}"
  echo "Версия $old → $new"
fi

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

(cd -- "$out" && rm -f SHA256SUMS && shasum -a 256 -- * > SHA256SUMS)

echo
echo "Готово: $out"
ls -lh -- "$out"
echo
echo "Дальше (вручную):"
[[ "$new" != "$old" ]] && echo "  git add build && git commit -m \"build: версия $new\""
cat <<EOF
  git tag -a v$new -m "Forktopus $new" && git push origin main v$new
  gh release create v$new $out/* --title "Forktopus $new" --notes "…"
EOF
if want macos && [[ "$host" != Darwin ]]; then
  cat <<'EOF'

.app не подписан (на Linux подписать нечем). На Mac после распаковки:
  xattr -cr forktopus.app && codesign --force --deep --sign - forktopus.app
EOF
fi
