#!/usr/bin/env bash
# Создаёт много Git-репозиториев для нагрузочного прогона в .local/load-repos/.
# Пишет только туда, не трогает существующие репозитории и ничего не удаляет.
# Использование: scripts/create-load-repos.sh [число репозиториев, по умолчанию 240]
set -euo pipefail

COUNT="${1:-240}"
PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
DEST="$PROJECT_ROOT/.local/load-repos"

if [[ -e "$DEST" || -L "$DEST" ]]; then
  echo "Каталог уже существует: $DEST" >&2
  echo "Скрипт не меняет существующую папку. Для нового запуска перемести её вручную." >&2
  exit 1
fi

export GIT_AUTHOR_NAME="Forktopus Load" GIT_AUTHOR_EMAIL="load@example.invalid"
export GIT_COMMITTER_NAME="$GIT_AUTHOR_NAME" GIT_COMMITTER_EMAIL="$GIT_AUTHOR_EMAIL"

# Общий remote с двумя коммитами: клоны, откатанные на один коммит, отстают.
REMOTE="$DEST/.remotes/base.git"
SEED="$DEST/.internal/seed"
mkdir -p -- "$DEST/.remotes" "$DEST/.internal"
git init -q --bare -b main "$REMOTE"
git init -q -b main "$SEED"
printf '%s\n' 'Load fixture' > "$SEED/README.md"
git -C "$SEED" add README.md
git -C "$SEED" commit -qm 'Initial load commit'
printf '%s\n' 'Second line' >> "$SEED/README.md"
git -C "$SEED" commit -qam 'Second load commit'
git -C "$SEED" remote add origin "$REMOTE"
git -C "$SEED" push -q origin main

hosts=(github.com gitlab.com git.example.org)
for ((i = 1; i <= COUNT; i++)); do
  host="${hosts[i % ${#hosts[@]}]}"
  dir="$DEST/$host/team-$((i % 12))/repo-$(printf '%03d' "$i")"
  mkdir -p -- "$(dirname -- "$dir")"
  git clone -q "$REMOTE" "$dir"
  # Разные состояния, чтобы статус читался не только у чистых копий.
  if ((i % 7 == 0)); then
    git -C "$dir" reset -q --hard HEAD~1 # отстаёт на 1
  fi
  if ((i % 10 == 0)); then
    printf '%s\n' 'Local edit' >> "$dir/README.md"
    printf '%s\n' 'Untracked' > "$dir/notes.txt"
  fi
  if ((i % 17 == 0)); then
    git -C "$dir" switch -q -c "feature-$i"
    printf '%s\n' "Feature $i" > "$dir/feature.txt"
    git -C "$dir" add feature.txt
    git -C "$dir" commit -qm "Feature $i"
  fi
  if ((i % 23 == 0)); then
    git -C "$dir" checkout -q --detach HEAD
  fi
done

cat <<OUT
Создано репозиториев: $COUNT в $DEST
Сканируй корень $DEST. .remotes/ и .internal/ — вспомогательные Git-каталоги.
OUT
