#!/usr/bin/env bash
# Создаёт изолированные Git-фикстуры в .local/git-fixtures/.
# Не трогает существующие репозитории пользователя и ничего не удаляет.
set -euo pipefail

PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
DEST="$PROJECT_ROOT/.local/git-fixtures"

# Не продолжаем, если папка уже есть: таким образом исключаем перезапись данных.
if [[ -e "$DEST" || -L "$DEST" ]]; then
  echo "Фикстуры уже существуют: $DEST" >&2
  echo "Скрипт не меняет существующую папку. Для нового запуска перемести её вручную." >&2
  exit 1
fi

mkdir -p -- "$DEST"

init_repo() {
  local dir="$1"
  local text="$2"
  mkdir -p -- "$dir"
  git init -q -b main "$dir"
  git -C "$dir" config user.name "Forktopus Fixture"
  git -C "$dir" config user.email "fixtures@example.invalid"
  printf '%s\n' "$text" > "$dir/README.md"
  git -C "$dir" add README.md
  git -C "$dir" commit -qm 'Initial fixture commit'
}

# 1. Чистый репозиторий без upstream.
CLEAN="$DEST/github.com/example/demo-clean"
init_repo "$CLEAN" 'Clean fixture'

# 2. staged + unstaged + untracked одновременно.
DIRTY="$DEST/github.com/example/demo-dirty"
init_repo "$DIRTY" 'Dirty fixture'
printf '%s\n' 'Local edit' >> "$DIRTY/README.md"
printf '%s\n' 'Index edit' > "$DIRTY/staged.txt"
git -C "$DIRTY" add staged.txt
printf '%s\n' 'Untracked' > "$DIRTY/untracked.txt"

# 3. Локальная ветка на один коммит впереди upstream.
REMOTE="$DEST/.remotes/demo.git"
mkdir -p -- "$(dirname -- "$REMOTE")"
git init -q --bare "$REMOTE"
git --git-dir="$REMOTE" symbolic-ref HEAD refs/heads/main

AHEAD="$DEST/gitlab.com/team/demo-ahead"
init_repo "$AHEAD" 'Ahead fixture'
git -C "$AHEAD" remote add origin "$REMOTE"
git -C "$AHEAD" push -q -u origin main
printf '%s\n' 'One local commit' >> "$AHEAD/README.md"
git -C "$AHEAD" add README.md
git -C "$AHEAD" commit -qm 'Local commit ahead of origin'

# 4. Ещё одна копия на один коммит позади того же remote.
BEHIND="$DEST/gitlab.com/team/demo-behind"
git clone -q "$REMOTE" "$BEHIND"
git -C "$BEHIND" config user.name "Forktopus Fixture"
git -C "$BEHIND" config user.email "fixtures@example.invalid"

WRITER="$DEST/.internal/writer"
mkdir -p -- "$(dirname -- "$WRITER")"
git clone -q "$REMOTE" "$WRITER"
git -C "$WRITER" config user.name "Forktopus Fixture"
git -C "$WRITER" config user.email "fixtures@example.invalid"
printf '%s\n' 'Remote commit' >> "$WRITER/README.md"
git -C "$WRITER" add README.md
git -C "$WRITER" commit -qm 'Remote commit ahead of local clone'
git -C "$WRITER" push -q origin main
git -C "$BEHIND" fetch -q origin

# 5. Detached HEAD.
DETACHED="$DEST/gitea.local/team/demo-detached"
init_repo "$DETACHED" 'Detached HEAD fixture'
git -C "$DETACHED" checkout -q --detach HEAD

# 6. Дополнительный worktree: внутри лежит .git ФАЙЛ, а не папка.
WORKTREE="$DEST/github.com/example/demo-worktree"
git -C "$CLEAN" worktree add -q -b demo-worktree "$WORKTREE"

# 7. Синтаксис remote self-hosted (без сетевого подключения).
HOSTED="$DEST/gitea.local/team/demo-self-hosted"
init_repo "$HOSTED" 'Self-hosted remote fixture'
git -C "$HOSTED" remote add origin 'ssh://git@gitea.local:2222/team/demo-self-hosted.git'

cat <<OUT
Созданы Git-фикстуры: $DEST
  clean       : $CLEAN
  dirty       : $DIRTY
  ahead       : $AHEAD
  behind      : $BEHIND
  detached    : $DETACHED
  worktree    : $WORKTREE
  self-hosted : $HOSTED

Сканируй корень $DEST. Фикстуры не содержат реальных токенов и ключей.
ВНИМАНИЕ: .remotes/ и .internal/ — вспомогательные Git-каталоги.
OUT
