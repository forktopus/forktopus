import React from 'react'
import ReactDOM from 'react-dom/client'
import { App } from './app/App'
import { pickSource } from './data'
import { createAppStore } from './store/appStore'
import { AppStoreProvider } from './store/context'

const store = createAppStore(await pickSource())
void store.getState().init()

// Окно автозапуска (--hidden) до первого показа имеет нулевую ширину, а
// вёрстку при ней WebKitGTK потом не пересчитывает: строки grid остаются
// высокими. Store уже работает (сводка трея, события фона), а отрисовка ждёт
// окна ненулевого размера.
while (window.innerWidth === 0) {
  await new Promise((resolve) => window.addEventListener('resize', resolve, { once: true }))
}

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <AppStoreProvider store={store}>
      <App />
    </AppStoreProvider>
  </React.StrictMode>,
)
