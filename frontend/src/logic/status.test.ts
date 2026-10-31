import { describe, expect, it } from 'vitest'
import { Availability } from '../data/model'
import { repo, unavailable } from '../test/fixtures'
import { changesLabel, ciLabel, hasChanges, integrationIssue, isBehind, prLabel, remoteCount, syncLabel, syncState } from './status'

describe('unavailable не отображается как 0', () => {
  it('PR/CI без данных — «нет данных», а не 0', () => {
    const r = repo({ pr: unavailable, ci: unavailable })
    expect(remoteCount(r.pr, r.pr.open)).toBeNull()
    expect(prLabel(r).text).toBe('нет данных')
    expect(ciLabel(r).text).toBe('нет данных')
    expect(prLabel(r).text).not.toMatch(/0/)
  })

  it('число, оставшееся от прошлого ответа, не показывается при недоступном источнике', () => {
    const r = repo({ pr: { availability: Availability.AvailabilityError, open: 3 } })
    expect(remoteCount(r.pr, r.pr.open)).toBeNull()
    expect(prLabel(r)).toEqual({ tone: 'bad', text: 'ошибка источника' })
  })

  it('not_supported — отдельный текст', () => {
    const r = repo({ ci: { availability: Availability.AvailabilityNotSupported, active: null } })
    expect(ciLabel(r).text).toBe('не поддерживается')
  })

  it('0 показывается только при успешном ответе', () => {
    const r = repo({ pr: { open: 0 } })
    expect(remoteCount(r.pr, r.pr.open)).toBe(0)
    expect(prLabel(r).text).toBe('нет открытых')
  })
})

describe('статус не прочитан', () => {
  it('не выдаётся за «чисто» и «совпадает»', () => {
    const r = { ...repo(), branch: null, worktree: undefined }
    expect(changesLabel(r)).toEqual({ tone: 'muted', text: 'статус не прочитан' })
    expect(syncLabel(r).text).toBe('статус не прочитан')
    expect(hasChanges(r)).toBe(false)
    expect(isBehind(r)).toBe(false)
  })
})

describe('синхронизация с upstream', () => {
  it('состояния', () => {
    expect(syncState(repo().branch)).toBe('inSync')
    expect(syncState(repo({ branch: { ahead: 2 } }).branch)).toBe('ahead')
    expect(syncState(repo({ branch: { behind: 1 } }).branch)).toBe('behind')
    expect(syncState(repo({ branch: { ahead: 1, behind: 1 } }).branch)).toBe('diverged')
    expect(syncState(repo({ branch: { detached: true, name: undefined } }).branch)).toBe('detached')
  })

  it('без upstream ahead/behind не выдумываются (A08)', () => {
    const r = repo({ branch: { upstream: undefined, ahead: null, behind: null } })
    expect(syncState(r.branch)).toBe('noUpstream')
    expect(isBehind(r)).toBe(false)
    expect(syncLabel(r).text).toBe('нет upstream')
  })

  it('подписи с числами', () => {
    expect(syncLabel(repo({ branch: { behind: 3 } })).text).toBe('↓ 3 отстаёт')
    expect(syncLabel(repo({ branch: { ahead: 2, behind: 1 } })).tone).toBe('bad')
  })
})

describe('локальные изменения', () => {
  it('чисто / изменения / конфликты', () => {
    expect(changesLabel(repo()).text).toBe('Чисто')
    expect(changesLabel(repo({ worktree: { staged: 1, unstaged: 1, untracked: 1 } })).text).toBe('3 изменения')
    expect(changesLabel(repo({ worktree: { conflicts: 2 } }))).toEqual({ tone: 'bad', text: '2 конфликта' })
  })
})

describe('интеграция', () => {
  it('нет CLI или авторизации — проблема настройки, not_supported — нет', () => {
    expect(integrationIssue(repo({ pr: unavailable }))).toBe(true)
    expect(integrationIssue(repo({ ci: { availability: Availability.AvailabilityError } }))).toBe(true)
    expect(
      integrationIssue(
        repo({
          pr: { availability: Availability.AvailabilityNotSupported },
          ci: { availability: Availability.AvailabilityNotSupported },
        }),
      ),
    ).toBe(false)
  })
})
