import type { AppUpdate } from '../data/model'

/** Одинаковые правила кнопок для состояний Go и демо; сам пакет выбирает Go. */
export function updateActions(state: AppUpdate) {
  const busy = ['checking', 'downloading', 'verifying'].includes(state.status)
  return {
    busy,
    check: !busy && Boolean(state.format),
    download: !busy && state.canDownload,
    cancel: ['downloading', 'verifying'].includes(state.status),
    folder: state.status === 'ready',
    progress: state.size > 0 ? Math.min(100, Math.round((state.downloaded / state.size) * 100)) : 0,
  }
}
