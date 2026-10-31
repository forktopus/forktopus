# Провайдеры: результаты пилота CLI

Что фактически отдают `gh`, `glab` и `tea` для PR/MR и CI по ветке. По этим находкам пишутся адаптеры (этап 08); при обновлении CLI — перепроверить.

Пилотные репозитории — `OWNER/forktopus-pilot` на каждом сервере: workflow `ok` (успех), `fail` (падает, только ветка `pilot/fail`), `slow` (долгий, только `pilot/slow`); ветки `pilot/pr-open` (открытый PR), `pilot/pr-draft` (черновик), `pilot/closed-pr` (закрытый PR), `pilot/no-pr`.

## GitHub — `gh`

Проверено: `gh` 2.103.3, github.com, 2026-10-31.

### Авторизация

```sh
gh auth status --hostname github.com
```

| Ситуация | Код | stderr / stdout |
|---|---|---|
| Залогинен | 0 | `✓ Logged in to github.com account …` |
| Нет ни одного логина | 1 | `You are not logged into any GitHub hosts. To log in, run: gh auth login` |
| Неверный `GH_TOKEN` | 1 | `X Failed to log in to github.com using token (GH_TOKEN)` … `The token in GH_TOKEN is invalid.` |
| **Нет сети** | 1 | `X Failed to log in to github.com account … (keyring)` … `The token in keyring is invalid.` |

Важно: без сети `auth status` сообщает «токен недействителен», хотя токен в порядке. Отличить «нет авторизации» от «нет сети» по одному `auth status` нельзя — классифицировать по ошибке рабочей команды (ниже).

С `--json hosts` (`gh auth status --hostname HOST --json hosts`) код всегда 0, а различать можно: нет логина — `{"hosts":{}}`; есть — `hosts[HOST][]` с `state` (`success` / `error`), `active` и текстом `error`; без сети в `error` — сетевая ошибка (`dial tcp: lookup …`), а не «token invalid». Адаптер берёт этот вариант. Вывод `auth status` содержит логин, маскированный токен и scopes — в лог и UI целиком не выводить.

### Открытые PR, где ветка — источник

```sh
gh pr list -R OWNER/REPO --head BRANCH --state open --limit 30 \
  --json number,isDraft,state,url,headRefName,headRefOid,baseRefName,isCrossRepository,headRepositoryOwner,updatedAt
```

- Ответ — JSON-массив; нет PR — `[]` и код 0 (это достоверный ноль).
- `isDraft: true` — черновик; черновики входят в `--state open`. `state` — `OPEN` / `CLOSED` / `MERGED` (верхний регистр).
- Закрытый PR при `--state open` не попадает в ответ (проверено на `pilot/closed-pr`).
- `--head` фильтрует **только по имени ветки** (синтаксис `owner:branch` не поддерживается): PR из форка с тем же именем ветки тоже попадёт. Свои PR отличать по `isCrossRepository: false` или `headRepositoryOwner.login` = владелец выбранного remote (idea 04 §7).
- `-R` принимает и `HOST/OWNER/REPO` (`github.com/OWNER/REPO`) — пригодится для GitHub Enterprise.
- Запрос идёт в GraphQL, ~0,5 с.

Пример (сокращён):

```json
[{"number":2,"isDraft":true,"state":"OPEN","headRefName":"pilot/pr-draft","baseRefName":"main",
  "isCrossRepository":false,"headRepositoryOwner":{"login":"OWNER"},"url":"https://github.com/OWNER/forktopus-pilot/pull/2"}]
```

### CI-запуски ветки

```sh
gh run list -R OWNER/REPO --branch BRANCH --limit 20 \
  --json databaseId,status,conclusion,workflowName,event,headBranch,headSha,url,createdAt,updatedAt,attempt
```

- Ответ — JSON-массив, новые первыми (`createdAt` по убыванию); нет запусков — `[]` и код 0.
- В выдачу попадают запуски всех событий для этой ветки: `push`, `pull_request` (у PR `headBranch` — ветка-источник), `workflow_dispatch`.
- На один коммит обычно несколько запусков (по workflow) — итог считать по всем запускам последнего `headSha`, а не по одному верхнему.
- Наблюдались `status`: `queued`, `in_progress`, `completed`. По `gh run list --help` возможны ещё `requested`, `waiting`, `pending` — считать активными вместе с `queued` / `in_progress`.
- `conclusion` пустая строка, пока запуск не завершён. Наблюдались: `success`, `failure`, `cancelled`. По справке возможны ещё `neutral`, `skipped`, `stale`, `startup_failure`, `timed_out`, `action_required`.
- Запрос идёт в REST (`/repos/OWNER/REPO/actions/runs?branch=…`), ~0,7 с.

### Ошибки

| Ситуация | Код | stderr | Код ядра |
|---|---|---|---|
| Нет логина вообще | **4** | `To get started with GitHub CLI, please run:  gh auth login` | `AUTH_REQUIRED` |
| Неверный токен | 1 | `HTTP 401: Bad credentials (https://api.github.com/graphql)` | `AUTH_REQUIRED` |
| Нет сети | 1 | `error connecting to api.github.com` … `check your internet connection` | `NETWORK_ERROR` |
| Нет репозитория или доступа (PR) | 1 | `GraphQL: Could not resolve to a Repository with the name 'OWNER/REPO'. (repository)` | `REPO_UNRESOLVED` |
| Нет репозитория или доступа (CI) | 1 | `failed to get runs: HTTP 404: Not Found (…)` | `REPO_UNRESOLVED` |
| Неизвестный хост в `-R` | 1 | `error connecting to HOST` … | `NETWORK_ERROR` |
| Actions не запускаются из-за биллинга | 0 | запуски сразу `completed` / `failure`; причина видна только в аннотации (`gh run view`): «The job was not started because recent account payments have failed…» | — (это настоящий `failure`) |

- Переменные `HTTPS_PROXY` / `https_proxy` `gh` для GitHub не использует (запрос через закрытый прокси прошёл) — «нет сети» проверено отдельным сетевым namespace (`unshare -rn`).
- Лимиты: 5000 запросов в час и для REST, и для GraphQL (`gh api rate_limit`). Превышение не воспроизводилось; по документации GitHub — HTTP 403/429 с `API rate limit exceeded` → `RATE_LIMIT`.
- Без сети ошибка приходит мгновенно (не по таймауту).

## GitLab — `glab`

Проверено: `glab` 1.126.0, gitlab.com, 2026-10-31. Проект приватный, shared runners gitlab.com работают.

### Авторизация

```sh
glab auth status --hostname gitlab.com
```

| Ситуация | Код | Вывод (stderr) |
|---|---|---|
| Залогинен | 0 | `✓ Logged in to gitlab.com as …` и строки про протоколы и API |
| Токен недействителен | 1 | `x gitlab.com: API call failed: GET https://gitlab.com/api/v4/user: 401 {message: 401 Unauthorized}` |
| Нет сети | 1 | `x gitlab.com: API call failed: Get "…/api/v4/user": dial tcp: lookup gitlab.com …: network is unreachable` |

- В отличие от `gh`, без сети `auth status` честно показывает сетевую ошибку (`dial tcp`, `lookup`), а не «токен недействителен».
- Машинный вариант — `glab auth status --hostname HOST --output json`: `{"hosts":[{"host","state","error","api_endpoint",…}]}`. Если логин не прошёл проверку (`state: error`), код выхода **1**, но JSON печатается; без сети в `error` — сетевая ошибка, при отозванном токене — `401 Unauthorized`. Хоста без логина — код 1 и `{"error":{"message":"x HOST has not been authenticated with glab; …"}}` в stdout. Адаптер берёт этот вариант.
- Чистое «логина нет» без выхода из аккаунта воспроизвести не удалось: пустой `GLAB_CONFIG_DIR` всё равно берёт токен из keyring, но теряет признак OAuth, и сервер отвечает 401. Для адаптера это одно и то же — `AUTH_REQUIRED`.

### Открытые MR, где ветка — источник

```sh
glab mr list -R OWNER/REPO --source-branch BRANCH -F json
```

- По умолчанию только открытые (`state=opened`); закрытый MR не попадает (проверено), `--all` / `--closed` / `--merged` — другие состояния.
- Нет MR — `[]` и код 0.
- Черновик — `draft: true` (`work_in_progress` в ответе пустое).
- `state` — `opened` / `closed` / `merged` / `locked` (нижний регистр).
- MR из форка: `source_project_id` ≠ `target_project_id` — по ним отличать свои MR (idea 04 §7).
- Нужные поля: `iid`, `state`, `draft`, `source_branch`, `target_branch`, `source_project_id`, `target_project_id`, `sha`, `web_url`, `updated_at`. Ответ — полный объект REST API, список полей не выбирается.
- `-R` принимает `OWNER/REPO`, `GROUP/SUBGROUP/REPO`, `HOST/OWNER/REPO`, URL `https://…` и `git@…:….git` — проверено на gitlab.com. Self-hosted с подпутём (`https://host/gitlab/group/repo`) не проверялся: проверить на 08.6 (возможно, понадобится `GITLAB_HOST`).

### Pipelines ветки

```sh
glab ci list -R OWNER/REPO --ref BRANCH -F json
```

- Новые первыми (сортировка `id desc`); нет pipelines — `[]` и код 0.
- Поля: `id`, `iid`, `status`, `source`, `ref`, `sha`, `web_url`, `created_at`, `updated_at`, `name`, `project_id`.
- Наблюдались `status`: `created` (сразу после запуска), `running`, `success`, `failed`, `canceling`, `canceled`. По `glab ci list --help` возможны ещё `pending`, `skipped`, `manual`, `waiting_for_resource`, `preparing`, `scheduled`. Активные — `created`, `waiting_for_resource`, `preparing`, `pending`, `running`, `canceling`; `manual` и `scheduled` ждут человека или времени — не активные.
- В отличие от GitHub, итог pipeline — одно поле `status` (без `conclusion`); один pipeline на коммит ветки.
- Pipelines для MR (`source=merge_request_event`) имеют `ref` вида `refs/merge-requests/N/head` и в `--ref BRANCH` не попадают. В пилоте их нет (job'ы без `rules` для MR не создают MR-pipelines).

### Ошибки

| Ситуация | Код | Вывод | Код ядра |
|---|---|---|---|
| Токен недействителен / нет логина | 1 | `… 401 {message: 401 Unauthorized}` | `AUTH_REQUIRED` |
| Нет сети | 1 | `dial tcp: lookup gitlab.com …: network is unreachable` | `NETWORK_ERROR` |
| Нет проекта или доступа | 1 | `404 Not Found` | `REPO_UNRESOLVED` |

- Текст ошибки в stderr glab печатает в рамке `ERROR` и **переносит по ширине ~120 символов** (посреди URL и слов, `COLUMNS` не влияет) — перед разбором склеивать строки.
- `glab mr list -F json` при ошибке пишет JSON `{"error":{"message":"…"}}` **в stdout** (и текст в stderr); `glab ci list` — только в stderr. Разбирать stdout как список только при коде 0.
- Сообщения содержат URL API с путём проекта, но без токена.
- **Без сети ошибка приходит через ~12 с** (повторы DNS), а не мгновенно, — учесть в таймаутах и в UI (показывать «загрузка»).
- Лимиты: в заголовках ответа `Ratelimit-Name: burst_traffic_per_user_free_plan`, `Ratelimit-Limit: 100`. Превышение не воспроизводилось; по документации GitLab — HTTP 429 → `RATE_LIMIT`.

## Gitea — `tea`

Проверено: `tea` 0.16.0, Gitea 1.25.5 и 1.26.2 (git.alekseyyarkov.ru), 2026-10-31. Раннеры на инстансе онлайн, workflow из `.gitea/workflows/` выполняются.

### Общее

- **Каждая команда `tea` требует, чтобы рабочий каталог был git-репозиторием** — даже с `--login` и `--repo` (иначе `Error: git rev-parse --show-toplevel: fatal: …`). Запускать с `Dir` = путь репозитория.
- Логин по умолчанию может быть не задан (у пользователя оба логина `default: false`). Логин выбирать явно (`-l NAME`) по хосту remote: `tea logins list -o json` → `[{"name","url","default",…}]`; `default` — строка `"true"`/`"false"`.
- Без `-l` внутри репозитория `tea` сам выбирает логин по хосту remote — сработало, но полагаться на это не стоит при нескольких логинах на один хост.
- `tea whoami` не принимает `-l` — для проверки авторизации конкретного логина не годится.
- `tea --version` → `Version: 0.16.0\tgolang: …` с ANSI-кодами вокруг номера даже при `NO_COLOR` — номер выделять регулярным выражением.
- `tea repos create -o json` печатает не чистый JSON — не использовать как образец машинного вывода.

### Открытые PR ветки

```sh
tea pulls list -l LOGIN -r OWNER/REPO --state open --limit 50 --page N \
  -f index,state,head,base,url,title,updated,ci -o json
```

- **Фильтра по head-ветке нет** — отбирать на клиенте по `head` == ветка, читая страницы, пока страница полная (D-040).
- Все значения в JSON — **строки** (`"index": "2"`).
- `state` — `open` / `closed` (нижний регистр); закрытый PR при `--state open` не попадает.
- **Черновик** — только префикс заголовка `WIP:` (так его ставит `tea pulls create --draft`; Gitea по умолчанию понимает `WIP:` и `[WIP]`). Отдельного поля нет.
- `ci` — сводный статус коммитов head PR (`pending` / `success` / `failure` …).
- PR из форка по выводу `tea` не отличить (`head` — только имя ветки).

### CI-запуски ветки

```sh
tea api -l LOGIN -i "/repos/OWNER/REPO/actions/runs?branch=BRANCH&limit=20"
```

- Брать REST через `tea api`, а не `tea actions runs list` (D-041): в JSON `tea actions runs list -o json` нет `conclusion`, `head_sha` и URL (только `id`, `status`, `workflow`, `branch`, `event`, `started`, `duration`), а `tea actions runs view N` печатает итог только текстом. К тому же `tea actions runs` работает лишь с Gitea ≥ 1.26 (на 1.25.5 — `Error: gitea server at https://… is older than 1.26.0`, код 1), а REST-эндпоинт есть и на 1.25.5 — проверено на обеих версиях.
- Ответ в формате GitHub: `{"workflow_runs":[…],"total_count":N}`, новые первыми; `total_count` — всего запусков ветки, а не на странице. Нет запусков — `{"workflow_runs":[],"total_count":0}`.
- Поля запуска: `id`, `status`, `conclusion`, `display_title` (заголовок коммита, а не имя workflow), `path` (`ok.yml@refs/heads/BRANCH` — имя файла workflow), `head_branch`, `head_sha`, `event`, `run_number`, `run_attempt`, `started_at`, `completed_at`, `html_url`. Числа — числа (в отличие от `tea pulls list`).
- Наблюдались `status`: `in_progress`, `completed`. `conclusion`: `success`, `failure`; пока запуск не завершён, **поля нет**, а `completed_at` = `1970-01-01…` (нулевое время) — не показывать как дату. Фильтр `&status=` принимает `success`, `failure`, `pending`, `queued`, `in_progress`, `skipped`, `canceled` (по справке `tea`); отменённый запуск не воспроизводился: в API нет эндпоинта отмены (`POST …/runs/N/cancel` → 404), а `tea actions runs delete` (он же `cancel`) удаляет запуск — в пилоте не вызывался.
- Как и на GitHub, на один коммит — по запуску на каждый workflow: итог считать по всем запускам последнего `head_sha`.
- Время в ответе — с часовым поясом сервера (`+03:00`).
- Если у репозитория выключены Actions (`has_actions: false`), эндпоинт всё равно отвечает 200 и отдаёт прежние запуски. Маршрута нет на сервере (старая Gitea) — `404` с телом `404 page not found` **текстом**, а нет репозитория — `404` с JSON `{"message":"not found"}`: так адаптер отличает «не поддерживается» (A18) от «нет проекта».
- Ссылка на запуски ветки: фильтра по ветке в веб-интерфейсе Actions нет — адаптер ведёт на последний запуск (`html_url`).
- Сводный статус коммита (все статусы, не только Actions) — `/repos/OWNER/REPO/commits/REF/status` → `state` (`pending` / `success` / `failure`).

### Ошибки

| Ситуация | Код | stderr | Код ядра |
|---|---|---|---|
| Нет ни одного логина | 1 | `Error: no available login` | `AUTH_REQUIRED` |
| Нет логина с таким именем | 1 | `Error: login name 'NAME' does not exist` | `AUTH_REQUIRED` |
| Неверный токен | 1 | `Error: invalid username, password or token` | `AUTH_REQUIRED` |
| Нет сети | 1 | `Error: Get "https://HOST/api/v1/…": dial tcp: lookup HOST …: network is unreachable` (сразу) | `NETWORK_ERROR` |
| Нет репозитория или доступа | 1 | `Error: not found` (`tea api`: код 0, тело `{"message":"not found",…}`, `HTTP/2.0 404`) | `REPO_UNRESOLVED` |
| Сервер старше, чем нужно команде | 1 | `Error: gitea server at … is older than 1.26.0` | `UNSUPPORTED` |
| Вне git-репозитория | 1 | `Error: git rev-parse --show-toplevel: …` | ошибка вызова (не должно случаться) |

- **`tea api` возвращает код 0 и на 401/404** — тело `{"message":"invalid username, password or token"}` / `{"message":"not found",…}`. Статус HTTP виден только с `-i`: первая строка stderr (`HTTP/2.0 200 OK` / `HTTP/2.0 404 Not Found`), JSON — в stdout. Успех определять по статусу, а не по коду завершения.
