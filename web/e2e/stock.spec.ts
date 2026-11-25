import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'

import { addDays, lisbonToday } from '../src/shared/lib/format'
import { login } from './accounts'

/**
 * Ecrã de entrada de stock (UC-04). Estes testes nunca gravam lotes: os lotes
 * não se apagam e os testes correm em cada deploy de staging. O registo de
 * ponta a ponta está em ensureInStock (stock.ts), que o catálogo usa só
 * quando o produto de teste fica sem stock.
 */

/** Abre o ecrã como funcionário e espera pelo formulário (ou pela lista vazia). */
async function openStock(page: Page) {
  await login(page, 'funcionario', '/app/stock')
  await expect(page.getByRole('heading', { name: 'Stock', level: 1 })).toBeVisible()
  const form = page.getByRole('button', { name: 'Registar entrada' })
  await expect(form.or(page.getByText('Ainda não há produtos ativos'))).toBeVisible()
  return (await form.count()) > 0
}

test('o funcionário vê o formulário e as regras são verificadas antes de enviar', async ({ page }) => {
  const posts: string[] = []
  page.on('request', (r) => {
    if (r.method() === 'POST' && r.url().includes('/api/v1/stock/lots')) posts.push(r.url())
  })
  const hasProducts = await openStock(page)
  test.skip(!hasProducts, 'Ainda não há produtos ativos nesta base de dados (primeira execução).')

  const today = lisbonToday()
  await expect(page.getByLabel('Data de colheita')).toHaveValue(today)

  await page.getByLabel(/^Quantidade/).fill('0')
  await page.getByLabel('Validade').fill(addDays(today, 3))
  await page.getByRole('button', { name: 'Registar entrada' }).click()
  await expect(page.getByRole('alert')).toContainText('A quantidade tem de ser maior do que 0.')

  await page.getByLabel(/^Quantidade/).fill('2')
  await page.getByLabel('Validade').fill(addDays(today, -1))
  await page.getByRole('button', { name: 'Registar entrada' }).click()
  await expect(page.getByRole('alert')).toContainText('A validade não pode ser anterior à data de colheita.')

  expect(posts).toEqual([])
})

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`modo ${colorScheme === 'light' ? 'claro' : 'escuro'}`, () => {
    test.use({ colorScheme })

    test('/app/stock não tem problemas de acessibilidade WCAG 2.1 AA, também com um erro à vista', async ({ page }) => {
      const hasProducts = await openStock(page)
      const axe = async () => {
        await page.waitForLoadState('networkidle')
        const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
        expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([])
      }
      await axe()
      if (hasProducts) {
        await page.getByRole('button', { name: 'Registar entrada' }).click()
        await expect(page.getByRole('alert')).toBeVisible()
        await axe()
      }
    })
  })
}
