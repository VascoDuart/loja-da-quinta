import { randomBytes } from 'node:crypto'

import { expect, type Page, test } from '@playwright/test'

import { login } from './accounts'

/**
 * Gestão de utilizadores (UC-02) de ponta a ponta. O teste corre também
 * contra staging, por isso usa sempre a mesma conta temporária por
 * dispositivo: cria-a na primeira vez e, nas seguintes, só a edita. Assim a
 * base de dados não acumula uma conta nova por cada deploy.
 */
test('desativar uma conta termina a sessão aberta dela', async ({ browser, page }, testInfo) => {
  const email = `e2e-temporario-${testInfo.project.name}@loja.test`
  // Palavra-passe nova em cada execução: nunca fica escrita em lado nenhum.
  const password = randomBytes(18).toString('base64url')

  // 1. O gestor garante que a conta existe, ativa e com esta palavra-passe.
  await login(page, 'gestor', '/app/utilizadores')
  const list = page.getByRole('list', { name: 'Utilizadores' })
  await expect(list).toBeVisible()
  const row = list.getByRole('listitem').filter({ hasText: email })

  if ((await row.count()) === 0) {
    await page.getByRole('button', { name: 'Novo utilizador' }).click()
    await page.getByLabel('E-mail').fill(email)
    await page.getByLabel('Nome').fill(`Temporário ${testInfo.project.name}`)
    await page.getByRole('radio', { name: 'Funcionário' }).check()
    await page.getByLabel('Palavra-passe').fill(password)
    await page.getByRole('button', { name: 'Criar utilizador' }).click()
  } else {
    await row.getByRole('button', { name: /^Editar / }).click()
    await page.getByRole('checkbox', { name: 'Conta ativa' }).check()
    await page.getByLabel('Palavra-passe nova').fill(password)
    await page.getByRole('button', { name: 'Guardar' }).click()
  }
  await expect(page.getByRole('status')).toBeVisible()
  await expect(row).toContainText('Ativo')

  // 2. A conta temporária inicia sessão noutro browser.
  const other = await browser.newContext()
  const otherPage = await other.newPage()
  await loginAs(otherPage, email, password)
  await expect(otherPage.getByRole('heading', { name: 'Painel', level: 1 })).toBeVisible()

  // 3. O gestor desativa-a.
  await row.getByRole('button', { name: /^Editar / }).click()
  await page.getByRole('checkbox', { name: 'Conta ativa' }).uncheck()
  await page.getByRole('button', { name: 'Guardar' }).click()
  await expect(row).toContainText('Desativado')

  // 4. A sessão aberta terminou: ao recarregar, volta ao login, e já não entra.
  await otherPage.reload()
  await expect(otherPage).toHaveURL(/\/entrar/)
  await otherPage.getByLabel('E-mail').fill(email)
  await otherPage.getByLabel('Palavra-passe').fill(password)
  await otherPage.getByRole('button', { name: 'Entrar' }).click()
  await expect(otherPage.getByRole('alert')).toHaveText('E-mail ou palavra-passe incorretos.')
  await other.close()
})

async function loginAs(page: Page, email: string, password: string) {
  await page.goto('/entrar')
  await page.getByLabel('E-mail').fill(email)
  await page.getByLabel('Palavra-passe').fill(password)
  await page.getByRole('button', { name: 'Entrar' }).click()
  await expect(page).toHaveURL(/\/app\/painel$/)
}
