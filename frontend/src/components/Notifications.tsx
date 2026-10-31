import { Alert, Snackbar } from '@mui/material'
import type { AlertColor } from '@mui/material'
import type { Tone } from '../theme/theme'
import { useApp } from '../store/context'

const severity: Record<Tone, AlertColor> = { good: 'success', warn: 'warning', bad: 'error', info: 'info', muted: 'info' }

/** Уведомления по одному, в порядке появления; ошибки висят дольше. */
export function Notifications() {
  const notice = useApp((s) => s.notices[0])
  const dismiss = useApp((s) => s.dismissNotice)
  if (!notice) return null
  const close = () => dismiss(notice.id)
  return (
    <Snackbar
      key={notice.id}
      open
      autoHideDuration={notice.tone === 'bad' ? 10_000 : 5_000}
      onClose={(_, reason) => reason !== 'clickaway' && close()}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
    >
      <Alert severity={severity[notice.tone]} onClose={close} variant="filled" sx={{ maxWidth: 720, overflowWrap: 'anywhere' }}>
        {notice.text}
      </Alert>
    </Snackbar>
  )
}
