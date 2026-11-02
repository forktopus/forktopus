import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, Link, Tooltip, Typography, useTheme } from '@mui/material'
import { useEffect, useState } from 'react'
import type { Diagnostics } from '../data/model'
import { formatRelative, formatTime, t } from '../i18n'
import { selfCheckCommands, toolHint, toolLabel } from '../logic/tools'
import { useApp } from '../store/context'
import { toneColor } from '../theme/theme'
import { monoFontFamily } from '../theme/tokens'

/** Команда для терминала с кнопкой копирования. */
function Command({ text }: { text: string }) {
  const theme = useTheme()
  const copyText = useApp((s) => s.copyText)
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'flex-start',
        gap: 0.5,
        bgcolor: theme.app.chip,
        border: `1px solid ${theme.app.chipLine}`,
        borderRadius: '8px',
        px: 1.25,
        py: 0.75,
        mt: 0.75,
      }}
    >
      <Box component="pre" sx={{ flex: 1, m: 0, font: `13px ${monoFontFamily}`, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
        {text}
      </Box>
      <Tooltip title={t('action.copy')}>
        <IconButton size="small" aria-label={t('action.copy')} onClick={() => void copyText(text)} sx={{ p: 0.25 }}>
          <ContentCopyIcon sx={{ fontSize: 15 }} />
        </IconButton>
      </Tooltip>
    </Box>
  )
}

/** ОС, пути и последние ошибки из отчёта диагностики. */
function SystemInfo({ diag }: { diag: Diagnostics }) {
  const theme = useTheme()
  const rows: [string, string][] = [
    [t('tools.os'), diag.os],
    [t('tools.config'), diag.configPath],
  ]
  if (diag.logPath) rows.push([t('tools.log'), diag.logPath])
  const errors = diag.errors ?? []
  return (
    <Box sx={{ mt: 2.5 }}>
      <Typography variant="body2" sx={{ fontWeight: 700, mb: 0.75 }}>
        {t('tools.system')}
      </Typography>
      {rows.map(([label, value]) => (
        <Typography key={label} variant="caption" sx={{ display: 'block', overflowWrap: 'anywhere' }}>
          <Box component="span" sx={{ color: 'text.secondary' }}>
            {label}:
          </Box>{' '}
          <Box component="span" sx={{ fontFamily: monoFontFamily }}>
            {value}
          </Box>
        </Typography>
      ))}
      <Typography variant="body2" sx={{ fontWeight: 700, mt: 2, mb: 0.75 }}>
        {t('tools.errors')}
      </Typography>
      {errors.length === 0 && (
        <Typography variant="caption" sx={{ color: 'text.secondary' }}>
          {t('tools.noErrors')}
        </Typography>
      )}
      {errors.slice(0, 5).map((e, i) => (
        <Typography
          key={i}
          variant="caption"
          sx={{ display: 'block', font: `12.5px ${monoFontFamily}`, overflowWrap: 'anywhere', py: 0.5, borderBottom: `1px solid ${theme.app.line}` }}
        >
          {formatTime(e.time)} {e.message}
        </Typography>
      ))}
      <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', mt: 1.5 }}>
        {t('tools.reportHint')}
      </Typography>
    </Box>
  )
}

export function ToolsDialog({ open, onClose }: { open: boolean; onClose(): void }) {
  const theme = useTheme()
  const tools = useApp((s) => s.tools)
  const checkedAt = useApp((s) => s.toolsCheckedAt)
  const checkTools = useApp((s) => s.checkTools)
  const loadDiagnostics = useApp((s) => s.loadDiagnostics)
  const copyText = useApp((s) => s.copyText)
  const openURL = useApp((s) => s.openURL)
  const [busy, setBusy] = useState(false)
  const [diag, setDiag] = useState<Diagnostics | null>(null)

  // Отчёт перечитывается при каждом открытии: в нём свежие ошибки и операции.
  useEffect(() => {
    if (!open) return
    let alive = true
    void loadDiagnostics().then((d) => alive && setDiag(d))
    return () => {
      alive = false
    }
  }, [open, loadDiagnostics])

  const recheck = async () => {
    setBusy(true)
    await checkTools()
    setDiag(await loadDiagnostics())
    setBusy(false)
  }

  const copyReport = async () => {
    const d = await loadDiagnostics()
    if (!d) return
    setDiag(d)
    await copyText(d.report)
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
          const label = toolLabel(tool)
          const color = toneColor(theme, label.tone)
          const details = [tool.host, tool.version && t('tools.version', { version: tool.version }), tool.path].filter(Boolean).join(' · ')
          const hint = toolHint(tool)
          return (
            <Box
              key={`${tool.name}|${tool.host ?? ''}`}
              sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5, py: 1.5, borderBottom: `1px solid ${theme.app.line}` }}
            >
              <Box
                component="code"
                sx={{ font: `13px ${monoFontFamily}`, bgcolor: theme.app.chip, border: `1px solid ${theme.app.chipLine}`, borderRadius: '7px', px: 1, py: 0.5 }}
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
                {hint?.url && (
                  <Link component="button" variant="caption" onClick={() => openURL(hint.url!)} sx={{ display: 'block', mt: 0.5 }}>
                    {t('tools.installLink', { name: tool.name })}
                  </Link>
                )}
                {hint?.command && <Command text={hint.command} />}
              </Box>
              <Box component="span" sx={{ fontSize: 12, fontWeight: 700, color, border: `1px solid ${color}`, borderRadius: '7px', px: 1, py: 0.5 }}>
                {label.text}
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
        {diag && <SystemInfo diag={diag} />}
      </DialogContent>
      <DialogActions>
        <Button startIcon={<ContentCopyIcon />} onClick={() => void copyReport()} disabled={!diag} sx={{ mr: 'auto' }}>
          {t('tools.copyReport')}
        </Button>
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
