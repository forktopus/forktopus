import { Alert, Box, Button, LinearProgress, Typography, useTheme } from '@mui/material'
import { useEffect } from 'react'
import { t, formatDateTime } from '../i18n'
import { updateActions } from '../logic/updates'
import { useApp } from '../store/context'

/** Наблюдение хранится в общем store и источнике, поэтому возвращение в
 * настройки показывает продолжающуюся загрузку без второго запроса latest. */
export function AppUpdates({ open }: { open: boolean }) {
  const theme = useTheme()
  const state = useApp((s) => s.appUpdate)
  const load = useApp((s) => s.loadAppUpdate)
  const check = useApp((s) => s.checkAppUpdate)
  const download = useApp((s) => s.downloadAppUpdate)
  const cancel = useApp((s) => s.cancelAppUpdate)
  const folder = useApp((s) => s.openAppUpdateFolder)
  const openURL = useApp((s) => s.openURL)
  useEffect(() => {
    if (open) void load()
  }, [open, load])
  if (!state) return null
  const actions = updateActions(state)
  // Инструкция установки относится к найденному пакету; до его появления
  // оставляем только результат проверки и доступные действия.
  const hasPackage = Boolean(state.version && state.filename)
  return (
    <Box sx={{ mt: 4, pt: 2, borderTop: `1px solid ${theme.app.line}` }}>
      <Typography variant="subtitle2">{t('updates.title')}</Typography>
      <Typography variant="body2" sx={{ mt: 1 }}>
        {t('updates.current', { version: state.currentVersion })}
      </Typography>
      {state.version && (
        <Typography variant="body2" sx={{ mt: 1 }}>
          {t('updates.available', { version: state.version })}
        </Typography>
      )}
      {state.filename && (
        <Typography variant="body2" sx={{ mt: 0.5, overflowWrap: 'anywhere', color: 'text.secondary' }}>
          {state.filename} · {(state.size / 1024 / 1024).toFixed(1)} {t('updates.mib')}
        </Typography>
      )}
      {state.status === 'checking' && (
        <Typography variant="body2" role="status" sx={{ mt: 1 }}>
          {t('updates.checking')}
        </Typography>
      )}
      {actions.cancel && (
        <Box sx={{ mt: 1 }}>
          <LinearProgress
            variant={state.status === 'verifying' ? 'indeterminate' : 'determinate'}
            value={actions.progress}
            aria-label={t('updates.progress')}
          />
          <Typography variant="body2" role="status" sx={{ mt: 0.5 }}>
            {state.status === 'verifying' ? t('updates.verifying') : t('updates.downloaded', { percent: actions.progress })}
          </Typography>
        </Box>
      )}
      {state.message && (
        <Alert severity={state.status === 'error' ? 'error' : state.status === 'ready' ? 'success' : 'info'} sx={{ mt: 1 }}>
          {state.message}
        </Alert>
      )}
      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mt: 1 }}>
        <Button size="small" disabled={!actions.check} onClick={() => void check()}>
          {t('updates.check')}
        </Button>
        {actions.download && (
          <Button size="small" variant="contained" onClick={() => void download()}>
            {t('updates.download')}
          </Button>
        )}
        {actions.cancel && (
          <Button size="small" onClick={() => void cancel()}>
            {t('updates.cancel')}
          </Button>
        )}
        {actions.folder && (
          <Button size="small" variant="contained" onClick={() => void folder()}>
            {t('updates.folder')}
          </Button>
        )}
        {state.releaseUrl && (
          <Button size="small" onClick={() => openURL(state.releaseUrl)}>
            {t('updates.notes')}
          </Button>
        )}
      </Box>
      {hasPackage && state.instruction && (
        <Typography variant="body2" sx={{ mt: 1, color: 'text.secondary' }}>
          {state.instruction}
        </Typography>
      )}
      {state.checkedAt && (
        <Typography variant="caption" sx={{ display: 'block', mt: 1, color: 'text.secondary' }}>
          {t('updates.checked', { at: formatDateTime(state.checkedAt) })}
        </Typography>
      )}
      {hasPackage && (
        <Typography variant="caption" sx={{ display: 'block', mt: 1, color: 'text.secondary' }}>
          {t('updates.manual')}
        </Typography>
      )}
    </Box>
  )
}
