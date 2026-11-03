#!/usr/bin/env bash
# Wails генерирует AppRun заново. Оборачиваем его после сборки и упаковываем
# штатным linuxdeploy, сохраняя relocation WebKit и runtime AppImage.
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)
build="$root/build/linux/appimage/build"
arch=$(uname -m)
appdir="$build/forktopus-$arch.AppDir"
if ! cmp -s "$appdir/AppRun" "$root/build/linux/appimage/AppRun"; then
  mv "$appdir/AppRun" "$appdir/AppRun.wails"
fi
mkdir -p "$appdir/usr/libexec"
(cd "$root" && CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags='-s -w' -o "$appdir/usr/libexec/forktopus-appimage" ./scripts/appimage)
export OUTPUT="$root/bin/forktopus-$arch.AppImage" NO_STRIP=1
"$build/linuxdeploy-$arch.AppImage" --appimage-extract-and-run --appdir "$appdir" \
  --custom-apprun "$root/build/linux/appimage/AppRun" --output appimage
