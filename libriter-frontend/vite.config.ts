import { writeFileSync } from 'node:fs'
import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'

// Frontend se sestavuje přímo do backendu, odkud se přes go:embed
// vkompiluje do binárky – žádný kopírovací krok není potřeba.
const outDir = path.resolve(import.meta.dirname, '../libriter-backend/internal/web/dist')

const backendUrl = process.env.BACKEND_URL ?? 'http://localhost:8080'

// emptyOutDir smaže i .gitkeep, který drží adresář v gitu a zajišťuje,
// že `go build` projde i bez sestaveného frontendu. Po buildu ho vrátíme.
function keepGitkeep(): Plugin {
  return {
    name: 'libriter-keep-gitkeep',
    apply: 'build',
    closeBundle() {
      writeFileSync(path.join(outDir, '.gitkeep'), '')
    },
  }
}

export default defineConfig({
  plugins: [react(), tailwindcss(), keepGitkeep()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, 'src'),
    },
  },
  server: {
    // Naslouchat na všech rozhraních (0.0.0.0), aby šel dev server otevřít
    // i z jiného počítače, ne jen z localhostu.
    host: true,
    port: 5173,
    // Povolit i přístup přes hostname, ne jen IP adresu.
    allowedHosts: true,
    // Backend běží samostatně (výchozí :8080); proxy nás zbavuje potřeby CORS
    // a frontend tak volá stejné relativní cesty jako v produkci.
    // Jiný port backendu: BACKEND_URL=http://localhost:9000 npm run dev
    proxy: {
      '/api': backendUrl,
      '/health': backendUrl,
    },
  },
  build: {
    outDir,
    emptyOutDir: true,
  },
})
