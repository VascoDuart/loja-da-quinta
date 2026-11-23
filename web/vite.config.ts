import react from '@vitejs/plugin-react'
import { configDefaults } from 'vitest/config'
import { defineConfig, type Plugin } from 'vite'

// Commit do build. A Vercel define VERCEL_GIT_COMMIT_SHA; localmente é "dev".
const version = (process.env.VERCEL_GIT_COMMIT_SHA ?? process.env.APP_VERSION ?? 'dev').slice(0, 40)

/**
 * Escreve a versão numa <meta> do index.html. O workflow de deploy lê-a para
 * saber quando a versão nova já está no ar, antes de correr os testes E2E.
 * É uma meta (não um script inline), por isso a CSP continua sem 'unsafe-inline'.
 */
function appVersion(): Plugin {
  return {
    name: 'app-version',
    transformIndexHtml: () => [{ tag: 'meta', attrs: { name: 'app-version', content: version }, injectTo: 'head' }],
  }
}

export default defineConfig({
  plugins: [react(), appVersion()],
  build: {
    // As fontes ficam sempre em ficheiros próprios: embebidas como data: no
    // CSS seriam bloqueadas pela CSP (font-src 'self'), que não se relaxa.
    assetsInlineLimit: (filePath) => (/\.(woff2?|ttf|otf)$/.test(filePath) ? false : undefined),
  },
  server: {
    port: 5173,
    // Em desenvolvimento, /api vai para a API local, como o rewrite da
    // Vercel faz em produção (ADR-0007): o frontend só vê uma origem.
    // API_PROXY_TARGET permite apontar para a API de staging (ex.: testes E2E
    // locais sem Docker).
    proxy: {
      '/api': { target: process.env.API_PROXY_TARGET ?? 'http://localhost:8080', changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    exclude: [...configDefaults.exclude, 'e2e/**'],
  },
})
