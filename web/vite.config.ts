/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react()],
  build: {
    // As fontes ficam sempre em ficheiros próprios: embebidas como data: no
    // CSS seriam bloqueadas pela CSP (font-src 'self'), que não se relaxa.
    assetsInlineLimit: (filePath) => (/\.(woff2?|ttf|otf)$/.test(filePath) ? false : undefined),
  },
  server: {
    port: 5173,
    // Em desenvolvimento, /api vai para a API local, como o rewrite da
    // Vercel faz em produção (ADR-0007): o frontend só vê uma origem.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
