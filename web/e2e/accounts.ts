import { expect, type Page } from '@playwright/test'

/**
 * Contas usadas nos testes E2E.
 * - Localmente: as de `make dev-users` (DEV_USER_PASSWORD do .env).
 * - Em staging: criadas pelo workflow deploy-staging, com as palavras-passe
 *   nos segredos do GitHub.
 */
const devPassword = process.env.DEV_USER_PASSWORD ?? 'troca-me-dev-local'

export const accounts = {
  gestor: {
    email: process.env.E2E_GESTOR_EMAIL ?? 'gestor@loja.test',
    password: process.env.E2E_GESTOR_PASSWORD ?? devPassword,
  },
  funcionario: {
    email: process.env.E2E_FUNCIONARIO_EMAIL ?? 'funcionario@loja.test',
    password: process.env.E2E_FUNCIONARIO_PASSWORD ?? devPassword,
  },
} as const

export type Account = keyof typeof accounts

/** Inicia sessão pelo formulário e espera chegar à página pedida. */
export async function login(page: Page, who: Account, next = '/app/painel') {
  await page.goto(`/entrar?next=${encodeURIComponent(next)}`)
  await page.getByLabel('E-mail').fill(accounts[who].email)
  await page.getByLabel('Palavra-passe').fill(accounts[who].password)
  await page.getByRole('button', { name: 'Entrar' }).click()
  await expect(page).toHaveURL((url) => url.pathname === next)
}
