# Forktopus

<img src="assets/branding/forktopus-icon-256.png" alt="Forktopus" width="128" align="right">

Локальное десктопное приложение для обзора множества Git-репозиториев: текущие ветки, локальные изменения, отставание от upstream, открытые PR/MR и CI-запуски на GitHub, GitLab и Gitea. Позволяет безопасно выполнять `fetch`, fast-forward `pull` и переключение веток.

Сайт: [forktopus.ru](https://forktopus.ru)

> **Статус:** MVP в разработке. Сейчас в репозитории — шаблон Wails 3, функциональности приложения ещё нет.

## Принципы

- Работает локально, без аккаунта, не хранит токены и пароли.
- Источник правды о репозитории — `git`. Сведения о PR/MR/CI берутся из установленных и авторизованных пользователем CLI: `gh`, `glab`, `tea`. Нет CLI — локальная часть работает как обычно.
- Никаких скрытых `reset`, `stash`, `rebase`, force-операций и автоматических коммитов.

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

Замечания:

- `wails3 dev` использует порт 9245, контейнерный `ui` — 5173, их можно запускать одновременно.
- `node_modules` общий для хоста и контейнера (оба linux/amd64). На macOS/Windows перед `wails3 dev` выполнить `npm ci` на хосте.
- Фикстуры создаются **на хосте**: скрипт прописывает remote абсолютным путём, и созданные в контейнере (`/workspace/...`) репозитории на хосте будут со сломанным remote. Повторный запуск не перезаписывает существующую папку — сначала удалить `.local/git-fixtures/` вручную.
- `frontend/bindings/` генерируется Wails (`wails3 generate bindings`, а также при `dev`/`build`) и хранится в репозитории, чтобы фронтенд собирался без wails3.

## Лицензия

[MIT](LICENSE)
