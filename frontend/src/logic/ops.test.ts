import { describe, expect, it } from 'vitest'
import { ErrorCode, Operation, type OpResult } from '../data/model'
import { repo } from '../test/fixtures'
import { opResultLabel } from './ops'

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
