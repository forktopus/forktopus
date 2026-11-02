import { ThemeProvider } from '@mui/material'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { createAppStore } from '../store/appStore'
import { AppStoreProvider } from '../store/context'
import { createMockSource } from '../data/mockSource'
import { createAppTheme } from '../theme/theme'
import { repo } from '../test/fixtures'
import { PRIndicator, CIIndicator } from './indicators'
import { MiniCard } from '../views/ExplorerView'
import type { ReactNode } from 'react'

function render(node: ReactNode) {
  return renderToStaticMarkup(
    <ThemeProvider theme={createAppTheme('dark')}>
      <AppStoreProvider store={createAppStore(createMockSource({ delayMs: 0 }))}>{node}</AppStoreProvider>
    </ThemeProvider>,
  )
}
describe('ссылки чисел во всех карточках', () => {
  it('мини-карточка делает ноль ссылкой, но отсутствие числа оставляет текстом', () => {
    const props = { name: 'PR/MR', note: '', url: 'https://git.example/o/r/pulls', linkLabel: 'Открыть список PR/MR' }
    expect(render(<MiniCard {...props} value={0} />)).toContain('href="https://git.example/o/r/pulls"')
    expect(render(<MiniCard {...props} value={null} />)).not.toContain('href=')
  })
  it('общие индикаторы реестра, деталей и проводника используют адрес списка', () => {
    const r = repo({
      pr: { open: 2, url: 'https://git.example/o/r/pulls/2', listUrl: 'https://git.example/o/r/pulls' },
      ci: { active: 0, listUrl: 'https://git.example/o/r/actions' },
    })
    expect(render(<PRIndicator repo={r} />)).toContain('href="https://git.example/o/r/pulls"')
    expect(render(<CIIndicator repo={r} mode="inline" />)).toContain('href="https://git.example/o/r/actions"')
  })
})
