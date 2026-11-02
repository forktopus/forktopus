import { afterEach, describe, expect, it, vi } from 'vitest'
import type { AppUpdate } from '../data/model'
import { createMockSource } from '../data/mockSource'
import { createAppStore } from '../store/appStore'
import { updateActions } from './updates'

const base: AppUpdate = {
  revision: 1,
  status: 'available',
  currentVersion: '0.3.1',
  version: '0.4.0',
  format: 'deb',
  filename: 'file.deb',
  size: 100,
  downloaded: 0,
  releaseUrl: '',
  checkedAt: '',
  message: '',
  canDownload: true,
  instruction: '',
}
afterEach(() => vi.useRealTimers())

describe('обновления приложения', () => {
  it('правила действий охватывают ожидание, проверку, скачивание, ошибку и готовый файл', () => {
    for (const status of ['checking', 'downloading', 'verifying']) {
      const a = updateActions({ ...base, status })
      expect(a.check).toBe(false)
      expect(a.download).toBe(false)
      expect(a.folder).toBe(false)
      expect(a.cancel).toBe(status !== 'checking')
    }
    expect(updateActions({ ...base, status: 'current', canDownload: false }).check).toBe(true)
    expect(updateActions({ ...base, status: 'error' }).download).toBe(true)
    expect(updateActions({ ...base, status: 'error', canDownload: false }).download).toBe(false)
    expect(updateActions({ ...base, status: 'ready', canDownload: false }).folder).toBe(true)
    expect(updateActions({ ...base, format: '', status: 'unsupported' }).check).toBe(false)
    expect(updateActions({ ...base, downloaded: 150 }).progress).toBe(100)
  })
  it('открытие настроек только читает снимок и не проверяет сеть', async () => {
    const source = createMockSource({ delayMs: 0 })
    const check = vi.spyOn(source, 'checkAppUpdate')
    const store = createAppStore(source)
    await store.getState().loadAppUpdate()
    expect(store.getState().appUpdate?.status).toBe('idle')
    expect(check).not.toHaveBeenCalled()
  })
  it('повторный вход во время скачивания восстанавливает прогресс, затем доступны проверка и папка', async () => {
    vi.useFakeTimers()
    const source = createMockSource({ delayMs: 0 })
    const store = createAppStore(source)
    await store.getState().checkAppUpdate()
    await store.getState().downloadAppUpdate()
    await vi.advanceTimersByTimeAsync(400)
    const before = store.getState().appUpdate!
    expect(before.status).toBe('downloading')
    expect(before.downloaded).toBeGreaterThan(0)
    // Новый наблюдатель читает уже работающую операцию; сеть и файл не перезапускаются.
    const reopened = createAppStore(source)
    const loading = reopened.getState().loadAppUpdate()
    await vi.advanceTimersByTimeAsync(0)
    await loading
    expect(reopened.getState().appUpdate?.downloaded).toBe(before.downloaded)
    await vi.advanceTimersByTimeAsync(1000)
    expect(reopened.getState().appUpdate?.status).toBe('ready')
    expect(updateActions(reopened.getState().appUpdate!).folder).toBe(true)
  })
  it('отмена останавливает прогресс, повтор начинает новую загрузку', async () => {
    vi.useFakeTimers()
    const store = createAppStore(createMockSource({ delayMs: 0 }))
    await store.getState().checkAppUpdate()
    await store.getState().downloadAppUpdate()
    await vi.advanceTimersByTimeAsync(200)
    await store.getState().cancelAppUpdate()
    await vi.advanceTimersByTimeAsync(1500)
    expect(store.getState().appUpdate?.status).toBe('available')
    await store.getState().downloadAppUpdate()
    await vi.advanceTimersByTimeAsync(1500)
    expect(store.getState().appUpdate?.status).toBe('ready')
  })
  it('запоздалый ответ не затирает более новое событие', async () => {
    const source = createMockSource({ delayMs: 0 })
    let emit: (s: AppUpdate) => void = () => {}
    let resolve!: (s: AppUpdate) => void
    source.onAppUpdate = (cb) => {
      emit = cb
      return () => {}
    }
    source.getAppUpdate = () =>
      new Promise((r) => {
        resolve = r
      })
    const store = createAppStore(source)
    const pending = store.getState().loadAppUpdate()
    emit({ ...base, revision: 3, status: 'downloading', downloaded: 50 })
    resolve({ ...base, revision: 2 })
    await pending
    expect(store.getState().appUpdate?.downloaded).toBe(50)
  })
  it('ошибка сети не ломает store; повтор и открытие папки вызывают нужные методы', async () => {
    const source = createMockSource({ delayMs: 0 })
    const check = vi.spyOn(source, 'checkAppUpdate').mockRejectedValueOnce(new Error('Нет сети'))
    const folder = vi.spyOn(source, 'openAppUpdateFolder').mockResolvedValue()
    const store = createAppStore(source)
    await store.getState().checkAppUpdate()
    expect(store.getState().notices.at(-1)?.text).toContain('Нет сети')
    await store.getState().checkAppUpdate()
    expect(check).toHaveBeenCalledTimes(2)
    expect(store.getState().appUpdate?.status).toBe('available')
    await store.getState().openAppUpdateFolder()
    expect(folder).toHaveBeenCalledOnce()
  })
})
