import { defineConfig, devices } from '@playwright/test'

/**
 * Testes E2E (ponta a ponta) num browser real.
 *
 * - Com E2E_BASE_URL, correm contra um ambiente publicado (staging no CI).
 * - Sem E2E_BASE_URL, arrancam o servidor de desenvolvimento local. A API é a
 *   do docker compose, ou outra com API_PROXY_TARGET (ex.: a de staging).
 */
const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:5173'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  // O primeiro pedido à API de staging pode ter de a acordar (ADR-0007).
  timeout: 90_000,
  expect: { timeout: 75_000 },
  use: {
    baseURL,
    locale: 'pt-PT',
    timezoneId: 'Europe/Lisbon',
    trace: 'retain-on-failure',
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
