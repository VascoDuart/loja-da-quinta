import { expect, type Page } from '@playwright/test'

import { login } from './accounts'

/**
 * Garante que existem a categoria e o produto de teste (em kg, só interno,
 * nunca no catálogo público) e deixa o produto aberto em edição. Os testes
 * correm também contra staging e os produtos não se apagam: cria-os na
 * primeira vez e depois só os reutiliza. Devolve a linha do produto na lista.
 */
export async function ensureProduct(page: Page, category: string, name: string) {
  await login(page, 'gestor', '/app/produtos')
  await expect(page.getByRole('heading', { name: 'Categorias' })).toBeVisible()

  if ((await page.locator('.categories__name', { hasText: category }).count()) === 0) {
    await page.getByLabel('Nova categoria').fill(category)
    await page.getByRole('button', { name: 'Adicionar' }).click()
    await expect(page.locator('.categories__name', { hasText: category })).toBeVisible()
  }

  // Espera pela lista de produtos (ou pela mensagem de lista vazia) antes de
  // decidir se o produto existe.
  const list = page.getByRole('list', { name: 'Produtos' })
  await expect(list.or(page.getByText('Ainda não há produtos.'))).toBeVisible()
  const row = list.getByRole('listitem').filter({ hasText: name })
  if ((await row.count()) === 0) {
    await page.getByRole('button', { name: 'Novo produto' }).click()
    await page.getByLabel('Nome', { exact: true }).fill(name)
    await page.getByLabel('Categoria', { exact: true }).selectOption({ label: category })
    await page.getByLabel(/^Preço por kg/).fill('2,50')
    await page.getByRole('checkbox', { name: 'Visível no catálogo público' }).uncheck()
    await page.getByRole('button', { name: 'Criar produto' }).click()
    // Depois de criar, a página fica em edição do produto novo.
    await expect(page.getByRole('heading', { name: `Editar ${name}` })).toBeVisible()
  } else {
    await row.getByRole('button', { name: `Editar ${name}` }).click()
  }
  return row
}
