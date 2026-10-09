import { writeFileSync } from 'node:fs'
import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'

// Frontend se sestavuje přímo do backendu, odkud se přes go:embed
// vkompiluje do binárky (server/internal/sceniter/web).
const outDir = path.resolve(import.meta.dirname, '../server/internal/sceniter/web/dist')

const backendUrl = process.env.BACKEND_URL ?? 'http://localhost:8080'

// emptyOutDir smaže i .gitkeep, který drží adresář v gitu; po buildu ho vrátíme.
function keepGitkeep(): Plugin {
  return {
    name: 'sceniter-keep-gitkeep',
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
    host: true,
    // Libriter běží na 5173; oba weby jdou proti stejnému backendu.
    port: 5174,
    allowedHosts: true,
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
