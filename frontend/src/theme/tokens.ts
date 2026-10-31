// Цветовые токены. Тёмная тема снята с макета
// idea/mockups/interactive-prototypes.html (:root и классы), светлая — производная
// от неё с опорой на светлый «Проводник» того же макета.

export type ThemeMode = 'light' | 'dark'

export interface ProviderColors {
  bg: string
  fg: string
}

export interface AppTokens {
  body: string
  bg: string
  panel: string
  panelAlt: string
  line: string
  text: string
  muted: string
  accent: string
  primary: string
  good: string
  warn: string
  bad: string
  sky: string
  nav: string
  navLine: string
  tabs: string
  tabActive: string
  tabActiveText: string
  chip: string
  chipLine: string
  provider: Record<'github' | 'gitlab' | 'gitea' | 'other', ProviderColors>
}

export const fontFamily = 'system-ui, -apple-system, "Segoe UI", Arial, sans-serif'
export const monoFontFamily = 'ui-monospace, SFMono-Regular, Consolas, monospace'

export const tokens: Record<ThemeMode, AppTokens> = {
  dark: {
    body: '#080d19',
    bg: '#0b1020',
    panel: '#151e31',
    panelAlt: '#19263d',
    line: '#263249',
    text: '#e9edf5',
    muted: '#98a5be',
    accent: '#7f93ff',
    primary: '#758dff',
    good: '#39c897',
    warn: '#f2ba69',
    bad: '#fb788a',
    sky: '#73c4ef',
    nav: 'rgba(9, 14, 27, 0.94)',
    navLine: '#242c42',
    tabs: '#111b2b',
    tabActive: '#304268',
    tabActiveText: '#ffffff',
    chip: '#1c2a41',
    chipLine: '#32445c',
    provider: {
      github: { bg: '#293e64', fg: '#c8d8ff' },
      gitlab: { bg: '#543b3e', fg: '#ffb878' },
      gitea: { bg: '#284c47', fg: '#85e0c0' },
      other: { bg: '#42435b', fg: '#ced6e8' },
    },
  },
  light: {
    body: '#eef1f4',
    bg: '#f4f5f6',
    panel: '#ffffff',
    panelAlt: '#f9fafb',
    line: '#dce2e7',
    text: '#1e3043',
    muted: '#78899a',
    accent: '#306aaf',
    primary: '#306aaf',
    good: '#1f9a72',
    warn: '#b7791f',
    bad: '#c94a5c',
    sky: '#2b7fb8',
    nav: 'rgba(255, 255, 255, 0.94)',
    navLine: '#dce2e7',
    tabs: '#eef1f4',
    tabActive: '#e1edfb',
    tabActiveText: '#315d95',
    chip: '#f1f5f9',
    chipLine: '#d9e0e6',
    provider: {
      github: { bg: '#dfe8fb', fg: '#2c4a86' },
      gitlab: { bg: '#fbe6d6', fg: '#a65318' },
      gitea: { bg: '#d9f0e8', fg: '#1f7a5c' },
      other: { bg: '#e6e7ee', fg: '#4a4f66' },
    },
  },
}
