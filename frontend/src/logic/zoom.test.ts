import { describe, expect, it } from 'vitest'
import { clampZoom, zoomKey } from './zoom'

const key = (code: string, key: string, mod: Partial<Record<'ctrlKey' | 'metaKey' | 'altKey', boolean>> = { ctrlKey: true }) =>
  zoomKey({ ctrlKey: false, metaKey: false, altKey: false, code, key, ...mod })

describe('масштаб интерфейса (D-061)', () => {
  it('в пределах 100–200 %, шаг 10 %, мусор — 100 %', () => {
    expect(clampZoom(undefined)).toBe(1)
    expect(clampZoom(0)).toBe(1)
    expect(clampZoom(0.8)).toBe(1)
    expect(clampZoom(1.1 + 0.1)).toBe(1.2)
    expect(clampZoom(1.26)).toBe(1.3)
    expect(clampZoom(2.4)).toBe(2)
    expect(clampZoom(Number.NaN)).toBe(1)
  })

  it('Ctrl/⌘ + = − 0 и цифровой блок; без модификатора и с Alt — нет', () => {
    expect(key('Equal', '=')).toBe('in')
    expect(key('Equal', '+', { ctrlKey: true })).toBe('in')
    expect(key('NumpadAdd', '+')).toBe('in')
    expect(key('Minus', '-')).toBe('out')
    expect(key('NumpadSubtract', '-')).toBe('out')
    expect(key('Digit0', '0', { metaKey: true })).toBe('reset')
    expect(key('Equal', '=', {})).toBeNull()
    expect(key('Equal', '=', { ctrlKey: true, altKey: true })).toBeNull()
    expect(key('KeyA', 'a')).toBeNull()
  })
})
