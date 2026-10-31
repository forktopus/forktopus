import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography, useTheme } from '@mui/material'
import { useState } from 'react'
import { Availability } from '../data/model'
import { formatRelative, t } from '../i18n'
import { useApp } from '../store/context'
import { toneColor, type Tone } from '../theme/theme'
import { monoFontFamily } from '../theme/tokens'

const availabilityTone: Record<Availability, Tone> = {
  [Availability.$zero]: 'muted',
  [Availability.AvailabilityOK]: 'good',
  [Availability.AvailabilityUnavailable]: 'warn',
  [Availability.AvailabilityError]: 'bad',
  [Availability.AvailabilityNotSupported]: 'muted',
}

export function ToolsDialog({ open, onClose }: { open: boolean; onClose(): void }) {
  const theme = useTheme()
  const tools = useApp((s) => s.tools)
  const checkedAt = useApp((s) => s.toolsCheckedAt)
  const checkTools = useApp((s) => s.checkTools)
  const [busy, setBusy] = useState(false)

  const recheck = async () => {
    setBusy(true)
    await checkTools()
    setBusy(false)
  }

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{t('tools.title')}</DialogTitle>
      <DialogContent>
        {tools.length === 0 && <Typography color="text.secondary">{t('tools.empty')}</Typography>}
        {tools.map((tool) => {
          const color = toneColor(theme, availabilityTone[tool.availability] ?? 'muted')
          const details = [tool.host, tool.version && t('tools.version', { version: tool.version }), tool.path].filter(Boolean).join(' · ')
          return (
            <Box key={`${tool.name}|${tool.host ?? ''}`} sx={{ display: 'flex', alignItems: 'center', gap: 1.5, py: 1.5, borderBottom: `1px solid ${theme.app.line}` }}>
              <Box
                component="code"
                sx={{ font: `12px ${monoFontFamily}`, bgcolor: theme.app.chip, border: `1px solid ${theme.app.chipLine}`, borderRadius: '7px', px: 1, py: 0.5 }}
              >
                {tool.name}
              </Box>
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Typography variant="body2" sx={{ fontWeight: 700 }}>
                  {details || t('tools.notFound')}
                </Typography>
                {tool.error && (
                  <Typography variant="caption" sx={{ color: 'text.secondary', display: 'block', overflowWrap: 'anywhere' }}>
                    {tool.error.message}
                  </Typography>
                )}
              </Box>
              <Box component="span" sx={{ fontSize: 11, fontWeight: 700, color, border: `1px solid ${color}`, borderRadius: '7px', px: 1, py: 0.5 }}>
                {t(`availability.${tool.availability || 'unavailable'}`)}
              </Box>
            </Box>
          )
        })}
        {checkedAt && (
          <Typography variant="caption" sx={{ color: 'text.secondary', display: 'block', mt: 1.5 }}>
            {t('lastCheck.at', { when: formatRelative(checkedAt) })}
          </Typography>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={() => void recheck()} disabled={busy}>
          {t('action.recheck')}
        </Button>
        <Button variant="contained" onClick={onClose}>
          {t('action.close')}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
