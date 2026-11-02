import { describe, expect, it } from 'vitest'
import { Availability } from '../data/model'
import { repo } from '../test/fixtures'
import { indicatorURL, remoteListURL, serverURL } from './serverLinks'
import { radarTags } from './radar'

const listUrl = 'https://git.example/base/o/r/pulls?state=open'
describe('переходы из счётчиков', () => {
  it('ноль и несколько PR ведут к списку, а не к одному PR', () => {
    for (const open of [0, 1, 3]) {
      const r = repo({ pr: { open, listUrl, url: 'https://git.example/base/o/r/pulls/15' } })
      expect(indicatorURL(r, 'pr')).toBe(listUrl)
    }
  })
  it('нулевой CI открывает список; недоступные данные и старый кэш не угадывают ссылку', () => {
    expect(remoteListURL(repo({ ci: { active: 0, listUrl: 'https://git.example/o/r/actions' } }).ci)).toBe('https://git.example/o/r/actions')
    expect(remoteListURL(repo({ pr: { availability: Availability.AvailabilityError, listUrl } }).pr)).toBeUndefined()
    expect(remoteListURL(repo({ pr: { url: 'https://git.example/o/r/pull/1' } }).pr)).toBeUndefined()
  })
  it('последний закрытый PR сохраняет переход к конкретному PR', () => {
    const r = repo({ pr: { open: 0, listUrl, last: { number: 5, merged: true, url: 'https://git.example/o/r/pulls/5' } } })
    expect(indicatorURL(r, 'pr')).toBe('https://git.example/o/r/pulls/5')
  })
  it('радar помечает PR и CI явно, не разбирая переведённую подпись', () => {
    const tags = radarTags(repo({ pr: { open: 2, listUrl }, ci: { active: 1, listUrl: 'https://git.example/o/r/actions' } }))
    expect(tags.filter((t) => t.list).map((t) => t.list)).toEqual(['ci', 'pr'])
  })
  it('не открывает неподдерживаемый протокол, credentials и относительный адрес', () => {
    for (const bad of ['javascript:alert(1)', 'file:///tmp/a', '/o/r/pulls', 'https://u:p@git.example/o/r']) expect(serverURL(bad)).toBeUndefined()
    expect(serverURL('http://git.local/o/r/actions')).toBe('http://git.local/o/r/actions')
  })
})
