// Подписи для безопасных операций (fetch / ff-pull / switch).

import { ErrorCode, Operation, type OpResult, type Repository } from '../data/model'
import { t, type MessageKey } from '../i18n'
import { syncState, type Label } from './status'

const opKeys: Partial<Record<Operation, MessageKey>> = {
  [Operation.OperationFetch]: 'op.fetch',
  [Operation.OperationPull]: 'op.pull',
  [Operation.OperationSwitch]: 'op.switch',
}

export function opName(op: Operation): string {
  const key = opKeys[op]
  return key ? t(key) : op
}

/** Блокировки, после которых пользователю нужно лишь посмотреть свежий статус или подождать. */
const softCodes = new Set<string>([ErrorCode.CodeRepoBusy, ErrorCode.CodeStateChanged])

/** Итог последней операции: успех, блокировка с причиной или сбой Git. */
export function opResultLabel(res: OpResult): Label {
  const op = opName(res.op)
  if (!res.error) return { tone: 'good', text: t('op.done', { op, sec: Math.max(res.durationMs, 0) / 1000 }) }
  return { tone: softCodes.has(res.error.code) ? 'warn' : 'bad', text: `${t('op.failed', { op })}: ${res.error.message}` }
}

/**
 * Почему Pull сейчас не предлагается — по показанному статусу, как в
 * idea 04 §4; окончательно предусловия проверяет backend до вызова Git.
 * null — pull возможен.
 */
export function pullBlock(r: Repository): string | null {
  const b = r.branch
  const w = r.worktree
  if (!b || !w) return t('block.noStatus')
  if (w.conflicts > 0 || w.operation) return t('block.conflict')
  if (w.staged + w.unstaged + w.untracked > 0) return t('block.dirty')
  switch (syncState(b)) {
    case 'detached':
      return t('block.detached')
    case 'unborn':
      return t('block.unborn')
    case 'gone':
    case 'noUpstream':
      return t('block.noUpstream') // A08: pull не предлагается как готовое действие
    case 'diverged':
      return t('block.diverged')
    case 'inSync':
    case 'ahead':
      return t('block.nothingToPull')
  }
  return null
}

/** Почему переключение ветки сейчас невозможно; ветка-цель проверяется отдельно. */
export function switchBlock(r: Repository): string | null {
  const w = r.worktree
  if (!r.branch || !w) return t('block.noStatus')
  if (w.conflicts > 0 || w.operation) return t('block.conflict')
  if (w.staged + w.unstaged + w.untracked > 0) return t('block.dirty')
  return null
}
