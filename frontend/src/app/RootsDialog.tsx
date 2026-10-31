import CloseIcon from '@mui/icons-material/Close'
import FolderOpenIcon from '@mui/icons-material/FolderOpen'
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, TextField, Tooltip, Typography, useTheme } from '@mui/material'
import { useState, type FormEvent } from 'react'
import type { ScanIssue } from '../data/model'
import { t } from '../i18n'
import { useApp } from '../store/context'
import { useRepos } from '../store/selectors'
import { monoFontFamily } from '../theme/tokens'

// Корень добавляется системным диалогом или вводом пути (запасной вариант, и
// единственный в демо-режиме браузера). После добавления запускается сканирование.
export function RootsDialog({ open, onClose }: { open: boolean; onClose(): void }) {
  const theme = useTheme()
  const roots = useApp((s) => s.roots)
  const addRoot = useApp((s) => s.addRoot)
  const pickAndAddRoot = useApp((s) => s.pickAndAddRoot)
  const removeRoot = useApp((s) => s.removeRoot)
  const scan = useApp((s) => s.scan)
  const scanning = useApp((s) => s.scanning)
  const issues = useApp((s) => s.issues)
  const repos = useRepos()
  const [path, setPath] = useState('')
  const [busy, setBusy] = useState(false)

  const pick = async () => {
    setBusy(true)
    await pickAndAddRoot()
    setBusy(false)
  }

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
              <RootIssues issues={issues.filter((i) => i.rootId === root.id)} />
            </Box>
            <Tooltip title={t('action.remove')}>
              <IconButton aria-label={t('action.remove')} onClick={() => void removeRoot(root.id)}>
                <CloseIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        ))}
        <Button variant="contained" startIcon={<FolderOpenIcon />} onClick={() => void pick()} disabled={busy} sx={{ mt: 3 }}>
          {t('roots.pick')}
        </Button>
        <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', mt: 2, mb: 1 }}>
          {t('roots.orManual')}
        </Typography>
        <Box component="form" onSubmit={(e) => void submit(e)} sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
          <TextField
            fullWidth
            size="small"
            label={t('roots.pathLabel')}
            placeholder={t('roots.pathPlaceholder')}
            value={path}
            onChange={(e) => setPath(e.target.value)}
            autoComplete="off"
          />
          <Button type="submit" variant="outlined" disabled={busy || !path.trim()} sx={{ whiteSpace: 'nowrap' }}>
            {t('action.add')}
          </Button>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => void scan()} disabled={scanning || roots.length === 0}>
          {scanning ? t('empty.scanning') : t('action.rescan')}
        </Button>
        <Button onClick={onClose}>{t('action.close')}</Button>
      </DialogActions>
    </Dialog>
  )
}

function RootIssues({ issues }: { issues: ScanIssue[] }) {
  const [all, setAll] = useState(false)
  if (issues.length === 0) return null
  return (
    <Box sx={{ mt: 0.5 }}>
      <Typography variant="caption" sx={{ color: 'warning.main', display: 'block' }}>
        {t('roots.issues', { count: issues.length })}
      </Typography>
      {(all ? issues : issues.slice(0, 3)).map((i) => (
        <Typography key={i.path} variant="caption" sx={{ display: 'block', color: 'text.secondary', font: `11px ${monoFontFamily}`, overflowWrap: 'anywhere' }}>
          {i.error?.message ?? i.path}
        </Typography>
      ))}
      {issues.length > 3 && (
        <Button size="small" onClick={() => setAll(!all)} sx={{ px: 0, minWidth: 0 }}>
          {all ? t('roots.issuesLess') : t('roots.issuesMore', { count: issues.length - 3 })}
        </Button>
      )}
    </Box>
  )
}
