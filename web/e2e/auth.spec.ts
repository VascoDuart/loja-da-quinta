import { expect, test } from '@playwright/test'

import { login } from './accounts'

// Fluxos da sessão num browser real, contra a API e a base de dados reais.

test('sem sessão, o backoffice manda para o login e volta à página pedida', async ({ page }) => {
  await page.goto('/app/stock')
  await expect(page).toHaveURL(/\/entrar\?next=%2Fapp%2Fstock$/)
  await login(page, 'gestor', '/app/stock')
  await expect(page.getByRole('heading', { name: 'Stock', level: 1 })).toBeVisible()
})

test('credenciais erradas mostram a mensagem genérica', async ({ page }, testInfo) => {
  // Uma conta que não existe, diferente por projeto: as falhas contam para o
  // bloqueio da conta, e as contas reais dos testes nunca devem ficar bloqueadas.
  await page.goto('/entrar')
  await page.getByLabel('E-mail').fill(`nao-existe-${testInfo.project.name}@loja.test`)
  await page.getByLabel('Palavra-passe').fill('palavra-passe-errada')
  await page.getByRole('button', { name: 'Entrar' }).click()
  await expect(page.getByRole('alert')).toHaveText('E-mail ou palavra-passe incorretos.')
  await expect(page).toHaveURL(/\/entrar$/)
})

test('a sessão sobrevive a recarregar a página (refresh pelo cookie)', async ({ page }) => {
  await login(page, 'gestor', '/app/venda')
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Venda', level: 1 })).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'Backoffice' }).getByText('Gestor', { exact: true })).toBeVisible()
})

test('o access token nunca fica no localStorage nem num cookie legível', async ({ page }) => {
  await login(page, 'gestor')
  const stored = await page.evaluate(() => JSON.stringify(window.localStorage) + document.cookie)
  expect(stored).not.toMatch(/eyJ/) // início de qualquer JWT
  expect(stored).not.toContain('refresh_token')
})

test('terminar sessão: recarregar já não volta a entrar', async ({ page }) => {
  await login(page, 'gestor')
  await page.getByRole('button', { name: 'Terminar sessão' }).click()
  await expect(page.getByRole('heading', { name: 'Entrar no backoffice' })).toBeVisible()
  await page.goto('/app/painel')
  await expect(page).toHaveURL(/\/entrar\?next=/)
})

test('o funcionário não vê as páginas do gestor', async ({ page }) => {
  await login(page, 'funcionario', '/app/venda')
  const nav = page.getByRole('navigation', { name: 'Backoffice' })
  await expect(nav.getByRole('link', { name: 'Venda' })).toBeVisible()
  await expect(nav.getByRole('link', { name: 'Relatórios' })).toHaveCount(0)

  await page.goto('/app/relatorios')
  await expect(page.getByRole('heading', { name: 'Sem permissão' })).toBeVisible()
})
