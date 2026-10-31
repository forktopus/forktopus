// Подтверждение Pull и Switch: репозиторий, путь, ветка, операция и её эффект.
// Команды показаны как есть — пользователь видит, что именно выполнит Git.

import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography, useTheme } from '@mui/material'
import { Operation, type Repository } from '../data/model'
import { t } from '../i18n'
import { repoName } from '../logic/repo'
import type { PendingOp } from '../store/appStore'
import { useApp } from '../store/context'
import { monoFontFamily } from '../theme/tokens'
import { caption } from './status'

/** Команды Git, как их запускает internal/gitops (без shell). */
export function opCommands(repo: Repository, p: PendingOp): string {
  switch (p.op) {
    case Operation.OperationPull:
      return 'git pull --ff-only --no-rebase --no-autostash'
    case Operation.OperationSwitch:
      return `git switch --no-guess ${p.branch ?? ''}`
    default:
      return `git fetch --no-prune ${repo.remote ?? ''}`
  }
}

function opEffect(repo: Repository, p: PendingOp): string {
  const branch = repo.branch?.name ?? ''
  if (p.op === Operation.OperationPull) return t('confirm.pullEffect', { branch, upstream: repo.branch?.upstream ?? '' })
  if (p.op === Operation.OperationSwitch) return t('confirm.switchEffect', { from: branch || (repo.branch?.headSha ?? '').slice(0, 8), to: p.branch ?? '' })
  return t('confirm.fetchEffect', { remote: repo.remote ?? '' })
}

const titles = {
  [Operation.OperationPull]: 'confirm.pullTitle',
  [Operation.OperationSwitch]: 'confirm.switchTitle',
  [Operation.OperationFetch]: 'confirm.fetchTitle',
} as const

export function OpConfirmDialog() {
  const theme = useTheme()
  const pending = useApp((s) => s.pendingOp)
  const repo = useApp((s) => (s.pendingOp ? s.repos[s.pendingOp.id] : undefined))
  const confirmOp = useApp((s) => s.confirmOp)
  const cancelOp = useApp((s) => s.cancelOp)
  const open = pending !== null && repo !== undefined && pending.op !== Operation.$zero

  return (
    <Dialog open={open} onClose={cancelOp} fullWidth maxWidth="sm">
      {pending && repo && pending.op !== Operation.$zero && (
        <>
          <DialogTitle>{t(titles[pending.op])}</DialogTitle>
          <DialogContent>
            <Box sx={{ display: 'grid', gridTemplateColumns: '110px minmax(0, 1fr)', gap: 1, alignItems: 'baseline' }}>
              <Typography sx={caption}>{t('column.repo')}</Typography>
              <Typography sx={{ fontWeight: 700 }}>{repoName(repo)}</Typography>
              <Typography sx={caption}>{t('confirm.path')}</Typography>
              <Typography sx={{ font: `12px ${monoFontFamily}`, overflowWrap: 'anywhere' }}>{repo.path}</Typography>
              <Typography sx={caption}>{t('column.branch')}</Typography>
              <Typography sx={{ font: `12px ${monoFontFamily}` }}>
                {pending.op === Operation.OperationSwitch ? `${repo.branch?.name ?? t('sync.detached')} → ${pending.branch}` : (repo.branch?.name ?? t('sync.detached'))}
              </Typography>
            </Box>
            <Typography sx={{ mt: 2 }}>{opEffect(repo, pending)}</Typography>
            <Box component="pre" sx={{ font: `12px ${monoFontFamily}`, bgcolor: theme.app.chip, border: `1px solid ${theme.app.chipLine}`, borderRadius: '8px', p: 1.5, mt: 2, mb: 0, whiteSpace: 'pre-wrap' }}>
              {opCommands(repo, pending)}
            </Box>
            <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', mt: 1.5 }}>
              {t('confirm.safety')}
            </Typography>
          </DialogContent>
          <DialogActions>
            <Button onClick={cancelOp}>{t('action.cancel')}</Button>
            <Button variant="contained" onClick={() => void confirmOp()} autoFocus>
              {t(pending.op === Operation.OperationPull ? 'action.confirmPull' : 'action.confirmSwitch')}
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  )
}
