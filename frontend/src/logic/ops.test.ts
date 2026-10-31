import { describe, expect, it } from 'vitest'
import { ErrorCode, Operation, type OpResult } from '../data/model'
import { repo } from '../test/fixtures'
import { opResultLabel, pullBlock, switchBlock } from './ops'

const result = (patch: Partial<OpResult>): OpResult => ({ repoId: 'x', op: Operation.OperationFetch, at: '', durationMs: 1250, repo: repo(), ...patch })

describe('итог операции', () => {
  it('успех с длительностью', () => {
    expect(opResultLabel(result({}))).toEqual({ tone: 'good', text: 'Fetch: готово за 1,25 с' })
  })

  it('блокировка показывает причину, а не «готово»', () => {
    const label = opResultLabel(result({ op: Operation.OperationPull, error: { code: ErrorCode.CodeDirtyWorktree, message: 'есть локальные изменения' } }))
    expect(label).toEqual({ tone: 'bad', text: 'Pull не выполнен: есть локальные изменения' })
  })

  it('«занято» и «состояние изменилось» — предупреждение', () => {
    expect(opResultLabel(result({ error: { code: ErrorCode.CodeRepoBusy, message: 'занято' } })).tone).toBe('warn')
    expect(opResultLabel(result({ op: Operation.OperationSwitch, error: { code: ErrorCode.CodeStateChanged, message: 'изменилось' } })).tone).toBe('warn')
  })
})

describe('pullBlock / switchBlock', () => {
  it('pull предлагается только на чистой ветке, которая отстаёт по последнему fetch', () => {
    expect(pullBlock(repo({ branch: { behind: 2 } }))).toBeNull()
    expect(pullBlock(repo({ branch: { behind: 2 }, worktree: { untracked: 1 } }))).toMatch(/локальные изменения/)
    expect(pullBlock(repo({ branch: { behind: 2 }, worktree: { operation: 'merge' } }))).toMatch(/конфликты/)
    expect(pullBlock(repo({ branch: { upstream: undefined, ahead: null, behind: null } }))).toMatch(/нет upstream/) // A08
    expect(pullBlock(repo({ branch: { name: undefined, detached: true } }))).toMatch(/detached/)
    expect(pullBlock(repo({ branch: { ahead: 1, behind: 1 } }))).toMatch(/разошлась/)
    expect(pullBlock(repo())).toMatch(/сначала Fetch/)
    expect(pullBlock({ ...repo(), worktree: null })).toMatch(/не прочитан/)
  })

  it('switch блокируется изменениями, но не отставанием', () => {
    expect(switchBlock(repo({ branch: { behind: 3 } }))).toBeNull()
    expect(switchBlock(repo({ worktree: { staged: 1 } }))).toMatch(/локальные изменения/)
  })
})
