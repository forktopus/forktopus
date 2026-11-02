# Провайдеры: результаты пилота CLI

Что фактически отдают `gh`, `glab`, `tea` и `gt` для PR/MR и CI по ветке. По этим находкам пишутся адаптеры (этап 08); при обновлении CLI — перепроверить.

Ответы записаны на пилотных репозиториях `OWNER/forktopus-pilot` на каждом сервере (после проверок удалены; чтобы перепроверить CLI, их нужно создать заново с тем же набором): workflow `ok` (успех), `fail` (падает, только ветка `pilot/fail`), `slow` (долгий, только `pilot/slow`); ветки `pilot/pr-open` (открытый PR), `pilot/pr-draft` (черновик), `pilot/closed-pr` (закрытый PR), `pilot/no-pr`.

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

Последний закрытый или влитый PR (когда открытых нет, D-058):

```sh
gh pr list -R HOST/OWNER/REPO --head BRANCH --state closed --limit 20 \
  --json number,state,url,closedAt,mergedAt,isCrossRepository,headRefName
```

- `--state closed` включает и влитые: у них `state: "MERGED"` и `mergedAt`; у закрытых без слияния `mergedAt: null`. Время — RFC 3339 в UTC.

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
- Последний закрытый или влитый MR (D-058): тот же запрос с `--all --per-page 20`, из ответа берутся `state` `merged` (время `merged_at`) и `closed` (`closed_at`); у влитого `closed_at` — `null`. Время — с миллисекундами (`2026-10-31T14:02:20.028Z`).
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
- Последний закрытый или влитый PR (D-058) — через REST, там PR из форка видно:

  ```sh
  tea api -l LOGIN -i '/repos/OWNER/REPO/pulls?state=closed&sort=recentupdate&limit=50'
  ```

  `state=closed` включает влитые (`merged: true`, `merged_at`); свои PR — `head.ref` == ветка и `head.repo.id` == `base.repo.id` (у PR из удалённого форка `head.repo` — `null`). Читается одна страница: PR ветки, закрытый раньше 50 последних обновлённых, не найдётся. Время — с часовым поясом сервера (`2026-10-31T17:07:36+03:00`).

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

## Gitea — `gt` (на смену `tea`)

Репозиторий `gitea/tea` архивирован 2026-10-29; официальный CLI Gitea теперь `gt` (`gitea.com/gitea/cli`). Адаптер (D-074) работает через `gt`, если у него есть вход на сервер remote, иначе — через `tea` (раздел выше).

Как адаптер использует `gt`:

- логины — `gt auth status --json host,user,active` (пользователь → `--user`, `host` → `--host`);
- каждый вызов — с `--host URL --user USER --no-input`;
- PR, последний закрытый PR и CI — тот же REST, что у `tea api`, через `gt api repos/…` (ответы одинаковые, разбор общий): `gt pr list --head` отбирает PR на клиенте по первой странице (`--debug` показывает один запрос `pulls?limit=50&page=1&state=open`), а у `gt run list` лишний запрос `actions/workflows`;
- диагностика — `gt --version` и `gt api user`.

Проверено вживую (2026-11-01): на одном репозитории `gt` и `tea` дали одинаковые PR (черновик, ноль) и CI.

Проверено: `gt` 0.2.0, Gitea 1.26.2, 2026-11-01, только чтение. Устроен по образцу `gh`: `--json ПОЛЯ`, `--jq`, `-R OWNER/REPO`, `--host URL`.

### Общее

- **Работает вне git-репозитория** (в отличие от `tea`): инстанс — `--host https://HOST`, репозиторий — `-R OWNER/REPO`.
- Аккаунты: `gt auth status --json host,user,active,credential_available,error` → `[{"host":"https://HOST","user":…,"active":true,…}]`, токены не печатаются. С `--json` код всегда 0 — смотреть `credential_available` и `error`; `--verify` проверяет учётку на сервере (`verified: true`). Несколько аккаунтов на хосте — выбирать `--user`.
- Коды выхода документированы (`gt help exit-codes`): 0 — успех (и пустой список), 1 — ошибка, 2 — неверный вызов, 4 — нет аккаунта или учётка отклонена.
  - Нет аккаунта для хоста: код 4, `Error: no account for https://HOST; run gt auth login --host …`.
  - Нет репозитория: код 1, `Error: repository OWNER/REPO not found`.
  - `gt api` на ответ вне 2xx — код 1, тело в stdout, `Error: not found (HTTP 404 GET /api/v1/…)` в stderr (у `tea api` — код 0).
  - 403 (нет прав на раздел, например Actions) — код 1, **без статуса HTTP**: `Error: permission denied by the Gitea instance: user should have specific read permission …` → `ACCESS_DENIED` (D-068).
- Запрет входа (`This account is prohibited from signing in…`) и неактивность (`This account is not activated.`) — `ACCOUNT_BLOCKED` / unavailable: проверять учётную запись в разделе управления пользователями сервера. Это отдельная причина HTTP 403; распознаётся до общего отказа прав, в stderr `gt`/`tea` и JSON API. Повторная авторизация не предлагается.
- В диагностике CLI `ACCOUNT_BLOCKED` показывается как «Вход запрещён», `ACCESS_DENIED` — «Нет доступа», `UNSUPPORTED` — «Не поддерживается». Для PR/CI отказ прав сохраняет поведение D-068: приглушённое ограничение, без общего предупреждения о настройке подключения. Путь к разделу администрирования зависит от версии сервиса; непроверенные URL не подставляются.
- `gt --version` → `gt version 0.2.0 (2026-10-26)` без ANSI-кодов.
- Подсказка `more results available; use --page 2` — в stderr, JSON в stdout чистый.
- В бинарнике есть проверка `gitea server at … is older than …`: часть команд требует новой Gitea, на версии старше 1.26 не проверялось.

### Открытые PR ветки

```sh
gt pr list --host https://HOST -R OWNER/REPO --state open --head BRANCH -L 50 \
  --json number,title,state,draft,head,head_sha,head_repo,base,url,updated_at
```

- `--head BRANCH` фильтрует на стороне `gt` (`OWNER:BRANCH` — для форка); `--state` принимает `open|closed|merged|all`.
- **Черновик — поле `draft`** (PR с заголовком `WIP: …` → `draft: true`).
- **`head_repo` — `OWNER/REPO` источника**: PR из форка отличим (у `tea pulls list` — нет).
- Числа — числа, время — с часовым поясом сервера.
- Для «последнего закрытого или влитого» (D-058): `--state all --sort recentupdate` даёт `state`, `merged`, `merged_at`, но **нет `closed_at`** — время закрытия брать через `gt api 'repos/OWNER/REPO/pulls?state=closed&sort=recentupdate&limit=50'` (поле `closed_at` там есть).

### CI ветки

```sh
gt run list --host https://HOST -R OWNER/REPO --branch BRANCH -L 20 \
  --json id,status,conclusion,workflow,workflow_path,branch,commit,event,attempt,run_number,started_at,completed_at,url
```

- Есть `conclusion` (`success` …), `commit` (полный SHA), `url` — в отличие от `tea actions runs list` (D-041) REST не нужен.
- `--commit SHA` — фильтр по коммиту (для detached HEAD, D-059).
- У запусков по событию `pull_request` поле `branch` пустое; `--branch` находит запуски `push`.
- `attempt` — `0` у первой попытки.
- Незавершённый запуск (`conclusion` без значения) не воспроизводился.
- Сводный статус проверок PR — `gt pr checks N --json name,state,…` (по справке: код 8, пока проверки идут; не проверялось).

## Detached HEAD — поиск по коммиту (D-059)

Ветки нет — PR/MR и CI ищутся по текущему коммиту (`SHA`). Последний закрытый PR — по тем же правилам, что для ветки (D-058). Коммит, которого нет на сервере (не отправлен), — достоверный ноль, а не ошибка.

| | PR/MR | CI |
|---|---|---|
| `gh` | `gh pr list -R HOST/OWNER/REPO --search SHA --state open\|closed …` — поиск GitHub находит PR, где коммит **среди** коммитов; PR из форков отбрасываются | `gh run list … --commit SHA`; ссылка — страница коммита `/commit/SHA` |
| `glab` | `glab api --hostname HOST projects/OWNER%2FREPO/repository/commits/SHA/merge_requests` — MR всех состояний, содержащие коммит, одним ответом (открытые и последний закрытый); коммита нет — `404 Commit Not Found`, код 1 | `glab ci list … --sha SHA`; ссылка — `/-/commit/SHA/pipelines` |
| `tea`, `gt` | REST (`tea api` / `gt api`) `/repos/OWNER/REPO/pulls?state=open&limit=50&page=N` — PR, у которых `head.sha` == SHA (коммит в середине PR Gitea по API не находит); черновик — поле `draft` или префикс `WIP:` | `/actions/runs?head_sha=SHA&limit=20` (неизвестный SHA — `total_count: 0`) |

Проверено на пилотах (2026-11-01): коммит `pilot/pr-open` — открытый PR, коммит `pilot/closed-pr` — закрытый #3, несуществующий SHA — ноль, на всех трёх серверах.


## Ссылки из счётчиков

Числа PR/MR и CI конкретного репозитория открывают список в системном браузере:
в мини-карточках Проводника, индикаторах Реестра и панели деталей, метках Радара.
Ноль успешного ответа тоже открывает список. Если данных нет или формат сервера
не поддерживается, вместо числа остаётся причина; ссылка не угадывается.

Адрес списка (`listUrl`) отличается от адреса конкретного PR или запуска (`url`).
Go строит его из веб-адреса логина CLI и пути проекта, сохраняя порт и подпуть;
SSH-псевдоним и SSH-порт не превращаются в адрес браузера. GitHub и GitLab
сохраняют поддерживаемый фильтр ветки, Gitea открывает список открытых PR или
Actions проекта. Значки перехода к конкретным элементам сохраняются; номер
последнего закрытого/влитого PR ведёт именно к нему.

Старый кэш без `listUrl` получает ссылки после свежей проверки PR/CI. Общая
сводка нескольких репозиториев не ссылается на один произвольный сервер.
