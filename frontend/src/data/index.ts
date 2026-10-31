import type { DataSource } from './source'

// Признаки нативного окна Wails v3 (Windows WebView2, WebKit, прочие).
// window._wails не годится: его создаёт сам runtime и в обычном браузере.
function hasNativeHost(): boolean {
  const w = window as unknown as {
    chrome?: { webview?: unknown }
    webkit?: { messageHandlers?: { external?: unknown } }
    wails?: { invoke?: unknown }
  }
  return Boolean(w.chrome?.webview || w.webkit?.messageHandlers?.external || w.wails?.invoke)
}

/**
 * Демо-данные включаются явно (VITE_FORKTOPUS_DEMO=1) или когда страница
 * открыта не в окне Wails, например в Vite-превью из Docker.
 */
export async function pickSource(): Promise<DataSource> {
  if (import.meta.env.VITE_FORKTOPUS_DEMO === '1' || !hasNativeHost()) {
    const { createMockSource } = await import('./mockSource')
    return createMockSource()
  }
  const { wailsSource } = await import('./wailsSource')
  return wailsSource
}
