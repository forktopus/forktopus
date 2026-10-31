import { describe, expect, it } from 'vitest'
import { Availability, ErrorCode } from '../data/model'
import { repo, unavailable } from '../test/fixtures'
import { groupRadar, radarGroup, radarNote, radarTags } from './radar'

describe('группировка радара', () => {
  it('требует внимания: конфликт, незавершённая операция, ошибка чтения', () => {
    expect(radarGroup(repo({ worktree: { conflicts: 1 } }))).toBe('attention')
    expect(radarGroup(repo({ worktree: { operation: 'rebase' } }))).toBe('attention')
    expect(radarGroup(repo({ error: { code: ErrorCode.CodeOther, message: 'permission denied' } }))).toBe('attention')
  })

  it('в работе: изменения или достоверное отставание', () => {
    expect(radarGroup(repo({ worktree: { untracked: 1 } }))).toBe('progress')
    expect(radarGroup(repo({ branch: { behind: 2 } }))).toBe('progress')
  })

  it('в порядке: чисто, в том числе без upstream и с ahead', () => {
    expect(radarGroup(repo())).toBe('ok')
    expect(radarGroup(repo({ branch: { ahead: 3 } }))).toBe('ok')
    expect(radarGroup(repo({ branch: { upstream: undefined, ahead: null, behind: null } }))).toBe('ok')
  })

  it('отсутствие CLI не красит репозиторий в «Требует внимания»', () => {
    expect(radarGroup(repo({ pr: unavailable, ci: unavailable }))).toBe('ok')
    expect(
      radarGroup(repo({ ci: { availability: Availability.AvailabilityError, error: { code: ErrorCode.CodeAuthRequired, message: '' } } })),
    ).toBe('ok')
  })

  it('упавший CI — серверное состояние, не «требует внимания»', () => {
    expect(radarGroup(repo({ ci: { lastResult: 'failure' } }))).toBe('ok')
  })

  it('статус не прочитан — ни в одной группе, но ошибка чтения — во «внимании»', () => {
    expect(radarGroup({ ...repo(), worktree: null, branch: null })).toBeNull()
    expect(radarGroup({ ...repo({ error: { code: ErrorCode.CodeOther, message: 'x' } }), worktree: null })).toBe('attention')
    expect(groupRadar([{ ...repo({ id: 'u' }), worktree: null }]).unknown.map((r) => r.id)).toEqual(['u'])
  })

  it('groupRadar раскладывает все репозитории', () => {
    const groups = groupRadar([repo({ id: 'a' }), repo({ id: 'b', worktree: { conflicts: 1 } }), repo({ id: 'c', worktree: { staged: 1 } })])
    expect(groups.attention.map((r) => r.id)).toEqual(['b'])
    expect(groups.progress.map((r) => r.id)).toEqual(['c'])
    expect(groups.ok.map((r) => r.id)).toEqual(['a'])
  })
})

describe('метки и пояснение карточки', () => {
  const texts = (r: Parameters<typeof radarTags>[0]) => radarTags(r).map((l) => l.text)
  const now = new Date('2026-10-31T12:10:00Z')

  it('чистый и синхронизированный — «всё в порядке»', () => {
    expect(texts(repo())).toEqual(['✓ Всё в порядке'])
    expect(radarNote({ ...repo(), branch: { ...repo().branch!, lastFetchAt: '2026-10-31T12:00:00Z' } }, now)).toMatch(/^Совпадает с upstream · по fetch 10\sмин/)
  })

  it('изменения, отставание и серверные факты', () => {
    const r = repo({ worktree: { staged: 1, untracked: 2 }, branch: { behind: 3 }, pr: { open: 2 }, ci: { active: 1 } })
    expect(texts(r)).toEqual(['◉ 3 локальных', '↓ 3 коммита', '◌ 1 CI работает', '↗ 2 PR/MR'])
    expect(radarNote(r)).toBe('staged 1 · untracked 2')
  })

  it('разошлась, упавший CI и незавершённая настройка — метки, но не «всё в порядке»', () => {
    expect(texts(repo({ branch: { ahead: 1, behind: 1 } }))).toEqual(['⇅ Ветка разошлась'])
    expect(texts(repo({ branch: { unborn: true, upstream: undefined, ahead: null, behind: null } }))).toEqual(['нет коммитов'])
    expect(texts(repo({ ci: { lastResult: 'failure' } }))).toEqual(['✕ Ошибка CI'])
    const noCli = repo({ pr: { ...unavailable, error: { code: ErrorCode.CodeCLIMissing, message: '' } }, ci: unavailable })
    expect(texts(noCli)).toEqual(['⚠ CLI не установлен'])
    expect(radarGroup(noCli)).toBe('ok')
  })

  it('конфликт и операция; без upstream — пояснение', () => {
    expect(texts(repo({ worktree: { conflicts: 2, operation: 'merge' } }))).toEqual(['2 конфликта', 'Незавершённый merge'])
    expect(radarNote(repo({ branch: { upstream: undefined, ahead: null, behind: null } }))).toBe('Upstream не настроен — синхронизацию сравнить не с чем')
    expect(radarNote(repo({ branch: { behind: 1 } }))).toMatch(/^В upstream 1 новый коммит · /)
  })
})

