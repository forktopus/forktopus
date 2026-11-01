# Forktopus

<img src="assets/branding/forktopus-icon-256.png" alt="Forktopus" width="128" align="right">

Локальное десктопное приложение для обзора множества Git-репозиториев: текущие ветки, локальные изменения, отставание от upstream, открытые PR/MR и CI-запуски на GitHub, GitLab и Gitea. Позволяет безопасно выполнять `fetch`, fast-forward `pull` и переключение веток.

Сайт: [forktopus.ru](https://forktopus.ru)

> **Статус:** MVP (0.1.0). Проверен на Ubuntu 24.04 (GNOME, X11). Windows и macOS не проверялись.

Три режима на общих данных: **Реестр** (таблица с фильтрами), **Проводник** (дерево корень → сервер → владелец → репозиторий с панелью деталей) и **Радар** (карточки по группам «Требует внимания» / «В работе» / «В порядке»).

## Принципы

- Работает локально, без аккаунта, не хранит токены и пароли.
- Источник правды о репозитории — `git`. Сведения о PR/MR/CI берутся из установленных и авторизованных пользователем CLI: `gh`, `glab`, `tea`. Нет CLI — локальная часть работает как обычно.
- Никаких скрытых `reset`, `stash`, `rebase`, force-операций и автоматических коммитов.

## Что умеет

- Находит Git-репозитории в выбранных корневых папках на любой глубине (каталоги вроде `node_modules`, `vendor`, `build` пропускаются, список настраивается). Сервер определяется по `git remote`, а не по имени папки.
- Показывает ветку (или detached HEAD), изменения (staged / unstaged / untracked, конфликты, незавершённый merge/rebase), upstream и ahead/behind по последнему fetch.
- Выполняет три операции, каждую — только при выполненных предусловиях:
  - `git fetch --no-prune` — сразу;
  - `git pull --ff-only --no-rebase --no-autostash` — после подтверждения, только на чистой ветке с upstream;
  - `git switch --no-guess` на существующую локальную ветку — после подтверждения, только на чистой рабочей копии.
- Для текущей ветки показывает открытые PR/MR и итог CI на GitHub (`gh`), GitLab (`glab`) и Gitea (`tea`); данные обновляются раз в 2 минуты.
- «Диагностика CLI»: найден ли `git`/`gh`/`glab`/`tea`, авторизован ли, что выполнить в терминале; отчёт без секретов и домашнего каталога.

## Чего не делает

- Сам ничего не меняет: файлы рабочей копии меняют только подтверждённые Pull и Switch. Не пишет в `.git/config`, не создаёт и не удаляет ветки, не коммитит, не делает `reset`, `stash`, `rebase`, `merge`-коммиты, `push` и force-операции.
- Не устанавливает и не обновляет CLI, не авторизуется за пользователя, не спрашивает и не хранит токены.
- Не клонирует репозитории, не удаляет папки: «убрать корень» — только из списка.
- Не создаёт и не меняет PR/MR, не запускает CI.

## Установка и запуск (Linux)

Готовых пакетов нет — приложение собирается из исходников (см. «Требования для разработки» и «Быстрый старт»). Для запуска собранного `bin/forktopus` нужны GTK4 и WebKitGTK 6.0:

```bash
sudo apt install libgtk-4-1 libwebkitgtk-6.0-4   # Ubuntu 24.04+
```

Ярлык в меню и значок в панели задач — для текущего пользователя, в `~/.local/share` (в GTK4 значок окна берётся только из `.desktop`):

```bash
bash scripts/install-desktop.sh            # после wails3 build
bash scripts/install-desktop.sh --remove   # убрать
```

Файлы приложения:

| Что | Где |
|---|---|
| Настройки (корни, тема, режим, пропуски) | `~/.config/forktopus/config.json` |
| Кэш найденных репозиториев и статусов | `~/.cache/forktopus/repositories.json` |
| Лог | `~/.cache/forktopus/forktopus.log` |

Внешние CLI нужны только для PR/MR и CI; каждый должен быть установлен и авторизован пользователем (`gh auth login`, `glab auth login`, `tea login add`).

| CLI | Проверенная версия | Сервер |
|---|---|---|
| git | 2.43.0 | — |
| gh | 2.103.3 | github.com |
| glab | 1.126.0 | gitlab.com |
| tea | 0.16.0 | Gitea 1.24–1.26 (CI — с 1.25) |

Подробности поведения CLI — [docs/providers.md](docs/providers.md).

## Ресурсы

Замеры на Ubuntu 24.04, production-сборка:

- бинарник — около 10 МиБ (`-tags production -trimpath -ldflags "-w -s"`);
- окно появляется за ~0,25 с;
- первый скан 241 репозитория — ~1 с;
- память (PSS, приложение + WebKit): ~235 МБ без корней, ~0,6 ГБ на 241 репозитории (из них WebKit — ~450 МБ).

## Известные ограничения

- **Платформы.** Проверено только на Linux (GNOME, X11). Wayland, Windows и macOS не проверялись; иконки для Windows и macOS подготовлены, инсталляторов нет.
- **Тип сервера** определяется по известному хосту (github.com, gitlab.com) или по логину CLI на этот хост. Если на self-hosted хосте логины у двух CLI, тип задаётся в `config.json`: `"hostProviders": {"git.example.org": "gitea"}` (интерфейса для этого нет).
- **`tea` (Gitea):**
  - нужен логин на хост remote (`tea login add`); логин по умолчанию не используется — выбирается по хосту;
  - фильтра PR по ветке в `tea` нет: открытые PR читаются постранично (до 250) и отбираются по ветке на клиенте; если PR больше — данные помечаются неполными;
  - черновик распознаётся только по префиксу `WIP:` / `[WIP]` в заголовке, PR из форков не отличить;
  - CI читается через `tea api` (`/actions/runs`): на Gitea ниже 1.25 или с выключенными Actions — «не поддерживается».
- **`glab`:** без сети ошибка приходит примерно через 12 с; self-hosted GitLab с подпутём в URL не проверялся.
- **`gh`:** в PR учитываются только PR из того же репозитория, без форков.
- **Fetch** не удаляет устаревшие remote-ветки (`--no-prune`).
- **Окно.** На X11 размер окна по умолчанию иногда не применяется. На сотнях репозиториев списки не виртуализированы — WebKit занимает заметно больше памяти.

## Стек

Go + [Wails 3](https://v3.wails.io/) (системный WebView), React + TypeScript + Vite.

## Требования для разработки

| Компонент | Версия |
|---|---|
| Go | 1.26+ |
| Wails CLI | v3.0.0 (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`) |
| Node.js / npm | 24 / 11 |
| Linux | GTK4 и WebKitGTK 6.0: `sudo apt install build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev` (Ubuntu 24.04+) |

Проверка окружения: `wails3 doctor`.

## Быстрый старт

```bash
cd frontend && npm ci && cd ..
wails3 dev     # нативное окно с hot reload
wails3 build   # бинарник в bin/
```

## Разработка

Два контура:

- **Хост** — всё, что требует нативного окна, системных диалогов и настоящих `git`/`gh`/`glab`/`tea`: `wails3 dev`, `wails3 build`, Git-фикстуры.
- **Docker** (`compose.dev.yaml`) — воспроизводимые проверки без установки Go/Node: `npm ci`, сборка фронтенда, Go-тесты, Vite-превью на моковых данных. Контейнер видит только каталог проекта; GTK/WebKit в нём нет, поэтому Go собирается с `-tags server` и `CGO_ENABLED=0`.

### Подготовка Docker

```bash
cp .env.dev.example .env.dev   # при необходимости поправить LOCAL_UID/LOCAL_GID (id -u, id -g)
docker compose --env-file .env.dev -f compose.dev.yaml build toolbox
```

### Команды

| Что | Где | Команда |
|---|---|---|
| Нативное окно с hot reload | хост | `wails3 dev` |
| Production-сборка | хост | `wails3 build` → `bin/forktopus` |
| Зависимости фронтенда | Docker | `docker compose --env-file .env.dev -f compose.dev.yaml run --rm toolbox sh -lc 'cd frontend && npm ci'` |
| Сборка фронтенда | Docker | `docker compose --env-file .env.dev -f compose.dev.yaml run --rm toolbox sh -lc 'cd frontend && npm run build'` |
| Go-тесты | Docker | `docker compose --env-file .env.dev -f compose.dev.yaml run --rm toolbox sh -lc 'go test -tags server ./...'` |
| Go-тесты | хост | `go test ./...` |
| Vite-превью (моки) | Docker | `docker compose --env-file .env.dev -f compose.dev.yaml up ui` → http://127.0.0.1:5173 |
| Git-фикстуры | хост | `bash scripts/create-demo-repos.sh` → `.local/git-fixtures/` |
| Иконка из маскота | хост | `bash scripts/make-icon.sh && wails3 task common:generate:icons` |

Замечания:

- `wails3 dev` использует порт 9245, контейнерный `ui` — 5173, их можно запускать одновременно.
- `node_modules` общий для хоста и контейнера (оба linux/amd64). На macOS/Windows перед `wails3 dev` выполнить `npm ci` на хосте.
- Фикстуры создаются **на хосте**: скрипт прописывает remote абсолютным путём, и созданные в контейнере (`/workspace/...`) репозитории на хосте будут со сломанным remote. Повторный запуск не перезаписывает существующую папку — сначала удалить `.local/git-fixtures/` вручную.
- `frontend/bindings/` генерируется Wails (`wails3 generate bindings`, а также при `dev`/`build`) и хранится в репозитории, чтобы фронтенд собирался без wails3.

## Лицензия

[MIT](LICENSE)
