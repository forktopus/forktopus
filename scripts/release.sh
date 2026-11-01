#!/usr/bin/env bash
# Собирает файлы релиза: .deb и AppImage для Linux, .exe для Windows и
# SHA256SUMS — в bin/release/<версия>/ с именами, как на странице Releases.
#   bash scripts/release.sh 0.3.0   — сменить версию в build/ и собрать
#   bash scripts/release.sh         — собрать текущую версию
# Версия хранится в build/config.yml и ещё в десяти файлах build/ (манифесты,
# plist, nfpm) — скрипт меняет её везде. Ничего не коммитит, не ставит тег и
# не публикует: что сделать дальше — в конце вывода.
# AppImage при первой сборке скачивает linuxdeploy (кэш в build/linux/appimage/build).
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

# Файлы с версией. В MSIX она четырёхзначная (0.2.0.0), в Info.dev.plist —
# с суффиксом -dev: замена с границей слова покрывает оба случая.
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
[[ -n "$old" ]] || { echo "Не нашёл версию в build/config.yml" >&2; exit 1; }
new="${1:-$old}"
[[ "$new" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Версия — три числа через точку: 0.3.0" >&2; exit 1; }

if [[ "$new" != "$old" ]]; then
  pattern="\\b${old//./\\.}\\b"
  for f in "${FILES[@]}"; do
    grep -q "$pattern" "$f" || { echo "В $f нет версии $old — список FILES устарел" >&2; exit 1; }
  done
  sed -i "s/$pattern/$new/g" "${FILES[@]}"
  echo "Версия $old → $new"
fi

wails3 build
wails3 task linux:generate:deb
wails3 task linux:create:appimage
wails3 build GOOS=windows

# Проверка: пакет собран с нужной версией (nfpm добавляет ревизию: 0.3.0-1).
deb_version="$(dpkg-deb -f bin/forktopus.deb Version)"
[[ "${deb_version%-*}" == "$new" ]] || { echo "В .deb версия $deb_version, ожидалась $new" >&2; exit 1; }

out="bin/release/$new"
rm -rf -- "$out"
mkdir -p -- "$out"
cp bin/forktopus.deb "$out/forktopus_${new}_amd64.deb"
cp bin/forktopus-x86_64.AppImage "$out/Forktopus-${new}-x86_64.AppImage"
cp bin/forktopus.exe "$out/forktopus-${new}-windows-amd64.exe"
(cd -- "$out" && sha256sum -- * > SHA256SUMS)

echo
echo "Готово: $out"
ls -lh -- "$out"
cat <<EOF

Дальше (вручную):
  git add build && git commit -m "build: версия $new"
  git tag -a v$new -m "Forktopus $new" && git push origin main v$new
  gh release create v$new $out/* --title "Forktopus $new" --notes "…"
EOF
