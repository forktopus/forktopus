import { useEffect } from 'react'
import { zoomKey, zoomStep, minZoom } from '../logic/zoom'
import { useApp } from '../store/context'

// Ctrl+колесо: тачпад шлёт много мелких событий — шаг масштаба на каждые 100
// единиц прокрутки, как у одного щелчка колеса мыши.
const wheelPerStep = 100

/** Горячие клавиши масштаба (D-061): Ctrl/⌘ + = / − / 0 и Ctrl+колесо. */
export function useZoomKeys() {
  const zoom = useApp((s) => s.zoom)
  const setZoom = useApp((s) => s.setZoom)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const action = zoomKey(e)
      if (!action) return
      e.preventDefault()
      setZoom(action === 'reset' ? minZoom : zoom + (action === 'in' ? zoomStep : -zoomStep))
    }
    let wheel = 0
    const onWheel = (e: WheelEvent) => {
      if (!e.ctrlKey) return
      e.preventDefault()
      wheel += e.deltaY
      if (Math.abs(wheel) < wheelPerStep) return
      setZoom(zoom + (wheel < 0 ? zoomStep : -zoomStep))
      wheel = 0
    }
    window.addEventListener('keydown', onKey)
    window.addEventListener('wheel', onWheel, { passive: false })
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('wheel', onWheel)
    }
  }, [zoom, setZoom])
}
