import { createTheme, type Theme } from '@mui/material/styles'
import { fontFamily, tokens, type AppTokens, type ThemeMode } from './tokens'

declare module '@mui/material/styles' {
  interface Theme {
    app: AppTokens
  }
  interface ThemeOptions {
    app?: AppTokens
  }
}

/** Семантический тон статуса: цвет берётся из токенов темы. */
export type Tone = 'good' | 'warn' | 'bad' | 'info' | 'muted'

export function toneColor(theme: Theme, tone: Tone): string {
  const t = theme.app
  return { good: t.good, warn: t.warn, bad: t.bad, info: t.sky, muted: t.muted }[tone]
}

export function createAppTheme(mode: ThemeMode): Theme {
  const t = tokens[mode]
  return createTheme({
    app: t,
    palette: {
      mode,
      primary: { main: t.primary, contrastText: '#ffffff' },
      success: { main: t.good },
      warning: { main: t.warn },
      error: { main: t.bad },
      info: { main: t.sky },
      background: { default: t.bg, paper: t.panel },
      text: { primary: t.text, secondary: t.muted },
      divider: t.line,
    },
    shape: { borderRadius: 10 },
    typography: {
      fontFamily,
      fontSize: 14,
      button: { textTransform: 'none', fontWeight: 700 },
      h1: { fontSize: 32, fontWeight: 760, letterSpacing: '-1px' },
      h2: { fontSize: 21, fontWeight: 700 },
      overline: { fontSize: 12, fontWeight: 800, letterSpacing: '1.4px' },
    },
    components: {
      MuiCssBaseline: {
        styleOverrides: { body: { backgroundColor: t.body } },
      },
      MuiPaper: {
        // Без тени: размытую тень высокой карточки (таблица реестра) WebKitGTK
        // перерисовывает на каждом кадре анимации диалога — ~0,35 с на кадр.
        // Карточки и так с рамкой. У диалогов и меню elevation свой — тень остаётся.
        defaultProps: { elevation: 0 },
        styleOverrides: { root: { backgroundImage: 'none', border: `1px solid ${t.line}` } },
      },
      MuiButton: {
        defaultProps: { disableElevation: true },
        styleOverrides: { root: { borderRadius: 10 } },
      },
      MuiDialog: {
        styleOverrides: { paper: { borderRadius: 17 } },
      },
    },
  })
}
