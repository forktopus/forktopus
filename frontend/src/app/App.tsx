import AddIcon from '@mui/icons-material/Add'
import { Alert, Box, Button, CircularProgress, CssBaseline, Snackbar, ThemeProvider, Typography, useMediaQuery } from '@mui/material'
import { useMemo, useState } from 'react'
import { t } from '../i18n'
import { integrationIssueCount } from '../logic/registry'
import { useApp } from '../store/context'
import { useRepos } from '../store/selectors'
import { createAppTheme } from '../theme/theme'
import { ExplorerView } from '../views/ExplorerView'
import { RadarView } from '../views/RadarView'
import { RegistryView } from '../views/RegistryView'
import { RootsDialog } from './RootsDialog'
import { ToolsDialog } from './ToolsDialog'
import { TopBar } from './TopBar'

function EmptyRoots({ onAdd }: { onAdd(): void }) {
  return (
    <Box sx={{ maxWidth: 560, mx: 'auto', mt: 10, textAlign: 'center' }}>
      <Box component="img" src="/forktopus-icon.png" alt={t('app.mascotAlt')} sx={{ width: 96, height: 96, borderRadius: '24px', mb: 3 }} />
      <Typography variant="h1" sx={{ mb: 1.5 }}>
        {t('empty.noRoots.title')}
      </Typography>
      <Typography sx={{ color: 'text.secondary', mb: 3 }}>{t('empty.noRoots.text')}</Typography>
      <Button variant="contained" size="large" startIcon={<AddIcon />} onClick={onAdd}>
        {t('action.addRoot')}
      </Button>
    </Box>
  )
}

function Content({ onOpenRoots, onOpenTools }: { onOpenRoots(): void; onOpenTools(): void }) {
  const ready = useApp((s) => s.ready)
  const roots = useApp((s) => s.roots)
  const mode = useApp((s) => s.mode)
  const repos = useRepos()
  const issues = useMemo(() => integrationIssueCount(repos), [repos])

  if (!ready) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 2, mt: 12, color: 'text.secondary' }}>
        <CircularProgress size={22} />
        {t('app.loading')}
      </Box>
    )
  }
  if (roots.length === 0) return <EmptyRoots onAdd={onOpenRoots} />

  return (
    <>
      {issues > 0 && (
        <Alert
          severity="info"
          sx={{ mb: 2.5 }}
          action={
            <Button color="inherit" size="small" onClick={onOpenTools}>
              {t('action.tools')}
            </Button>
          }
        >
          {t('integration.banner', { count: issues })}
        </Alert>
      )}
      {repos.length === 0 && <Alert severity="info">{t('empty.noRepos')}</Alert>}
      {repos.length > 0 && mode === 'registry' && <RegistryView />}
      {repos.length > 0 && mode === 'explorer' && <ExplorerView />}
      {repos.length > 0 && mode === 'radar' && <RadarView />}
      {repos.length > 0 && (
        <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', mt: 3, textAlign: 'center' }}>
          {t('screen.draft')}
        </Typography>
      )}
    </>
  )
}

export function App() {
  const choice = useApp((s) => s.theme)
  const systemDark = useMediaQuery('(prefers-color-scheme: dark)')
  const mode = choice ?? (systemDark ? 'dark' : 'light')
  const theme = useMemo(() => createAppTheme(mode), [mode])
  const error = useApp((s) => s.error)
  const clearError = useApp((s) => s.clearError)
  const [dialog, setDialog] = useState<'tools' | 'roots' | null>(null)

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <TopBar onOpenTools={() => setDialog('tools')} onOpenRoots={() => setDialog('roots')} />
      <Box component="main" sx={{ maxWidth: 1550, mx: 'auto', px: 4, py: 4 }}>
        <Content onOpenRoots={() => setDialog('roots')} onOpenTools={() => setDialog('tools')} />
      </Box>
      <ToolsDialog open={dialog === 'tools'} onClose={() => setDialog(null)} />
      <RootsDialog open={dialog === 'roots'} onClose={() => setDialog(null)} />
      <Snackbar open={error !== null} autoHideDuration={8000} onClose={clearError} anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}>
        <Alert severity="error" onClose={clearError} variant="filled">
          {error}
        </Alert>
      </Snackbar>
    </ThemeProvider>
  )
}
