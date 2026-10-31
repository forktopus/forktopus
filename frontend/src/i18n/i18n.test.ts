import { describe, expect, it } from 'vitest'
import { formatNumber, formatRelative, t } from './index'

// Intl разделяет разряды неразрывным пробелом; в ожиданиях сравниваем без него.
const plain = (s: string) => s.replace(/\s/g, ' ')

describe('t', () => {
  it('подставляет параметры и форматирует числа', () => {
    expect(plain(t('kpi.reposFoot', { total: 12345 }))).toBe('из 12 345 всего')
    expect(t('tools.version', { version: '2.43.0' })).toBe('версия 2.43.0')
  })

  it('оставляет неизвестный параметр как есть', () => {
    expect(t('tools.version')).toBe('версия {version}')
  })

  it('выбирает форму множественного числа', () => {
    expect(t('roots.repoCount', { count: 1 })).toBe('1 репозиторий')
    expect(t('roots.repoCount', { count: 3 })).toBe('3 репозитория')
    expect(t('roots.repoCount', { count: 5 })).toBe('5 репозиториев')
    expect(t('roots.repoCount', { count: 21 })).toBe('21 репозиторий')
    expect(t('roots.repoCount', { count: 0 })).toBe('0 репозиториев')
  })
})

describe('форматтеры', () => {
  const now = new Date('2026-10-31T12:00:00Z')
  const ago = (seconds: number) => new Date(now.getTime() - seconds * 1000)

  it('formatNumber по ru-RU', () => {
    expect(plain(formatNumber(1234567))).toBe('1 234 567')
  })

  it('formatRelative', () => {
    expect(formatRelative(ago(10), now)).toBe('только что')
    expect(formatRelative(ago(5 * 60), now)).toBe('5 мин. назад')
    expect(formatRelative(ago(2 * 3600), now)).toBe('2 ч назад')
    expect(formatRelative(ago(24 * 3600), now)).toBe('вчера')
    expect(formatRelative(ago(3 * 24 * 3600).toISOString(), now)).toBe('3 дн. назад')
  })

  it('formatRelative для старых дат отдаёт дату', () => {
    expect(formatRelative(ago(90 * 24 * 3600), now)).toMatch(/2026/)
  })
})
