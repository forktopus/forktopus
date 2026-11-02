import { describe, expect, it } from 'vitest'
import { Availability, ErrorCode } from '../data/model'
import { repo, unavailable } from '../test/fixtures'
import {
  changesDetail,
  changesLabel,
  ciLabel,
  fileLabel,
  hasChanges,
  integrationIssue,
  isBehind,
  prLabel,
  remoteCaption,
  remoteCount,
  remoteDetail,
  syncFreshness,
  syncLabel,
  syncState,
} from './status'

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

describe('последний закрытый PR, когда открытых нет', () => {
  const now = new Date('2026-11-01T12:00:00Z')
  const checked = '2026-11-01T11:58:00Z'

  it('влитый — номер и когда', () => {
    const r = repo({ pr: { open: 0, lastAttemptAt: checked, last: { number: 5, merged: true, url: 'u5', closedAt: '2026-10-30T12:00:00Z' } } })
    expect(prLabel(r, now)).toEqual({ tone: 'muted', text: '#5 влит позавчера' })
    expect(remoteDetail(r.pr, now)).toBe('нет открытых · проверено 2 мин. назад')
  })

  it('влитый без времени', () => {
    expect(prLabel(repo({ pr: { open: 0, last: { number: 5, merged: true } } }), now).text).toBe('#5 влит')
  })

  it('закрытый без слияния — время в пояснении', () => {
    const r = repo({ pr: { open: 0, lastAttemptAt: checked, last: { number: 3, merged: false, closedAt: '2026-11-01T09:00:00Z' } } })
    expect(prLabel(r, now).text).toBe('#3 закрыт без слияния')
    expect(remoteDetail(r.pr, now)).toBe('нет открытых · закрыт 3 ч назад · проверено 2 мин. назад')
  })

  it('есть открытые — последний закрытый не показывается', () => {
    expect(prLabel(repo({ pr: { open: 1, draft: 0, last: { number: 3, merged: false } } }), now).text).toBe('1 открыт')
  })

  it('при ошибке источника — причина, а не прошлый PR', () => {
    const r = repo({ pr: { availability: Availability.AvailabilityError, open: null, last: { number: 3, merged: true } } })
    expect(prLabel(r, now).text).toBe('ошибка источника')
  })
})

describe('причина отсутствия данных PR/CI', () => {
  const err = (availability: Availability, code: ErrorCode) => ({ availability, open: null, active: null, error: { code, message: `${code}: подробности` } })

  it('нет CLI (A16), нет авторизации (A17), не поддерживается (A18) различаются', () => {
    expect(prLabel(repo({ pr: err(Availability.AvailabilityUnavailable, ErrorCode.CodeCLIMissing) }))).toEqual({ tone: 'warn', text: 'CLI не установлен' })
    expect(ciLabel(repo({ ci: err(Availability.AvailabilityUnavailable, ErrorCode.CodeAuthRequired) })).text).toBe('нужна авторизация')
    expect(ciLabel(repo({ ci: err(Availability.AvailabilityNotSupported, ErrorCode.CodeUnsupported) })).text).toBe('не поддерживается')
    // Вход есть, прав на раздел нет (HTTP 403, D-068): не проблема настройки CLI.
    const denied = repo({ ci: err(Availability.AvailabilityNotSupported, ErrorCode.CodeAccessDenied) })
    expect(ciLabel(denied)).toEqual({ tone: 'muted', text: 'нет прав' })
    expect(integrationIssue(denied)).toBe(false)
    expect(prLabel(repo({ pr: err(Availability.AvailabilityNotSupported, ErrorCode.CodeNoRemote) })).text).toBe('нет remote')
    expect(prLabel(repo({ pr: err(Availability.AvailabilityError, ErrorCode.CodeNetworkError) }))).toEqual({ tone: 'bad', text: 'нет связи с сервером' })
    expect(prLabel(repo({ pr: err(Availability.AvailabilityError, ErrorCode.CodeOther) })).text).toBe('ошибка источника')
    expect(prLabel(repo({ pr: { availability: Availability.$zero, open: null } })).text).toBe('ещё не запрошено')
  })

  it('пояснение: сообщение ошибки и возраст последних данных (A24)', () => {
    const now = new Date('2026-10-31T12:00:00Z')
    const pr = { ...err(Availability.AvailabilityError, ErrorCode.CodeNetworkError), lastSuccessAt: '2026-10-31T10:00:00Z' }
    expect(remoteDetail(pr, now)).toMatch(/^NETWORK_ERROR: подробности · последние данные 2\sч\.?\sназад$/) // Intl ставит неразрывный пробел
    expect(remoteDetail({ availability: Availability.AvailabilityOK, lastAttemptAt: '2026-10-31T11:55:00Z' }, now)).toMatch(/^проверено 5\sмин\.?\sназад$/)
  })

  it('черновики видны в подписи PR (A19)', () => {
    expect(prLabel(repo({ pr: { open: 2, draft: 1 } })).text).toBe('2 открыто · 1 черновик')
  })
})

describe('детализация', () => {
  it('staged, unstaged и untracked — отдельно (A07)', () => {
    expect(changesDetail(repo({ worktree: { staged: 1, unstaged: 1, untracked: 1 } }))).toBe('staged 1 · unstaged 1 · untracked 1')
    expect(changesDetail(repo())).toBe('')
  })

  it('ahead/behind подписаны временем fetch, а не «сейчас на сервере» (A09)', () => {
    const now = new Date('2026-10-31T12:00:00Z')
    expect(syncFreshness(repo({ branch: { lastFetchAt: '2026-10-29T12:00:00Z' } }), now)).toBe('по fetch позавчера')
    expect(syncFreshness(repo(), now)).toBe('fetch ещё не выполнялся')
    expect(syncFreshness(repo({ branch: { upstream: undefined } }), now)).toBe('')
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

  it('без коммитов и с удалённым upstream — без выдуманных чисел', () => {
    const unborn = repo({ branch: { unborn: true, headSha: undefined, upstream: undefined, ahead: null, behind: null } })
    expect(syncState(unborn.branch)).toBe('unborn')
    expect(syncLabel(unborn).text).toBe('нет коммитов')
    const gone = repo({ branch: { upstreamGone: true, ahead: null, behind: null } })
    expect(syncState(gone.branch)).toBe('gone')
    expect(syncLabel(gone)).toEqual({ tone: 'warn', text: 'upstream удалён' })
    expect(isBehind(gone)).toBe(false)
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

describe('подпись без сервера', () => {
  it('нет remote, remote — локальный путь, есть хост', () => {
    expect(remoteCaption(repo({ remotePath: null }))).toBe('без remote')
    const local = repo()
    local.remotes = [{ ...local.remotes![0], host: '', fetchUrl: '/srv/remotes/demo.git' }]
    expect(remoteCaption(local)).toBe('remote — локальный путь')
    expect(remoteCaption(repo())).toBe('')
  })
})

describe('изменённые файлы', () => {
  const label = (code: string) => fileLabel({ path: 'a', code })
  it('подпись и цвет по коду XY', () => {
    expect(label('??')).toEqual({ tone: 'warn', text: 'новый' })
    expect(label('M ')).toEqual({ tone: 'info', text: 'изменён' })
    expect(label(' M')).toEqual({ tone: 'warn', text: 'изменён' })
    expect(label('MM')).toEqual({ tone: 'warn', text: 'изменён' })
    expect(label('A ')).toEqual({ tone: 'info', text: 'добавлен' })
    expect(label(' D')).toEqual({ tone: 'warn', text: 'удалён' })
    expect(label('R ')).toEqual({ tone: 'info', text: 'переименован' })
    expect(label('C ')).toEqual({ tone: 'info', text: 'скопирован' })
  })
  it('все коды unmerged — конфликт', () => {
    for (const code of ['DD', 'AU', 'UD', 'UA', 'DU', 'AA', 'UU']) expect(label(code)).toEqual({ tone: 'bad', text: 'конфликт' })
  })
})

// Блокировка аккаунта требует вмешательства и не превращается в нулевые PR/CI.
it('запрет входа виден у PR/CI и считается проблемой подключения', () => {
  const error = { code: ErrorCode.CodeAccountBlocked, message: 'аккаунт заблокирован' }
  const r = repo({
    pr: { availability: Availability.AvailabilityUnavailable, error, open: 3 },
    ci: { availability: Availability.AvailabilityUnavailable, error, active: 2 },
  })
  expect(prLabel(r)).toEqual({ tone: 'warn', text: 'вход запрещён' })
  expect(ciLabel(r)).toEqual({ tone: 'warn', text: 'вход запрещён' })
  expect(integrationIssue(r)).toBe(true)
})
