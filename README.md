# Forktopus

<img src="assets/branding/forktopus-icon-256.png" alt="Forktopus" width="128" align="right">

Локальное десктопное приложение для обзора Git-репозиториев: текущие ветки, локальные изменения, отставание от upstream, открытые PR/MR и CI-запуски на GitHub, GitLab и Gitea. Позволяет безопасно выполнять `fetch`, fast-forward `pull` и переключение веток.

Сайт: [forktopus.ru](https://forktopus.ru)

> **Статус:** 0.2.0. Проверен на Ubuntu 24.04 (GNOME, X11); на Windows 11 — локальная часть (без `gh`/`glab`/`tea`). macOS не проверялся.

Три режима на общих данных: **Реестр** (таблица с фильтрами), **Проводник** (дерево корень → сервер → владелец → репозиторий с панелью деталей) и **Радар** (карточки по группам «Требует внимания» / «В работе» / «В порядке»).

## Принципы

- Работает локально, без аккаунта, не хранит токены и пароли.
- Источник правды о репозитории — `git`. Сведения о PR/MR/CI берутся из установленных и авторизованных пользователем CLI: `gh`, `glab`, `tea`. Нет CLI — локальная часть работает как обычно.
- Никаких скрытых `reset`, `stash`, `rebase`, force-операций и автоматических коммитов.

## Что умеет

- Находит Git-репозитории в выбранных корневых папках на любой глубине (каталоги вроде `node_modules`, `vendor`, `build` пропускаются, список настраивается). Сервер определяется по `git remote`, а не по имени папки.
- Показывает ветку (или detached HEAD), изменения (staged / unstaged / untracked, конфликты, незавершённый merge/rebase), upstream и ahead/behind по последнему fetch. В Проводнике — список изменённых файлов, как в `git status --short` (до 100 на репозиторий).
- Выполняет три операции, каждую — только при выполненных предусловиях:
  - `git fetch --no-prune` — сразу; удаление устаревших remote-веток (`--prune`) включается в «Настройках»;
  - `git pull --ff-only --no-rebase --no-autostash` — после подтверждения, только на чистой ветке с upstream;
  - `git switch --no-guess` на существующую локальную ветку — после подтверждения, только на чистой рабочей копии.
- Для текущей ветки показывает открытые PR/MR (если их нет — последний закрытый или влитый: «#3 влит вчера», «#3 закрыт без слияния») и итог CI на GitHub (`gh`), GitLab (`glab`) и Gitea (`tea`); при detached HEAD — PR/MR с текущим коммитом и CI этого коммита.
- Работает в фоне: раз в интервал (по умолчанию 5 минут, от 1 до 60) сам перечитывает Git-статус и PR/CI всех репозиториев, даже когда окно скрыто; `git fetch` перед проверкой — отдельная настройка, выключена. Крестик прячет окно в трей (если трея нет — закрывает приложение); в меню значка — сводка, «Показать», «Проверить сейчас», «Выйти».
- Системные уведомления о переменах: CI текущей ветки упал или снова проходит, ветка отстала от upstream, PR/MR открыт, влит или закрыт. Больше трёх событий сразу — одно сводное; клик открывает карточку репозитория. Виды уведомлений включаются в «Настройках».
- Запуск при входе в систему (настройка, выключена): приложение стартует свёрнутым в трей (`forktopus --hidden`).
- «Диагностика CLI»: найден ли `git`/`gh`/`glab`/`tea`, авторизован ли, что выполнить в терминале; отчёт без секретов и домашнего каталога.

## Чего не делает

- Сам ничего не меняет: файлы рабочей копии меняют только подтверждённые Pull и Switch. Не пишет в `.git/config`, не создаёт и не удаляет ветки, не коммитит, не делает `reset`, `stash`, `rebase`, `merge`-коммиты, `push` и force-операции.
- Не устанавливает и не обновляет CLI, не авторизуется за пользователя, не спрашивает и не хранит токены.
- Не клонирует репозитории, не удаляет папки: «убрать корень» — только из списка.
- Не создаёт и не меняет PR/MR, не запускает CI.

## Установка

Готовые сборки — на странице [Releases](https://github.com/forktopus/forktopus/releases):

| Файл | Система | Как поставить |
|---|---|---|
| `forktopus_<версия>_amd64.deb` | Ubuntu 24.04+ / Debian 13+ | `sudo apt install ./forktopus_<версия>_amd64.deb` — подтянет GTK4 и WebKitGTK 6.0, добавит ярлык в меню |
| `Forktopus-<версия>-x86_64.AppImage` | другие Linux x86_64 | `chmod +x` и запустить; GTK4 и WebKitGTK внутри (≈128 МБ), ярлыка в меню нет |
| `forktopus-<версия>-windows-amd64.exe` | Windows 10/11 x64 | запустить; нужен WebView2 Runtime — в Windows 11 он есть всегда |

Сборки не подписаны: Windows SmartScreen предупредит о неизвестном издателе («Подробнее» → «Выполнить в любом случае»).

### Из исходников

См. «Требования для разработки» и «Быстрый старт». Для запуска собранного `bin/forktopus` нужны GTK4 и WebKitGTK 6.0:

```bash
sudo apt install libgtk-4-1 libwebkitgtk-6.0-4   # Ubuntu 24.04+
```

Ярлык в меню и значок в панели задач — для текущего пользователя, в `~/.local/share` (в GTK4 значок окна берётся только из `.desktop`):

```bash
bash scripts/install-desktop.sh            # после wails3 build
bash scripts/install-desktop.sh --remove   # убрать
```

Пакеты: `wails3 task linux:generate:deb` (упаковывает уже собранный `bin/forktopus`) и `wails3 task linux:create:appimage` (скачивает linuxdeploy) кладут `.deb` и `.AppImage` в `bin/`. Windows: `wails3 build GOOS=windows` собирает `bin/forktopus.exe` прямо на Linux (без CGO и Docker).

Файлы релиза с `SHA256SUMS` — в `bin/release/<версия>/`:

```bash
bash scripts/release.sh                                  # текущая версия: .deb, AppImage и .exe
bash scripts/release.sh --version 0.3.0                  # сменить версию во всех файлах build/ и собрать
bash scripts/release.sh --version 0.3.0 --os windows     # только .exe; --os можно повторять или через запятую
```

`--os linux` собирается только на Linux, `--os windows` — где угодно, `--os macos` — `.app` (Intel и Apple Silicon) в zip, не проверялось:

- на Linux — в Docker-образе `wails-cross` (Zig и macOS SDK внутри; один раз `wails3 task setup:docker`, образ ~4 ГБ на диске). SDK образ скачивает из стороннего репозитория, а не у Apple. `.app` получается без подписи — на Mac перед первым запуском: `xattr -cr forktopus.app && codesign --force --deep --sign - forktopus.app`;
- на macOS — нативно (нужен Xcode), с ad-hoc подписью.

Скрипт ничего не коммитит и не публикует — в конце печатает команды для коммита, тега и `gh release create`.

Файлы приложения:

| Что | Где |
|---|---|
| Настройки (корни, тема, режим, пропуски) | `~/.config/forktopus/config.json` |
| Кэш найденных репозиториев и статусов | `~/.cache/forktopus/repositories.json` |
| Лог | `~/.cache/forktopus/forktopus.log` |

На Windows настройки — в `%AppData%\forktopus\`, кэш и лог — в `%LocalAppData%\forktopus\`.

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
- память (PSS, приложение + WebKit): ~235 МБ без корней, ~345 МБ на 241 репозитории (из них WebKit — ~160 МБ: списки рисуют только видимые строки);
- переключение на реестр с 241 репозиторием — ~40–65 мс.

## Известные ограничения

- **Платформы.** Полностью проверено на Linux (GNOME, X11). На Windows 11 (VirtualBox, только git) проверены окно, системный диалог выбора папки, скан, Fetch / Pull / Switch, диагностика и сохранение настроек; PR/MR и CI на Windows не проверялись. Wayland и macOS не проверялись. Трей, уведомления и автозапуск проверены только на Linux (GNOME с расширением AppIndicator: без него значка нет, и крестик закрывает приложение); на Windows и macOS — нет. Сборки не подписаны.
- **Тип сервера** определяется по известному хосту (github.com, gitlab.com) или по логину CLI на этот хост. Если на self-hosted хосте логины у двух CLI, тип задаётся в `config.json`: `"hostProviders": {"git.example.org": "gitea"}` (интерфейса для этого нет).
- **`tea` (Gitea):**
  - нужен логин на хост remote (`tea login add`); логин по умолчанию не используется — выбирается по хосту;
  - фильтра PR по ветке в `tea` нет: открытые PR читаются постранично (до 250) и отбираются по ветке на клиенте; если PR больше — данные помечаются неполными;
  - черновик распознаётся только по префиксу `WIP:` / `[WIP]` в заголовке, открытые PR из форков не отличить;
  - последний закрытый PR ищется среди 50 недавно обновлённых закрытых PR репозитория;
  - при detached HEAD находятся только PR, у которых текущий коммит — последний;
  - CI читается через `tea api` (`/actions/runs`): на Gitea ниже 1.25 или с выключенными Actions — «не поддерживается».
- **`glab`:** без сети ошибка приходит примерно через 12 с; self-hosted GitLab с подпутём в URL не проверялся.
- **`gh`:** в PR учитываются только PR из того же репозитория, без форков.
- **Окно.** На X11 размер окна по умолчанию иногда не применяется. Уже ~1170 px таблица реестра прокручивается по горизонтали.

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
| Форматирование фронтенда (перед коммитом) | хост | `npm --prefix frontend run format`; проверка — `npm --prefix frontend run format:check` |
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
