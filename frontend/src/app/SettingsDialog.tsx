import AddIcon from '@mui/icons-material/Add'
import RemoveIcon from '@mui/icons-material/Remove'
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  MenuItem,
  Select,
  Switch,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material'
import { t } from '../i18n'
import { maxZoom, minZoom, zoomStep } from '../logic/zoom'
import { backgroundIntervals } from '../store/appStore'
import { useApp } from '../store/context'

// Настройки приложения: применяются и сохраняются сразу, кнопки «Сохранить» нет.
export function SettingsDialog({ open, onClose }: { open: boolean; onClose(): void }) {
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{t('settings.title')}</DialogTitle>
      <DialogContent sx={{ '& > :first-of-type': { mt: 0, pt: 0, borderTop: 'none' } }}>
        <BackgroundSetting />
        <FetchPruneSetting />
        <ZoomSetting />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('action.close')}</Button>
      </DialogActions>
    </Dialog>
  )
}

/** Фоновая проверка (D-063): интервал и fetch перед ней; применяются и сохраняются сразу. */
function BackgroundSetting() {
  const theme = useTheme()
  const minutes = useApp((s) => s.backgroundMinutes)
  const setMinutes = useApp((s) => s.setBackgroundMinutes)
  const autoFetch = useApp((s) => s.autoFetch)
  const setAutoFetch = useApp((s) => s.setAutoFetch)
  return (
    <Box sx={{ mt: 4, pt: 2, borderTop: `1px solid ${theme.app.line}` }}>
      <Typography variant="subtitle2">{t('settings.backgroundTitle')}</Typography>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mt: 1 }}>
        <Typography variant="body2">{t('settings.backgroundInterval')}</Typography>
        <Select
          size="small"
          value={minutes}
          onChange={(e) => setMinutes(Number(e.target.value))}
          inputProps={{ 'aria-label': t('settings.backgroundInterval') }}
        >
          {backgroundIntervals.map((m) => (
            <MenuItem key={m} value={m}>
              {t('settings.backgroundMinutes', { n: m })}
            </MenuItem>
          ))}
        </Select>
      </Box>
      <Typography variant="body2" sx={{ color: 'text.secondary', mt: 0.5 }}>
        {t('settings.backgroundDescription')}
      </Typography>
      <FormControlLabel
        sx={{ mt: 1.5 }}
        control={<Switch checked={autoFetch} onChange={(e) => setAutoFetch(e.target.checked)} />}
        label={<Typography variant="subtitle2">{t('settings.autoFetchTitle')}</Typography>}
      />
      <Typography variant="body2" sx={{ color: 'text.secondary', mt: 0.5 }}>
        {t('settings.autoFetchDescription')}
      </Typography>
    </Box>
  )
}

/** Fetch с --prune (D-057): применяется к следующим Fetch, сохраняется сразу. */
function FetchPruneSetting() {
  const theme = useTheme()
  const fetchPrune = useApp((s) => s.fetchPrune)
  const setFetchPrune = useApp((s) => s.setFetchPrune)
  return (
    <Box sx={{ mt: 4, pt: 2, borderTop: `1px solid ${theme.app.line}` }}>
      <FormControlLabel
        control={<Switch checked={fetchPrune} onChange={(e) => setFetchPrune(e.target.checked)} />}
        label={<Typography variant="subtitle2">{t('settings.pruneTitle')}</Typography>}
      />
      <Typography variant="body2" sx={{ color: 'text.secondary', mt: 0.5 }}>
        {t('settings.pruneDescription')}
      </Typography>
    </Box>
  )
}

/** Масштаб интерфейса (D-061): применяется и сохраняется сразу. */
function ZoomSetting() {
  const theme = useTheme()
  const zoom = useApp((s) => s.zoom)
  const setZoom = useApp((s) => s.setZoom)
  return (
    <Box sx={{ mt: 4, pt: 2, borderTop: `1px solid ${theme.app.line}` }}>
      <Typography variant="subtitle2">{t('settings.zoomTitle')}</Typography>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 1 }}>
        <Tooltip title={t('settings.zoomOut')}>
          <span>
            <IconButton aria-label={t('settings.zoomOut')} disabled={zoom <= minZoom} onClick={() => setZoom(zoom - zoomStep)}>
              <RemoveIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
        <Typography sx={{ minWidth: 56, textAlign: 'center', fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>{Math.round(zoom * 100)} %</Typography>
        <Tooltip title={t('settings.zoomIn')}>
          <span>
            <IconButton aria-label={t('settings.zoomIn')} disabled={zoom >= maxZoom} onClick={() => setZoom(zoom + zoomStep)}>
              <AddIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
        <Button size="small" disabled={zoom === minZoom} onClick={() => setZoom(minZoom)}>
          {t('settings.zoomReset')}
        </Button>
      </Box>
      <Typography variant="body2" sx={{ color: 'text.secondary', mt: 0.5 }}>
        {t('settings.zoomDescription')}
      </Typography>
    </Box>
  )
}

// Список пропусков правится черновиком: «Применить» сохраняет его в настройках
// Go и пересканирует корни (D-024). Пустой список Go сбрасывает к умолчанию.
