import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'

import { login } from './accounts'

// Testes de fumo: provam que o frontend, o rewrite /api, a API e a base de
// dados funcionam juntos, e que as páginas principais são acessíveis.

test('o catálogo público abre com a marca e o título', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveTitle('Produtos da época · Loja da Quinta')
  await expect(page.getByRole('link', { name: 'Loja da Quinta' })).toBeVisible()
})

test('o painel chega à base de dados através do rewrite /api', async ({ page }) => {
  await login(page, 'gestor', '/app/painel')
  await expect(page.getByRole('heading', { name: 'Painel', level: 1 })).toBeVisible()
  await expect(page.getByText('Servidor pronto. A base de dados responde.')).toBeVisible()
})

test('o tema escuro mantém-se ao mudar de página', async ({ page }) => {
  await login(page, 'funcionario', '/app/venda')
  await expect(page.getByRole('heading', { name: 'Venda', level: 1 })).toBeVisible()
  const html = page.locator('html')
  // O botão pode estar no modo oposto se o sistema já estiver em escuro.
  const toggle = page.getByRole('button', { name: /Mudar para o modo (escuro|claro)/ })
  if ((await toggle.getAttribute('aria-label')) === 'Mudar para o modo escuro') await toggle.click()
  await expect(html).toHaveAttribute('data-theme', 'dark')

  await page.getByRole('link', { name: 'Stock' }).click()
  await expect(page.getByRole('heading', { name: 'Stock', level: 1 })).toBeVisible()
  await expect(html).toHaveAttribute('data-theme', 'dark')
})

test('um endereço desconhecido mostra a página 404', async ({ page }) => {
  await page.goto('/isto-nao-existe')
  await expect(page.getByRole('heading', { name: 'Página não encontrada' })).toBeVisible()
})

async function expectNoAxeViolations(page: Page) {
  await page.waitForLoadState('networkidle')
  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
  expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([])
}

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`modo ${colorScheme === 'light' ? 'claro' : 'escuro'}`, () => {
    test.use({ colorScheme })

    for (const path of ['/', '/entrar']) {
      test(`${path} não tem problemas de acessibilidade WCAG 2.1 AA`, async ({ page }) => {
        await page.goto(path)
        await expectNoAxeViolations(page)
      })
    }

    for (const path of ['/app/painel', '/app/utilizadores']) {
      test(`${path} não tem problemas de acessibilidade WCAG 2.1 AA`, async ({ page }) => {
        await login(page, 'gestor', path)
        await expectNoAxeViolations(page)
      })
    }

    test('o formulário de utilizador não tem problemas de acessibilidade WCAG 2.1 AA', async ({ page }) => {
      await login(page, 'gestor', '/app/utilizadores')
      await page.getByRole('button', { name: 'Novo utilizador' }).click()
      await expect(page.getByLabel('E-mail')).toBeVisible()
      await expectNoAxeViolations(page)
    })
  })
}
