import AddIcon from '@mui/icons-material/Add'
import DarkModeIcon from '@mui/icons-material/DarkMode'
import LightModeIcon from '@mui/icons-material/LightMode'
import RefreshIcon from '@mui/icons-material/Refresh'
import SearchIcon from '@mui/icons-material/Search'
import SettingsIcon from '@mui/icons-material/Settings'
import TerminalIcon from '@mui/icons-material/Terminal'
import { Box, Button, ButtonBase, CircularProgress, IconButton, InputAdornment, TextField, Tooltip, Typography, useMediaQuery, useTheme } from '@mui/material'
import { useEffect, useRef } from 'react'
import { formatRelative, t, type MessageKey } from '../i18n'
import { modes, type Mode } from '../store/appStore'
import { useApp } from '../store/context'
import { useNow } from './useNow'

const modeLabel: Record<Mode, MessageKey> = { registry: 'mode.registry', explorer: 'mode.explorer', radar: 'mode.radar' }

interface Props {
  onOpenTools(): void
  onOpenRoots(): void
  onOpenSettings(): void
}

export function TopBar({ onOpenTools, onOpenRoots, onOpenSettings }: Props) {
  const theme = useTheme()
  const app = theme.app
  const mode = useApp((s) => s.mode)
  const setMode = useApp((s) => s.setMode)
  const query = useApp((s) => s.query)
  const setQuery = useApp((s) => s.setQuery)
  const scanning = useApp((s) => s.scanning)
  const scanFound = useApp((s) => s.scanFound)
  const refresh = useApp((s) => s.refresh)
  const lastCheckAt = useApp((s) => s.lastCheckAt)
  const demo = useApp((s) => s.sourceKind === 'mock')
  const setTheme = useApp((s) => s.setTheme)
  const now = useNow()
  const searchRef = useRef<HTMLInputElement>(null)
  // Окно не уже 1000 px (MinWidth в main.go); до 1200 px панель компактная.
  const compact = useMediaQuery(theme.breakpoints.down('lg'))
  // Совсем узко (маленькое окно или крупный масштаб) — без названия и номеров вкладок, иначе шапка не помещается.
  const narrow = useMediaQuery(theme.breakpoints.down('md'))

  // «/» вне полей ввода — фокус в поиск, как в макете.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (document.activeElement?.tagName ?? '').toUpperCase()
      if (e.key === '/' && !['INPUT', 'TEXTAREA', 'SELECT'].includes(tag)) {
        e.preventDefault()
        searchRef.current?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const lastCheck = scanning
    ? t('lastCheck.scanning', { count: scanFound })
    : lastCheckAt
      ? t('lastCheck.at', { when: formatRelative(lastCheckAt, now) })
      : t('lastCheck.never')
  const dark = theme.palette.mode === 'dark'

  return (
    <Box
      component="header"
      sx={{
        position: 'sticky',
        top: 0,
        zIndex: theme.zIndex.appBar,
        height: 73,
        px: compact ? 2 : 3,
        display: 'flex',
        alignItems: 'center',
        gap: compact ? 1.5 : 2,
        bgcolor: app.nav,
        backdropFilter: 'blur(22px)',
        borderBottom: `1px solid ${app.navLine}`,
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, whiteSpace: 'nowrap' }}>
        <Box component="img" src="/forktopus-icon.png" alt={t('app.mascotAlt')} sx={{ width: 34, height: 34, filter: 'drop-shadow(0 4px 10px #685ee860)' }} />
        <Box sx={{ display: narrow ? 'none' : undefined }}>
          <Typography component="strong" sx={{ display: 'block', fontSize: 16, fontWeight: 760, letterSpacing: '-0.5px' }}>
            {t('app.name')}
          </Typography>
          {demo && (
            <Tooltip title={t('app.demoHint')}>
              <Box
                component="span"
                sx={{
                  display: 'inline-block',
                  fontSize: 12,
                  fontWeight: 800,
                  letterSpacing: '1px',
                  textTransform: 'uppercase',
                  color: app.warn,
                  border: `1px dashed ${app.warn}`,
                  borderRadius: '6px',
                  px: 0.75,
                }}
              >
                {t('app.demoBadge')}
              </Box>
            </Tooltip>
          )}
        </Box>
      </Box>

      <Box
        role="tablist"
        aria-label={t('mode.tabsLabel')}
        sx={{ display: 'flex', gap: 0.75, p: '5px', bgcolor: app.tabs, border: `1px solid ${app.line}`, borderRadius: '13px' }}
      >
        {modes.map((m, i) => {
          const active = m === mode
          return (
            <ButtonBase
              key={m}
              role="tab"
              aria-selected={active}
              onClick={() => setMode(m)}
              sx={{
                borderRadius: '9px',
                px: compact ? 1.25 : 1.75,
                py: 1.1,
                fontSize: 13,
                fontWeight: 720,
                whiteSpace: 'nowrap',
                color: active ? app.tabActiveText : app.muted,
                bgcolor: active ? app.tabActive : 'transparent',
                '&:hover': { color: app.tabActiveText },
                '&.Mui-focusVisible': { outline: `2px solid ${app.accent}` },
              }}
            >
              <Box component="span" sx={{ display: narrow ? 'none' : undefined, fontSize: 12, opacity: 0.6, mr: 1 }}>
                {String(i + 1).padStart(2, '0')}
              </Box>
              {t(modeLabel[m])}
            </ButtonBase>
          )
        })}
      </Box>

      <TextField
        inputRef={searchRef}
        size="small"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder={t('search.placeholder')}
        sx={{ flex: 1, minWidth: 120, maxWidth: 560 }}
        slotProps={{
          htmlInput: { 'aria-label': t('search.label') },
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" />
              </InputAdornment>
            ),
          },
        }}
      />

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, ml: 'auto' }}>
        {!compact && (
          <Typography variant="caption" sx={{ color: 'text.secondary', whiteSpace: 'nowrap', mr: 0.5 }}>
            {lastCheck}
          </Typography>
        )}
        <Tooltip title={`${t('action.refresh')} · ${lastCheck}`}>
          <span>
            <IconButton aria-label={t('action.refresh')} onClick={() => void refresh()} disabled={scanning}>
              {scanning ? <CircularProgress size={20} /> : <RefreshIcon />}
            </IconButton>
          </span>
        </Tooltip>
        <Tooltip title={t('action.tools')}>
          <IconButton aria-label={t('action.tools')} onClick={onOpenTools}>
            <TerminalIcon />
          </IconButton>
        </Tooltip>
        <Tooltip title={t('action.settings')}>
          <IconButton aria-label={t('action.settings')} onClick={onOpenSettings}>
            <SettingsIcon />
          </IconButton>
        </Tooltip>
        <Tooltip title={dark ? t('action.themeToLight') : t('action.themeToDark')}>
          <IconButton aria-label={dark ? t('action.themeToLight') : t('action.themeToDark')} onClick={() => setTheme(dark ? 'light' : 'dark')}>
            {dark ? <LightModeIcon /> : <DarkModeIcon />}
          </IconButton>
        </Tooltip>
        {compact ? (
          <Tooltip title={t('action.addRoot')}>
            <Button variant="contained" aria-label={t('action.addRoot')} onClick={onOpenRoots} sx={{ ml: 0.5, minWidth: 0, px: 1.25 }}>
              <AddIcon />
            </Button>
          </Tooltip>
        ) : (
          <Button variant="contained" startIcon={<AddIcon />} onClick={onOpenRoots} sx={{ ml: 1, whiteSpace: 'nowrap' }}>
            {t('action.addRoot')}
          </Button>
        )}
      </Box>
    </Box>
  )
}
