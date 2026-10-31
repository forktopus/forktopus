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

## Лицензия

[MIT](LICENSE)
