import { describe, expect, it } from 'vitest'
import { createMockSource } from '../data/mockSource'
import type { DataSource } from '../data/source'
import { createAppStore, fromUI, toUI } from './appStore'

function counted(source: DataSource) {
  const calls = { listRepositories: 0, saveUI: 0 }
  const wrapped: DataSource = {
    ...source,
    listRepositories: () => {
      calls.listRepositories++
      return source.listRepositories()
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
    const before = calls.listRepositories

    store.getState().setMode('radar')
    store.getState().setMode('explorer')
    store.getState().setTheme('light')
    store.getState().setFilters({ registry: 'dirty', asc: false })

    expect(calls.listRepositories).toBe(before)
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

  it('ошибка источника попадает в error, а не роняет store', async () => {
    const store = createAppStore(createMockSource({ delayMs: 0 }))
    expect(await store.getState().addRoot('relative/path')).toBe(false)
    expect(store.getState().error).toMatch(/абсолютный путь/)
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
