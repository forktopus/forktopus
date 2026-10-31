import { describe, expect, it } from 'vitest'
import { Availability, ErrorCode } from '../data/model'
import { repo, unavailable } from '../test/fixtures'
import { groupRadar, radarGroup } from './radar'

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
