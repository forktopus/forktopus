import { AppService } from '../../bindings/github.com/forktopus/forktopus/internal/app'
import type { DataSource } from './source'

export const wailsSource: DataSource = {
  kind: 'wails',
  listRoots: async () => (await AppService.ListRoots()) ?? [],
  addRoot: (path) => AppService.AddRoot(path),
  removeRoot: (id) => AppService.RemoveRoot(id),
  listRepositories: async () => (await AppService.ListRepositories()) ?? [],
  checkTools: async () => (await AppService.CheckTools()) ?? [],
  getUI: () => AppService.GetUI(),
  saveUI: (ui) => AppService.SaveUI(ui),
}
