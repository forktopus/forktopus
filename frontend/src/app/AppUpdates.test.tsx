import { ThemeProvider } from '@mui/material'
import { renderToStaticMarkup } from 'react-dom/server'
import { createStore } from 'zustand/vanilla'
import { describe, expect, it } from 'vitest'
import type { AppUpdate } from '../data/model'
import { createMockSource } from '../data/mockSource'
import { createAppStore } from '../store/appStore'
import { AppStoreProvider } from '../store/context'
import { createAppTheme } from '../theme/theme'
import { AppUpdates } from './AppUpdates'

// Серверный рендер проверяет доступные действия без нового DOM-окружения.
// Сетевые переходы и повторное открытие проверяются отдельно через DataSource.
function render(status: string, patch: Partial<AppUpdate> = {}) {
  const initial = createAppStore(createMockSource({ delayMs: 0 })).getState()
  const state: AppUpdate = {
    revision: 1,
    status,
    currentVersion: '0.3.1',
    version: '0.4.0',
    format: 'deb',
    filename: 'file.deb',
    size: 100,
    downloaded: 50,
    releaseUrl: 'https://github.com/forktopus/forktopus/releases/tag/v0.4.0',
    checkedAt: '',
    message: '',
    canDownload: false,
    instruction: 'Закройте приложение и установите пакет вручную.',
    ...patch,
  }
  const store = createStore(() => ({ ...initial, appUpdate: state }))
  return renderToStaticMarkup(
    <ThemeProvider theme={createAppTheme('dark')}>
      <AppStoreProvider store={store}>
        <AppUpdates open />
      </AppStoreProvider>
    </ThemeProvider>,
  )
}

describe('секция обновлений', () => {
  it('показывает версию, пакет и скачивание доступного обновления', () => {
    const html = render('available', { canDownload: true })
    expect(html).toContain('0.3.1')
    expect(html).toContain('0.4.0')
    expect(html).toContain('file.deb')
    expect(html).not.toContain('combobox')
    expect(html).not.toContain('Формат пакета')
    expect(html).toContain('Скачать')
    expect(html).toContain('Что нового')
    expect(html).not.toContain('Открыть папку')
  })
  it('при скачивании показывает проценты и отмену', () => {
    const html = render('downloading')
    expect(html).toContain('Скачано: 50 %')
    expect(html).toContain('Отменить')
    expect(html).not.toContain('Открыть папку')
  })
  it('готовый пакет открывает папку с инструкцией ручной установки', () => {
    const html = render('ready', { message: 'Размер и SHA-256 проверены.' })
    expect(html).toContain('Открыть папку')
    expect(html).toContain('установите пакет вручную')
    expect(html).not.toContain('Отменить')
    expect(html).not.toContain('>Скачать<')
  })
  it('при ошибке предлагает повторное скачивание, для dev объясняет ограничение', () => {
    expect(render('error', { canDownload: true, message: 'SHA-256 не совпадает' })).toContain('Скачать')
    const html = render('unsupported', { format: '', version: '', filename: '', message: 'Сборка разработки' })
    expect(html).toContain('Сборка разработки')
    expect(html).not.toContain('>Скачать<')
  })
})
