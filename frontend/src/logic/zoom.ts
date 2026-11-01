// Масштаб интерфейса (D-061): нативный зум WebView, 100–200 % с шагом 10 %.
// Ниже 100 % Wails на Linux не опускается — поэтому и на других ОС не даём.

export const minZoom = 1
export const maxZoom = 2
export const zoomStep = 0.1

/** Масштаб в допустимых пределах, кратный шагу; мусор — 100 %. */
export function clampZoom(zoom: number | undefined): number {
  if (zoom == null || !Number.isFinite(zoom)) return minZoom
  const stepped = Math.round(zoom / zoomStep) * zoomStep
  return Math.min(maxZoom, Math.max(minZoom, Number(stepped.toFixed(1))))
}

/** Действие по сочетанию клавиш: Ctrl/⌘ + «=», «+», «−», «0» (и цифровой блок). */
export function zoomKey(e: Pick<KeyboardEvent, 'ctrlKey' | 'metaKey' | 'altKey' | 'code' | 'key'>): 'in' | 'out' | 'reset' | null {
  if (!(e.ctrlKey || e.metaKey) || e.altKey) return null
  // code — физическая клавиша: на русской раскладке key у этих клавиш тот же, но не везде.
  switch (e.code) {
    case 'Equal':
    case 'NumpadAdd':
      return 'in'
    case 'Minus':
    case 'NumpadSubtract':
      return 'out'
    case 'Digit0':
    case 'Numpad0':
      return 'reset'
  }
  if (e.key === '+' || e.key === '=') return 'in'
  if (e.key === '-') return 'out'
  return null
}
