#!/usr/bin/env bash
# Собирает build/appicon.png из маскота: 1024×1024, белый фон → прозрачность,
# логотип интерфейса frontend/public/forktopus-icon.png (256×256) и значок
# трея internal/desktop/trayicon.png (64×64) из него же.
# Исходник assets/branding/forktopus-mascot.png не меняется (D-052).
#   bash scripts/make-icon.sh && wails3 task common:generate:icons
# Фон — области, близкие к белому и связанные с углами, плюс замкнутые
# «карманы» фона между щупальцами и узлами и полоса тени под нижним узлом
# (точки-затравки в координатах 1024×1024).
# Там прозрачность считается от белого (мягкая тень остаётся полупрозрачной),
# внутри фигуры пиксели не меняются — белки глаз и узлы остаются белыми.
set -euo pipefail

PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
SRC="$PROJECT_ROOT/assets/branding/forktopus-mascot.png"
OUT="$PROJECT_ROOT/build/appicon.png"
UI="$PROJECT_ROOT/frontend/public/forktopus-icon.png"
TRAY="$PROJECT_ROOT/internal/desktop/trayicon.png"
FUZZ=25%
command -v convert >/dev/null || { echo "Нужен ImageMagick (convert)" >&2; exit 1; }

T=$(mktemp -d)
trap 'rm -rf -- "$T"' EXIT
convert "$SRC" -filter Lanczos -resize 1024x1024 -alpha off "$T/src.png"
convert "$T/src.png" -fuzz "$FUZZ" -fill '#ff00ff' \
  -draw 'color 0,0 floodfill' -draw 'color 1023,0 floodfill' -draw 'color 0,1023 floodfill' -draw 'color 1023,1023 floodfill' \
  -draw 'color 149,500 floodfill' -draw 'color 715,629 floodfill' -draw 'color 516,668 floodfill' \
  -draw 'color 265,538 floodfill' -draw 'color 410,628 floodfill' -draw 'color 371,702 floodfill' -draw 'color 594,830 floodfill' \
  -fuzz 0 -fill black +opaque '#ff00ff' -fill white -opaque '#ff00ff' "$T/bg.png"
# Фигура без полосы сглаживания в 2 px — её цвет тоже пересчитывается от белого.
convert "$T/bg.png" -morphology Dilate Disk:2 -negate "$T/inner.png"
# Color-to-alpha: A = max(1-R, 1-G, 1-B), цвет = 1 - (1-C)/A.
convert "$T/src.png" -fx 'max(max(1-r,1-g),1-b)' -colorspace Gray "$T/a.png"
convert "$T/src.png" "$T/a.png" -fx 'v.r<0.02 ? 0 : max(0,1-(1-u)/v.r)' "$T/col.png"
convert "$T/a.png" -fx 'u<0.02 ? 0 : u' "$T/a2.png"
convert "$T/a2.png" "$T/inner.png" -compose Lighten -composite "$T/alpha.png"
convert "$T/col.png" "$T/src.png" "$T/inner.png" -composite "$T/rgb.png"
convert "$T/rgb.png" "$T/alpha.png" -alpha off -compose CopyOpacity -composite -strip "$OUT"
convert "$OUT" -filter Lanczos -resize 256x256 -strip "$UI"
convert "$OUT" -filter Lanczos -resize 64x64 -strip "$TRAY"
echo "Готово: build/appicon.png, frontend/public/forktopus-icon.png, internal/desktop/trayicon.png"
