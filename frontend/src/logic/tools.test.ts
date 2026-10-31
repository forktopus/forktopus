import { describe, expect, it } from 'vitest'
import { Availability, ErrorCode } from '../data/model'
import { toolHint } from './tools'

describe('toolHint', () => {
  it('готовому CLI подсказка не нужна', () => {
    expect(toolHint({ name: 'gh', availability: Availability.AvailabilityOK })).toBeNull()
  })

  it('нет CLI — где взять; нет авторизации — команда входа для хоста', () => {
    const missing = toolHint({ name: 'git', availability: Availability.AvailabilityUnavailable, error: { code: ErrorCode.CodeCLIMissing, message: '' } })
    expect(missing?.text).toMatch(/git-scm\.com/) // A28
    const auth = toolHint({ name: 'glab', host: 'gitlab.com', availability: Availability.AvailabilityUnavailable, error: { code: ErrorCode.CodeAuthRequired, message: '' } })
    expect(auth?.command).toBe('glab auth login --hostname gitlab.com')
    const tea = toolHint({ name: 'tea', host: 'git.example.org', availability: Availability.AvailabilityUnavailable, error: { code: ErrorCode.CodeAuthRequired, message: '' } })
    expect(tea?.command).toBe('tea login add --url https://git.example.org')
  })

  it('нет сети — про сеть, а не про авторизацию', () => {
    const h = toolHint({ name: 'gh', host: 'github.com', availability: Availability.AvailabilityError, error: { code: ErrorCode.CodeNetworkError, message: '' } })
    expect(h?.text).toMatch(/Нет связи с github\.com/)
  })
})
