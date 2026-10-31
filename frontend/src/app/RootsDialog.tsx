import CloseIcon from '@mui/icons-material/Close'
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, TextField, Tooltip, Typography, useTheme } from '@mui/material'
import { useState, type FormEvent } from 'react'
import { t } from '../i18n'
import { useApp } from '../store/context'
import { useRepos } from '../store/selectors'
import { monoFontFamily } from '../theme/tokens'

// Выбор папки нативным диалогом появится на шаге 05.6; пока путь вводится вручную.
export function RootsDialog({ open, onClose }: { open: boolean; onClose(): void }) {
  const theme = useTheme()
  const roots = useApp((s) => s.roots)
  const addRoot = useApp((s) => s.addRoot)
  const removeRoot = useApp((s) => s.removeRoot)
  const repos = useRepos()
  const [path, setPath] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!path.trim()) return
    setBusy(true)
    if (await addRoot(path.trim())) setPath('')
    setBusy(false)
  }

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{t('roots.title')}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" sx={{ color: 'text.secondary', mb: 2 }}>
          {t('roots.description')}
        </Typography>
        {roots.length === 0 && <Typography color="text.secondary">{t('roots.empty')}</Typography>}
        {roots.map((root) => (
          <Box key={root.id} sx={{ display: 'flex', alignItems: 'center', gap: 1.5, py: 1.25, borderBottom: `1px solid ${theme.app.line}` }}>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography sx={{ font: `13px ${monoFontFamily}`, overflowWrap: 'anywhere' }}>{root.path}</Typography>
              <Typography variant="caption" sx={{ color: 'text.secondary' }}>
                {t('roots.repoCount', { count: repos.filter((r) => r.rootId === root.id).length })}
              </Typography>
            </Box>
            <Tooltip title={t('action.remove')}>
              <IconButton aria-label={t('action.remove')} onClick={() => void removeRoot(root.id)}>
                <CloseIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        ))}
        <Box component="form" onSubmit={(e) => void submit(e)} sx={{ display: 'flex', gap: 1, mt: 3, alignItems: 'flex-start' }}>
          <TextField
            fullWidth
            size="small"
            label={t('roots.pathLabel')}
            placeholder={t('roots.pathPlaceholder')}
            value={path}
            onChange={(e) => setPath(e.target.value)}
            autoComplete="off"
          />
          <Button type="submit" variant="contained" disabled={busy || !path.trim()} sx={{ whiteSpace: 'nowrap' }}>
            {t('action.add')}
          </Button>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('action.close')}</Button>
      </DialogActions>
    </Dialog>
  )
}
