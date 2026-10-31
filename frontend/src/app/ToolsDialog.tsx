import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, Tooltip, Typography, useTheme } from '@mui/material'
import { useState } from 'react'
import { formatRelative, t } from '../i18n'
import { availabilityTone, selfCheckCommands, toolHint } from '../logic/tools'
import { useApp } from '../store/context'
import { toneColor } from '../theme/theme'
import { monoFontFamily } from '../theme/tokens'

/** Команда для терминала с кнопкой копирования. */
function Command({ text }: { text: string }) {
  const theme = useTheme()
  const copyText = useApp((s) => s.copyText)
  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 0.5, bgcolor: theme.app.chip, border: `1px solid ${theme.app.chipLine}`, borderRadius: '8px', px: 1.25, py: 0.75, mt: 0.75 }}>
      <Box component="pre" sx={{ flex: 1, m: 0, font: `12px ${monoFontFamily}`, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
        {text}
      </Box>
      <Tooltip title={t('action.copy')}>
        <IconButton size="small" aria-label={t('action.copy')} onClick={() => void copyText(text)} sx={{ p: 0.25 }}>
          <ContentCopyIcon sx={{ fontSize: 14 }} />
        </IconButton>
      </Tooltip>
    </Box>
  )
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
        <Typography variant="body2" sx={{ color: 'text.secondary', mb: 1 }}>
          {t('tools.description')}
        </Typography>
        {tools.length === 0 && <Typography color="text.secondary">{t('tools.empty')}</Typography>}
        {tools.map((tool) => {
          const color = toneColor(theme, availabilityTone[tool.availability] ?? 'muted')
          const details = [tool.host, tool.version && t('tools.version', { version: tool.version }), tool.path].filter(Boolean).join(' · ')
          const hint = toolHint(tool)
          return (
            <Box key={`${tool.name}|${tool.host ?? ''}`} sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5, py: 1.5, borderBottom: `1px solid ${theme.app.line}` }}>
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
                {hint && (
                  <Typography variant="caption" sx={{ display: 'block', mt: 0.5 }}>
                    {hint.text}
                  </Typography>
                )}
                {hint?.command && <Command text={hint.command} />}
              </Box>
              <Box component="span" sx={{ fontSize: 11, fontWeight: 700, color, border: `1px solid ${color}`, borderRadius: '7px', px: 1, py: 0.5 }}>
                {t(`availability.${tool.availability || 'unavailable'}`)}
              </Box>
            </Box>
          )
        })}
        <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', mt: 2 }}>
          {t('tools.selfCheck')}
        </Typography>
        <Command text={selfCheckCommands} />
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
