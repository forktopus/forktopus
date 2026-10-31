// Словарь строк интерфейса. Параметры подставляются как {name}; числовые
// параметры форматируются по ru-RU. Объект с формами one/few/many — строка
// с множественным числом, форма выбирается по параметру {count}.

export interface PluralForms {
  one: string
  few: string
  many: string
  other?: string
}

export const ru = {
  'app.name': 'Forktopus',
  'app.mascotAlt': 'Маскот Forktopus',
  'app.loading': 'Загрузка…',
  'app.demoBadge': 'демо-данные',
  'app.demoHint': 'Данные вымышлены: окно Wails недоступно или включён VITE_FORKTOPUS_DEMO. Git и CLI не вызываются.',

  'mode.tabsLabel': 'Режим просмотра',
  'mode.registry': 'Реестр',
  'mode.explorer': 'Проводник',
  'mode.radar': 'Радар',

  'search.label': 'Поиск репозиториев',
  'search.placeholder': 'Поиск по имени, ветке, каталогу…',

  'action.refresh': 'Проверить статусы',
  'action.tools': 'Диагностика CLI',
  'action.addRoot': 'Добавить корень',
  'action.add': 'Добавить',
  'action.remove': 'Убрать из списка',
  'action.close': 'Закрыть',
  'action.recheck': 'Проверить снова',
  'action.themeToLight': 'Включить светлую тему',
  'action.themeToDark': 'Включить тёмную тему',

  'lastCheck.never': 'ещё не проверялось',
  'lastCheck.at': 'проверено {when}',

  'time.justNow': 'только что',

  'tools.title': 'Диагностика CLI',
  'tools.empty': 'Проверка ещё не выполнялась.',
  'tools.version': 'версия {version}',
  'tools.notFound': 'не найден в PATH',

  'availability.ok': 'Готов',
  'availability.unavailable': 'Недоступен',
  'availability.error': 'Ошибка',
  'availability.not_supported': 'Не поддерживается',

  'roots.title': 'Корневые папки',
  'roots.description':
    'Корень может содержать любое число вложенных Git-репозиториев. Сервер определяется по git remote, а не по имени папки. Убрать корень из списка — не значит удалить файлы.',
  'roots.empty': 'Корневых папок пока нет.',
  'roots.pathLabel': 'Абсолютный путь к папке',
  'roots.pathPlaceholder': '/srv/repos/github.com',
  'roots.pick': 'Выбрать папку…',
  'roots.orManual': 'или введите путь вручную',
  'roots.issues': { one: '{count} каталог не удалось просмотреть', few: '{count} каталога не удалось просмотреть', many: '{count} каталогов не удалось просмотреть' },
  'roots.repoCount': { one: '{count} репозиторий', few: '{count} репозитория', many: '{count} репозиториев' },

  'empty.noRoots.title': 'Добавьте первую корневую папку',
  'empty.noRoots.text':
    'Forktopus найдёт Git-репозитории внутри неё на любой глубине. Файлы на диске и настройки Git не изменяются.',
  'empty.noRepos': 'В добавленных папках не найдено Git-репозиториев.',
  'empty.scanning': 'Идёт поиск репозиториев…',

  'scan.stale': 'Показан список от {time} — с прошлого запуска. Идёт проверка, данные обновятся.',
  'scan.staleIdle': 'Показан список от {time} — с прошлого запуска. Нажмите «Проверить статусы», чтобы обновить.',
  'scan.unknownStatus': { one: 'Статус ещё не прочитан у {count} репозитория.', few: 'Статус ещё не прочитан у {count} репозиториев.', many: 'Статус ещё не прочитан у {count} репозиториев.' },
  'empty.noMatches': 'Ничего не найдено по текущим фильтрам.',

  'screen.draft': 'Черновой вид режима: полноценный экран появится на следующих этапах.',

  'filter.all': 'Все',
  'filter.attention': 'Требуют внимания',
  'filter.dirty': 'Изменения',
  'filter.behind': 'Отстают',
  'filter.ci': 'Ошибки CI',
  'filter.allHosts': 'Все хосты',

  'kpi.repos': 'Репозиториев в выборке',
  'kpi.reposFoot': 'из {total} всего',
  'kpi.changes': 'Есть локальные изменения',
  'kpi.changesFoot': 'staged, unstaged, untracked',
  'kpi.behind': 'Ветки отстают от upstream',
  'kpi.behindFoot': 'по последнему fetch',
  'kpi.noStatus': { one: 'статус не прочитан у {count}', few: 'статус не прочитан у {count}', many: 'статус не прочитан у {count}' },
  'kpi.none': '—',
  'kpi.prci': 'PR/MR · запуски CI',
  'kpi.partial': 'из доступных источников',
  'kpi.complete': 'по всем источникам',

  'column.repo': 'Репозиторий',
  'column.branch': 'Ветка',
  'column.changes': 'Изменения',
  'column.sync': 'Upstream',
  'column.pr': 'PR/MR',
  'column.ci': 'CI',

  'status.unknown': 'статус не прочитан',

  'changes.clean': 'Чисто',
  'changes.count': { one: '{count} изменение', few: '{count} изменения', many: '{count} изменений' },
  'changes.conflicts': { one: '{count} конфликт', few: '{count} конфликта', many: '{count} конфликтов' },
  'changes.operation': 'Незавершённая операция: {operation}',

  'sync.detached': 'detached HEAD',
  'sync.noUpstream': 'нет upstream',
  'sync.unborn': 'нет коммитов',
  'sync.gone': 'upstream удалён',
  'sync.inSync': 'Совпадает',
  'sync.ahead': '↑ {ahead} впереди',
  'sync.behind': '↓ {behind} отстаёт',
  'sync.diverged': '↑ {ahead} ↓ {behind}',

  'remote.unavailable': 'нет данных',
  'remote.error': 'ошибка источника',
  'remote.notSupported': 'не поддерживается',

  'pr.open': { one: '{count} открыт', few: '{count} открыто', many: '{count} открыто' },
  'pr.none': 'нет открытых',
  'ci.active': '{count} выполняется',
  'ci.success': 'Успешно',
  'ci.failure': 'Ошибка',
  'ci.none': 'нет запусков',

  'repo.noRemote': 'без remote',
  'repo.readError': 'Ошибка чтения репозитория',

  'radar.attention': 'Требует внимания',
  'radar.progress': 'В работе',
  'radar.ok': 'В порядке',

  'integration.banner': {
    one: 'У {count} репозитория нет данных PR/CI: проверьте подключения в диагностике CLI.',
    few: 'У {count} репозиториев нет данных PR/CI: проверьте подключения в диагностике CLI.',
    many: 'У {count} репозиториев нет данных PR/CI: проверьте подключения в диагностике CLI.',
  },

  'explorer.noRemote': 'Без remote',
  'explorer.pickRepo': 'Выберите репозиторий в дереве слева.',
  'explorer.fetch': 'Remote-tracking',
  'explorer.fetchedAt': 'по данным fetch {time}',
  'explorer.neverFetched': 'время последнего fetch неизвестно',
  'explorer.status': 'Статус',
  'explorer.statusAt': 'прочитан {time}',
  'explorer.branches': 'Локальные ветки',
  'explorer.noBranches': 'Веток пока нет.',
  'explorer.branchCurrent': 'текущая',
  'explorer.branchBusy': 'занята в {path}',
  'action.refreshRepo': 'Перечитать статус',
  'action.fetch': 'Fetch',
  'action.pull': 'Pull (fast-forward)',
  'action.switch': 'Переключить',
  'op.fetch': 'Fetch',
  'op.pull': 'Pull',
  'op.switch': 'Switch',
  'op.running': '{op} выполняется…',
  'op.done': '{op}: готово за {sec} с',
  'op.failed': '{op} не выполнен',
} satisfies Record<string, string | PluralForms>
