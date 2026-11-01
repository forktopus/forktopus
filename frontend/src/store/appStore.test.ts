import { describe, expect, it, vi } from 'vitest'
import { createMockSource } from '../data/mockSource'
import { Availability, ErrorCode, Operation, Provider, type RemoteStatus, type Repository } from '../data/model'
import type { DataSource } from '../data/source'
import { createAppStore, fromUI, scanFlushMs, toUI, withRemote } from './appStore'

const hasLocal = (r: Repository) => Boolean(r.worktree && r.worktree.staged + r.worktree.unstaged + r.worktree.untracked + r.worktree.conflicts > 0)

function counted(source: DataSource) {
  const calls = { listRepositories: 0, scanRoots: 0, saveUI: 0 }
  const wrapped: DataSource = {
    ...source,
    listRepositories: () => {
      calls.listRepositories++
      return source.listRepositories()
    },
    scanRoots: () => {
      calls.scanRoots++
      return source.scanRoots()
    },
    saveUI: (ui) => {
      calls.saveUI++
      return source.saveUI(ui)
    },
  }
  return { wrapped, calls }
}

describe('store', () => {
  it('init загружает корни, репозитории и диагностику', async () => {
    const store = createAppStore(createMockSource({ delayMs: 0 }))
    await store.getState().init()
    const s = store.getState()
    expect(s.ready).toBe(true)
    expect(s.sourceKind).toBe('mock')
    expect(s.roots.length).toBeGreaterThan(0)
    expect(Object.keys(s.repos).length).toBeGreaterThan(0)
    expect(s.tools.map((t) => t.name)).toContain('git')
    expect(s.lastCheckAt).not.toBeNull()
  })

  it('переключение режима не перечитывает репозитории и сохраняется (A26, A25)', async () => {
    const source = createMockSource({ delayMs: 0 })
    const { wrapped, calls } = counted(source)
    const store = createAppStore(wrapped)
    await store.getState().init()
    const before = { ...calls }

    store.getState().setMode('radar')
    store.getState().setMode('explorer')
    store.getState().setTheme('light')
    store.getState().setFilters({ registry: 'dirty', asc: false })

    expect(calls.listRepositories).toBe(before.listRepositories)
    expect(calls.scanRoots).toBe(before.scanRoots)
    expect(calls.saveUI).toBe(4)
    await new Promise((r) => setTimeout(r, 0))

    // «Перезапуск»: новый store на том же источнике читает сохранённое.
    const restarted = createAppStore(source)
    await restarted.getState().init()
    const s = restarted.getState()
    expect(s.mode).toBe('explorer')
    expect(s.theme).toBe('light')
    expect(s.filters).toMatchObject({ registry: 'dirty', asc: false })
  })

  it('сначала показывает кэш с пометкой, после сканирования пометка снимается (D-027)', async () => {
    const store = createAppStore(createMockSource({ delayMs: 0 }))
    const states: { stale: boolean; scanning: boolean; repos: number }[] = []
    store.subscribe((s) => states.push({ stale: s.stale, scanning: s.scanning, repos: Object.keys(s.repos).length }))
    await store.getState().init()
    expect(states.some((s) => s.stale && s.repos > 0)).toBe(true)
    const s = store.getState()
    expect(s.stale).toBe(false)
    expect(s.scanning).toBe(false)
    expect(s.issues.length).toBeGreaterThan(0)
  })

  it('найденные во время сканирования репозитории появляются до конца сканирования', async () => {
    const source = createMockSource({ delayMs: 0 })
    const store = createAppStore(source)
    let seenDuringScan = 0
    store.subscribe((s) => {
      if (s.scanning) seenDuringScan = Math.max(seenDuringScan, Object.keys(s.repos).length)
    })
    await store.getState().removeRoot('r-github')
    await store.getState().scan()
    expect(seenDuringScan).toBeGreaterThan(0)
    // Итог сканирования восстанавливает parentId вложенных.
    expect(Object.values(store.getState().repos).every((r) => !r.nesting || r.parentId)).toBe(true)
  })

  it('выбор папки диалогом добавляет корень и сканирует', async () => {
    const store = createAppStore(createMockSource({ delayMs: 0 }))
    await store.getState().init()
    const before = store.getState().roots.length
    expect(await store.getState().pickAndAddRoot()).toBe(true)
    expect(store.getState().roots).toHaveLength(before + 1)
  })

  it('перечитывает статус одного репозитория и отдаёт его ветки', async () => {
    const store = createAppStore(createMockSource({ delayMs: 0 }))
    await store.getState().init()
    const [id, before] = Object.entries(store.getState().repos)[0]
    await new Promise((r) => setTimeout(r, 5))
    await store.getState().refreshRepo(id)
    expect(store.getState().repos[id].updatedAt > before.updatedAt).toBe(true)
    const branches = await store.getState().loadBranches(id)
    expect(branches?.filter((b) => b.current)).toHaveLength(1)
    expect(await store.getState().loadBranches('nope')).toBeNull()
    expect(store.getState().notices.at(-1)?.text).toMatch(/не найден/)
  })

  it('операции: итог и свежий статус, блокировка с причиной, одна операция за раз (D-031)', async () => {
    const store = createAppStore(createMockSource({ delayMs: 0 }))
    await store.getState().init()
    const repos = Object.values(store.getState().repos)
    const behind = repos.find((r) => (r.branch?.behind ?? 0) > 0 && (r.branch?.ahead ?? 0) === 0 && !hasLocal(r))!
    const dirty = repos.find((r) => hasLocal(r) && r.branch?.upstream)!

    const first = store.getState().runOp(behind.id, Operation.OperationPull)
    expect(store.getState().running[behind.id]).toBe(Operation.OperationPull)
    expect(await store.getState().runOp(behind.id, Operation.OperationFetch)).toBeNull()
    const res = await first
    expect(res?.error).toBeFalsy()
    expect(store.getState().repos[behind.id].branch?.behind).toBe(0)
    expect(store.getState().running[behind.id]).toBeUndefined()
    expect(store.getState().lastOps[behind.id]).toEqual(res)

    const blocked = await store.getState().runOp(dirty.id, Operation.OperationPull)
    expect(blocked?.error?.code).toBe(ErrorCode.CodeDirtyWorktree)
    expect(store.getState().lastOps[dirty.id]?.error?.message).toMatch(/локальные изменения/)
  })

  it('ошибка источника попадает в error, а не роняет store', async () => {
    const store = createAppStore(createMockSource({ delayMs: 0 }))
    expect(await store.getState().addRoot('relative/path')).toBe(false)
    expect(store.getState().notices.at(-1)).toMatchObject({ tone: 'bad', text: expect.stringMatching(/абсолютный путь/) })
  })

  it('серверные данные из события попадают в репозиторий только для его текущей ветки', async () => {
    const source = createMockSource({ delayMs: 0 })
    let emit: (st: RemoteStatus) => void = () => {}
    const store = createAppStore({ ...source, onRemoteStatus: (cb) => ((emit = cb), () => {}) })
    await store.getState().init()
    const repo = Object.values(store.getState().repos).find((r) => r.branch?.name && !r.branch.detached)!
    const one = 1
    const status: RemoteStatus = {
      repoId: repo.id,
      branch: repo.branch!.name!,
      provider: Provider.ProviderGitea,
      pr: { availability: Availability.AvailabilityOK, open: one, draft: 0 },
      ci: { availability: Availability.AvailabilityOK, active: 0, lastResult: 'failure' },
    }

    // События копятся и попадают в store пачкой.
    const flushed = () => new Promise((r) => setTimeout(r, scanFlushMs + 10))
    emit({ ...status, branch: 'other', pr: { ...status.pr, open: 7 } })
    await flushed()
    expect(store.getState().repos[repo.id].pr).toEqual(repo.pr)

    const before = store.getState().repos[repo.id]
    emit(status)
    expect(store.getState().repos[repo.id]).toBe(before)
    await flushed()
    const got = store.getState().repos[repo.id]
    expect(got.pr.open).toBe(1)
    expect(got.ci.lastResult).toBe('failure')
    expect(got.remotes?.find((r) => r.name === got.remote)?.provider).toBe(Provider.ProviderGitea)

    // Сканирование, ответ которого собран до события, не возвращает прежние данные.
    await store.getState().scan()
    expect(store.getState().repos[repo.id].ci.lastResult).toBe('failure')
  })

  it('обновление репозитория запрашивает и серверные данные мимо кэша', async () => {
    const source = createMockSource({ delayMs: 0 })
    let forced = 0
    const store = createAppStore({ ...source, refreshRemote: (id) => (forced++, source.refreshRemote(id)) })
    await store.getState().init()
    const id = Object.values(store.getState().repos).find((r) => r.pr.availability === Availability.AvailabilityOK)!.id
    await store.getState().refreshRepo(id)
    expect(forced).toBe(1)
    expect(store.getState().repos[id].pr.lastAttemptAt).toBeTruthy()
  })

  it('Pull и Switch ждут подтверждения, Fetch — нет; итог приходит уведомлением (D-047)', async () => {
    const store = createAppStore(createMockSource({ delayMs: 0 }))
    await store.getState().init()
    const repo = Object.values(store.getState().repos).find((r) => r.branch?.behind && !hasLocal(r))!

    store.getState().requestOp(repo.id, Operation.OperationPull)
    expect(store.getState().pendingOp).toEqual({ id: repo.id, op: Operation.OperationPull, branch: undefined })
    expect(store.getState().running[repo.id]).toBeUndefined()
    store.getState().cancelOp()
    expect(store.getState().pendingOp).toBeNull()

    store.getState().requestOp(repo.id, Operation.OperationPull)
    await store.getState().confirmOp()
    expect(store.getState().pendingOp).toBeNull()
    expect(store.getState().lastOps[repo.id]?.op).toBe(Operation.OperationPull)
    const notice = store.getState().notices.at(-1)!
    expect(notice.tone).toBe('good')
    expect(notice.text).toMatch(/Pull: готово/)
    store.getState().dismissNotice(notice.id)
    expect(store.getState().notices).not.toContainEqual(notice)

    store.getState().requestOp(repo.id, Operation.OperationFetch)
    expect(store.getState().pendingOp).toBeNull()
    expect(store.getState().running[repo.id]).toBe(Operation.OperationFetch)
  })

  it('прогресс сканирования — по событиям найденных репозиториев, с нуля при каждом скане', async () => {
    const source = createMockSource({ delayMs: 0 })
    const store = createAppStore(source)
    await store.getState().init()
    const total = Object.keys(store.getState().repos).length
    expect(store.getState().scanFound).toBe(total)

    const progress: number[] = []
    let renders = 0
    const unsubscribe = store.subscribe((s, prev) => {
      if (s.scanning) progress.push(s.scanFound)
      if (s.repos !== prev.repos) renders++
    })
    await store.getState().scan()
    unsubscribe()
    expect(progress[0]).toBe(0)
    expect(progress.at(-1)).toBe(total)
    // Быстрые события копятся: список меняется не на каждое событие, а итогом (11.1).
    expect(renders).toBe(1)
  })

  it('найденные во время долгого сканирования репозитории попадают в список пачками', async () => {
    const source = createMockSource({ delayMs: 0 })
    // Ответы мока идут через setTimeout — образец берём до подмены таймеров.
    const a = { ...(await source.listRepositories()).repositories[0] }
    vi.useFakeTimers()
    try {
      let emit: (r: Repository) => void = () => {}
      let finish: () => void = () => {}
      const store = createAppStore({
        ...source,
        onScanRepository: (cb) => ((emit = cb), () => {}),
        scanRoots: () => new Promise((resolve) => (finish = () => resolve({ repositories: [], issues: [], scannedAt: new Date().toISOString() }))),
      })
      const scan = store.getState().scan()
      emit({ ...a, id: 'x1' })
      emit({ ...a, id: 'x2' })
      expect(store.getState().repos.x1).toBeUndefined()
      await vi.advanceTimersByTimeAsync(scanFlushMs)
      expect(Object.keys(store.getState().repos)).toEqual(expect.arrayContaining(['x1', 'x2']))
      expect(store.getState().scanFound).toBe(2)

      emit({ ...a, id: 'x3' })
      finish()
      await scan
      await vi.advanceTimersByTimeAsync(scanFlushMs)
      // Итог сканирования не перезаписывается запоздавшей пачкой.
      expect(store.getState().repos.x3).toBeUndefined()
    } finally {
      vi.useRealTimers()
    }
  })

  it('фоновая проверка Go: статус заменяет запись пачкой, кроме сканирования и убранных репозиториев (18.1)', async () => {
    const source = createMockSource({ delayMs: 0 })
    let emitRepo: (r: Repository) => void = () => {}
    let emitChecked: (at: string) => void = () => {}
    const store = createAppStore({ ...source, onRepository: (cb) => ((emitRepo = cb), () => {}), onChecked: (cb) => ((emitChecked = cb), () => {}) })
    await store.getState().init()
    const [a, b] = Object.values(store.getState().repos)
    vi.useFakeTimers()
    try {
      const changed = { ...a, parentId: undefined, worktree: { ...a.worktree!, untracked: 42 } }
      emitRepo(changed)
      emitRepo({ ...b, id: 'removed' })
      expect(store.getState().repos[a.id].worktree?.untracked).not.toBe(42)
      await vi.advanceTimersByTimeAsync(scanFlushMs)
      expect(store.getState().repos[a.id].worktree?.untracked).toBe(42)
      expect(store.getState().repos[a.id].parentId).toBe(a.parentId)
      expect(store.getState().repos.removed).toBeUndefined()

      store.setState({ scanning: true })
      emitRepo({ ...b, worktree: { ...b.worktree!, untracked: 7 } })
      await vi.advanceTimersByTimeAsync(scanFlushMs)
      expect(store.getState().repos[b.id].worktree?.untracked).not.toBe(7)

      store.setState({ stale: true })
      emitChecked('2026-11-01T12:00:00Z')
      expect(store.getState().lastCheckAt).toBe('2026-11-01T12:00:00Z')
      expect(store.getState().stale).toBe(false)
    } finally {
      vi.useRealTimers()
    }
  })

  it('клик по уведомлению выбирает и открывает репозиторий (18.3)', async () => {
    const source = createMockSource({ delayMs: 0 })
    let reveal: (id: string) => void = () => {}
    const store = createAppStore({ ...source, onReveal: (cb) => ((reveal = cb), () => {}) })
    await store.getState().init()
    const id = Object.keys(store.getState().repos)[0]
    reveal('unknown')
    expect(store.getState().detailsId).toBeNull()
    reveal(id)
    expect(store.getState().selectedId).toBe(id)
    expect(store.getState().detailsId).toBe(id)
  })

  it('сводка для трея: после загрузки и только при изменении списка (18.2)', async () => {
    const source = createMockSource({ delayMs: 0 })
    const sent: string[] = []
    const store = createAppStore({ ...source, setTraySummary: async (text) => void sent.push(text) })
    await store.getState().init()
    expect(sent).toHaveLength(1)
    expect(sent[0]).toMatch(/^Требуют внимания: \d+ · в работе: \d+ · всего: \d+$/)
    store.getState().setQuery('demo')
    store.getState().setMode('radar')
    expect(sent).toHaveLength(1)
  })

  it('отчёт диагностики: из источника, ошибка — в уведомлениях', async () => {
    const source = createMockSource({ delayMs: 0, scenario: 'nocli' })
    const store = createAppStore(source)
    await store.getState().init()
    const d = await store.getState().loadDiagnostics()
    expect(d?.report).toMatch(/Утилиты/)
    expect(d?.errors?.some((e) => e.message.startsWith('gh'))).toBe(true)

    const broken = createAppStore({ ...source, getDiagnostics: () => Promise.reject(new Error('нет связи')) })
    expect(await broken.getState().loadDiagnostics()).toBeNull()
    expect(broken.getState().notices.at(-1)?.text).toBe('нет связи')
  })

  it('список пропусков: сохранение пересканирует, ошибка ввода — уведомление, пустой — умолчание', async () => {
    const source = createMockSource({ delayMs: 0 })
    const store = createAppStore(source)
    await store.getState().init()
    const initial = await store.getState().loadSkipDirs()
    expect(initial?.dirs).toEqual(initial?.defaults)

    let scans = 0
    const counting = createAppStore({ ...source, scanRoots: () => (scans++, source.scanRoots()) })
    const saved = await counting.getState().saveSkipDirs([' archive ', 'node_modules', 'archive'])
    expect(saved?.dirs).toEqual(['archive', 'node_modules'])
    expect(scans).toBe(1)

    expect(await counting.getState().saveSkipDirs(['a/b'])).toBeNull()
    expect(counting.getState().notices.at(-1)?.text).toMatch(/имя каталога/)
    expect(scans).toBe(1)
    expect((await counting.getState().loadSkipDirs())?.dirs).toEqual(['archive', 'node_modules'])

    expect((await counting.getState().saveSkipDirs([]))?.dirs).toEqual(initial?.defaults)
  })

  it('удаление корня убирает его репозитории из списка', async () => {
    const store = createAppStore(createMockSource({ delayMs: 0 }))
    await store.getState().init()
    const root = store.getState().roots[0]
    await store.getState().removeRoot(root.id)
    const s = store.getState()
    expect(s.roots.some((r) => r.id === root.id)).toBe(false)
    expect(Object.values(s.repos).some((r) => r.rootId === root.id)).toBe(false)
  })
})

describe('withRemote', () => {
  it('detached HEAD сопоставляется по коммиту (@SHA), provider "" не трогает remote', () => {
    const repo = {
      id: 'r',
      remote: 'origin',
      remotes: [{ name: 'origin', provider: Provider.ProviderGitHub }],
      branch: { detached: true, headSha: 'abc' },
    } as Repository
    const st = {
      repoId: 'r',
      branch: '@abc',
      provider: Provider.$zero,
      pr: { availability: Availability.AvailabilityOK, open: 1 },
      ci: { availability: Availability.AvailabilityOK },
    } as RemoteStatus
    const got = withRemote(repo, st)
    expect(got.pr.open).toBe(1)
    expect(got.remotes?.[0].provider).toBe(Provider.ProviderGitHub)
    expect(withRemote(repo, undefined)).toBe(repo)
    // Данные другого коммита или прежней ветки не подставляются.
    expect(withRemote(repo, { ...st, branch: '@def' })).toBe(repo)
    expect(withRemote(repo, { ...st, branch: '' })).toBe(repo)
  })
})

describe('масштаб', () => {
  it('применяется к окну, сохраняется, не выходит за пределы (D-061)', async () => {
    const source = createMockSource({ delayMs: 0 })
    const applied: number[] = []
    const store = createAppStore({ ...source, setZoom: async (z) => void applied.push(z) })
    await store.getState().init()
    expect(applied).toEqual([1])

    store.getState().setZoom(1.2)
    store.getState().setZoom(1.2) // то же значение — ни окна, ни сохранения
    store.getState().setZoom(5)
    store.getState().setZoom(0.5)
    expect(applied).toEqual([1, 1.2, 2, 1])
    store.getState().setZoom(1.5)
    await new Promise((r) => setTimeout(r, 0))

    const restarted = createAppStore(source)
    await restarted.getState().init()
    expect(restarted.getState().zoom).toBe(1.5)
  })
})

describe('fromUI / toUI', () => {
  it('неизвестные значения заменяются значениями по умолчанию', () => {
    expect(fromUI({ mode: 'kanban', theme: 'sepia', filters: { registry: '???', sort: 'x' } })).toEqual({
      mode: 'registry',
      theme: null,
      filters: { registry: 'all', host: '', sort: 'name', asc: true },
      fetchPrune: false,
      zoom: 1,
      backgroundMinutes: 5,
      autoFetch: false,
      quitOnClose: false,
      notifications: { ci: true, behind: true, pr: true },
    })
    // Интервал — только из списка; чужое значение — по умолчанию.
    expect(fromUI({ backgroundMinutes: 7 }).backgroundMinutes).toBe(5)
    expect(fromUI({ backgroundMinutes: 15 }).backgroundMinutes).toBe(15)
  })

  it('туда и обратно без потерь', () => {
    const state = {
      mode: 'radar' as const,
      theme: 'dark' as const,
      filters: { registry: 'ci' as const, host: 'gitlab.com', sort: 'pr' as const, asc: false },
      fetchPrune: true,
      zoom: 1.3,
      backgroundMinutes: 30,
      autoFetch: true,
      quitOnClose: true,
      notifications: { ci: false, behind: true, pr: false },
    }
    expect(fromUI(toUI(state))).toEqual(state)
  })
})
