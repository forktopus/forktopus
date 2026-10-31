// Подписи для безопасных операций (fetch / ff-pull / switch).

import { ErrorCode, Operation, type OpResult } from '../data/model'
import { t, type MessageKey } from '../i18n'
import type { Label } from './status'

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
