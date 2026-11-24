import { defineConfig, devices } from '@playwright/test'

/**
 * Testes E2E (ponta a ponta) num browser real.
 *
 * - Com E2E_BASE_URL, correm contra um ambiente publicado (staging no CI).
 * - Sem E2E_BASE_URL, arrancam o servidor de desenvolvimento local. A API é a
 *   do docker compose, ou outra com API_PROXY_TARGET (ex.: a de staging).
 */
const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:5173'

/**
 * Contra um ambiente publicado, os testes usam palavras-passe reais (de
 * contas de teste). Os traces e o relatório HTML guardam o que se escreve
 * nos campos, os pedidos e os tokens, e o repositório é público: os
 * artefactos do CI podem ser descarregados por qualquer pessoa. Por isso,
 * aí só ficam capturas de ecrã (os campos de palavra-passe aparecem como
 * pontos). Localmente, com as contas do make dev-users, fica tudo.
 */
const published = !!process.env.E2E_BASE_URL

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: published ? 'list' : process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  // O primeiro pedido à API de staging pode ter de a acordar (ADR-0007).
  timeout: 90_000,
  expect: { timeout: 75_000 },
  use: {
    baseURL,
    locale: 'pt-PT',
    timezoneId: 'Europe/Lisbon',
    trace: published ? 'off' : 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'tablet', use: { ...devices['Desktop Chrome'], viewport: { width: 1024, height: 768 } } },
    { name: 'telemovel', use: { ...devices['Pixel 7'] } },
  ],
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : { command: 'npm run dev', url: baseURL, reuseExistingServer: true, timeout: 60_000 },
})
