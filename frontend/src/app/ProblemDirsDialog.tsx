// Каталоги, которые сканер не смог прочитать (З-4, D-067): у каждого — путь,
// причина и действия. Сам каталог недоступен, поэтому папка и терминал
// открываются в родительском (или ближайшем существующем выше).

import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography, useTheme } from '@mui/material'
import { PathButtons } from '../components/PathButtons'
import { IssueReason } from '../data/model'
import { t, type MessageKey } from '../i18n'
import { parentPath } from '../logic/repo'
import { useApp } from '../store/context'
import { monoFontFamily } from '../theme/tokens'

const reasonText: Record<IssueReason, MessageKey> = {
  [IssueReason.$zero]: 'problems.reason.other',
  [IssueReason.IssueNoAccess]: 'problems.reason.noAccess',
  [IssueReason.IssueNotFound]: 'problems.reason.notFound',
  [IssueReason.IssueOther]: 'problems.reason.other',
}

export function ProblemDirsDialog({ open, onClose }: { open: boolean; onClose(): void }) {
  const theme = useTheme()
  const issues = useApp((s) => s.issues)
  const skipPath = useApp((s) => s.skipPath)

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="md">
      <DialogTitle>{t('problems.title')}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" sx={{ color: 'text.secondary', mb: 1 }}>
          {t('problems.description')}
        </Typography>
        {issues.length === 0 && <Typography sx={{ color: 'text.secondary', py: 2 }}>{t('problems.empty')}</Typography>}
        {issues.map((i) => (
          <Box key={i.path} sx={{ py: 1.5, borderBottom: `1px solid ${theme.app.line}` }}>
            <Typography sx={{ font: `14px ${monoFontFamily}`, overflowWrap: 'anywhere' }}>{i.path}</Typography>
            <Typography variant="body2" sx={{ color: 'warning.main', mt: 0.25 }}>
              {t(reasonText[i.reason ?? IssueReason.$zero])}
            </Typography>
            {i.error?.message && (
              <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', font: `12px ${monoFontFamily}`, overflowWrap: 'anywhere' }}>
                {i.error.message}
              </Typography>
            )}
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mt: 1 }}>
              <PathButtons path={parentPath(i.path)} />
              <Button size="small" color="warning" onClick={() => void skipPath(i.path)} sx={{ ml: 'auto' }}>
                {t('problems.skip')}
              </Button>
            </Box>
          </Box>
        ))}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('action.close')}</Button>
      </DialogActions>
    </Dialog>
  )
}
