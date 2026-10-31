// Временный экран для проверки связки Go ↔ фронтенд (этап 04).
// Заменяется полноценным интерфейсом на этапе 09.
import { useEffect, useState } from 'react'
import { AppService } from '../bindings/github.com/forktopus/forktopus/internal/app'
import type { Root, ToolDiagnostic } from '../bindings/github.com/forktopus/forktopus/internal/model'

// Сообщение из ошибки Wails: ошибки ядра приходят в cause как { code, message }.
function errorText(err: unknown): string {
  const cause = (err as { cause?: { code?: string; message?: string } })?.cause
  if (cause?.code) {
    return `${cause.code}: ${cause.message ?? ''}`
  }
  return err instanceof Error ? err.message : String(err)
}

function App() {
  const [tools, setTools] = useState<ToolDiagnostic[]>([])
  const [roots, setRoots] = useState<Root[]>([])
  const [path, setPath] = useState('')
  const [error, setError] = useState('')

  const refreshRoots = () =>
    AppService.ListRoots().then((r) => setRoots(r ?? [])).catch((e) => setError(errorText(e)))

  useEffect(() => {
    AppService.CheckTools().then((t) => setTools(t ?? [])).catch((e) => setError(errorText(e)))
    refreshRoots()
  }, [])

  const addRoot = () => {
    setError('')
    AppService.AddRoot(path.trim())
      .then(() => {
        setPath('')
        return refreshRoots()
      })
      .catch((e) => setError(errorText(e)))
  }

  const removeRoot = (id: string) => {
    setError('')
    AppService.RemoveRoot(id).then(refreshRoots).catch((e) => setError(errorText(e)))
  }

  return (
    <main style={{ padding: 32, fontFamily: 'sans-serif', color: '#e9edf5' }}>
      <h1>Forktopus</h1>

      <h2>Утилиты</h2>
      <ul>
        {tools.map((t) => (
          <li key={t.name}>
            {t.name}: {t.availability} {t.version} {t.error?.message}
          </li>
        ))}
      </ul>

      <h2>Корневые папки</h2>
      <ul>
        {roots.map((r) => (
          <li key={r.id}>
            {r.path} <button onClick={() => removeRoot(r.id)}>Убрать</button>
          </li>
        ))}
      </ul>
      <input
        value={path}
        onChange={(e) => setPath(e.target.value)}
        placeholder="Абсолютный путь к папке"
        style={{ width: 480 }}
      />{' '}
      <button onClick={addRoot}>Добавить</button>
      {error && <p style={{ color: '#fb788a' }}>{error}</p>}
    </main>
  )
}

export default App
