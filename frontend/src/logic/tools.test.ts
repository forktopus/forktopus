import { describe, expect, it } from 'vitest'
import { Availability, ErrorCode, type ToolDiagnostic } from '../data/model'
import { gitProblem, toolHint, toolLabel } from './tools'

describe('toolHint', () => {
  it('готовому CLI подсказка не нужна', () => {
    expect(toolHint({ name: 'gh', availability: Availability.AvailabilityOK })).toBeNull()
  })

  it('нет CLI — где взять; нет авторизации — команда входа для хоста', () => {
    const missing = toolHint({ name: 'git', availability: Availability.AvailabilityUnavailable, error: { code: ErrorCode.CodeCLIMissing, message: '' } })
    expect(missing?.url).toMatch(/git-scm\.com/) // A28
    const auth = toolHint({
      name: 'glab',
      host: 'gitlab.com',
      availability: Availability.AvailabilityUnavailable,
      error: { code: ErrorCode.CodeAuthRequired, message: '' },
    })
    expect(auth?.command).toBe('glab auth login --hostname gitlab.com')
    const tea = toolHint({
      name: 'tea',
      host: 'git.example.org',
      availability: Availability.AvailabilityUnavailable,
      error: { code: ErrorCode.CodeAuthRequired, message: '' },
    })
    expect(tea?.command).toBe('tea login add --url https://git.example.org')
    const gt = toolHint({
      name: 'gt',
      host: 'git.example.org:3000',
      availability: Availability.AvailabilityUnavailable,
      error: { code: ErrorCode.CodeAuthRequired, message: '' },
    })
    expect(gt?.command).toBe('gt auth login --host https://git.example.org:3000')
    const missingGt = toolHint({ name: 'gt', availability: Availability.AvailabilityUnavailable, error: { code: ErrorCode.CodeCLIMissing, message: '' } })
    expect(missingGt?.url).toMatch(/gitea\.com\/gitea\/cli/) // D-074
  })

  it('нет сети — про сеть, а не про авторизацию', () => {
    const h = toolHint({
      name: 'gh',
      host: 'github.com',
      availability: Availability.AvailabilityError,
      error: { code: ErrorCode.CodeNetworkError, message: '' },
    })
    expect(h?.text).toMatch(/Нет связи с github\.com/)
  })
})

describe('git в PATH (A28)', () => {
  it('проблема — только когда проверка была и git не готов', () => {
    expect(gitProblem([])).toBeNull()
    expect(gitProblem([{ name: 'git', availability: Availability.AvailabilityOK }])).toBeNull()
    const missing = {
      name: 'git',
      availability: Availability.AvailabilityUnavailable,
      error: { code: ErrorCode.CodeCLIMissing, message: 'git не найден в PATH' },
    }
    expect(gitProblem([{ name: 'gh', availability: Availability.AvailabilityOK }, missing])).toBe(missing)
  })
})

// Эти подписи используются и в общей диагностике, и в карточке репозитория.
describe('причины отказа CLI', () => {
  it.each([
    [ErrorCode.CodeAccountBlocked, Availability.AvailabilityUnavailable, 'Вход запрещён', 'администратора'],
    [ErrorCode.CodeAccessDenied, Availability.AvailabilityNotSupported, 'Нет доступа', 'разрешения токена'],
    [ErrorCode.CodeUnsupported, Availability.AvailabilityNotSupported, 'Не поддерживается', null],
  ] as const)('различает %s и даёт подходящее действие', (code, availability, text, hintText) => {
    const diagnostic: ToolDiagnostic = { name: 'gt', host: 'git.example.org', availability, error: { code, message: 'ответ сервера' } }
    expect(toolLabel(diagnostic).text).toBe(text)
    const hint = toolHint(diagnostic)
    if (hintText) expect(hint?.text).toContain(hintText)
    else expect(hint).toBeNull()
    // Повторный вход и переустановка не снимают запрет аккаунта или отказ прав.
    expect(hint?.command).toBeUndefined()
    expect(hint?.url).toBeUndefined()
  })

  it('сохраняет готовность после успешной проверки', () => {
    expect(toolLabel({ name: 'gt', availability: Availability.AvailabilityOK }).text).toBe('Готов')
  })
})
