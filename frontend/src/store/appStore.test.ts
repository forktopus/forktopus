import { describe, expect, it } from 'vitest'
import { createMockSource } from '../data/mockSource'
import { Availability, ErrorCode, Operation, Provider, type RemoteStatus, type Repository } from '../data/model'
import type { DataSource } from '../data/source'
import { createAppStore, fromUI, toUI, withRemote } from './appStore'

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
    expect(store.getState().error).toMatch(/не найден/)
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
    expect(store.getState().error).toMatch(/абсолютный путь/)
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

    emit({ ...status, branch: 'other', pr: { ...status.pr, open: 7 } })
    expect(store.getState().repos[repo.id].pr).toEqual(repo.pr)

    emit(status)
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
  it('detached HEAD сопоставляется с пустой веткой, provider "" не трогает remote', () => {
    const repo = { id: 'r', remote: 'origin', remotes: [{ name: 'origin', provider: Provider.ProviderGitHub }], branch: { detached: true, headSha: 'abc' } } as Repository
    const st = { repoId: 'r', branch: '', provider: Provider.$zero, pr: { availability: Availability.AvailabilityNotSupported }, ci: { availability: Availability.AvailabilityNotSupported } } as RemoteStatus
    const got = withRemote(repo, st)
    expect(got.pr.availability).toBe(Availability.AvailabilityNotSupported)
    expect(got.remotes?.[0].provider).toBe(Provider.ProviderGitHub)
    expect(withRemote(repo, undefined)).toBe(repo)
  })
})

describe('fromUI / toUI', () => {
  it('неизвестные значения заменяются значениями по умолчанию', () => {
    expect(fromUI({ mode: 'kanban', theme: 'sepia', filters: { registry: '???', sort: 'x' } })).toEqual({
      mode: 'registry',
      theme: null,
      filters: { registry: 'all', host: '', sort: 'name', asc: true },
    })
  })

  it('туда и обратно без потерь', () => {
    const state = { mode: 'radar' as const, theme: 'dark' as const, filters: { registry: 'ci' as const, host: 'gitlab.com', sort: 'pr' as const, asc: false } }
    expect(fromUI(toUI(state))).toEqual(state)
  })
})
