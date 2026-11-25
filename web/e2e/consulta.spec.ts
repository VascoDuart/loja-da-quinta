import { expect, type Page, test, type TestInfo } from '@playwright/test'

import { login } from './accounts'
import { ensureProduct } from './products'

/**
 * Consulta do stock e painel (RF-11) de ponta a ponta, também contra staging.
 * Não regista lotes nem movimentos, que nunca se apagam: cada dispositivo usa
 * sempre o mesmo produto só interno, sem lotes e com um limiar de 5 kg, que
 * por isso aparece sempre com stock baixo.
 */
const productName = (info: TestInfo) => `E2E Consulta ${info.project.name}`

/** Cria (na primeira vez) o produto de teste e garante o limiar de 5 kg. */
async function ensureLowProduct(page: Page, name: string) {
  // Categoria própria: o products.spec procura o produto dele pelo texto
  // "E2E <dispositivo>", que o cartão de um produto também mostra na categoria.
  await ensureProduct(page, name, name)
  const threshold = page.getByLabel(/^Stock baixo abaixo de/)
  await expect(threshold).toBeVisible()
  if ((await threshold.inputValue()) !== '5') {
    await threshold.fill('5')
    const saved = page.waitForResponse((r) => r.request().method() === 'PUT' && r.url().includes('/api/v1/products/'))
    await page.getByRole('button', { name: 'Guardar', exact: true }).click()
    expect((await saved).status()).toBe(200)
  }
}

test('um produto abaixo do limiar aparece destacado na consulta e no painel', async ({ browser, page }, testInfo) => {
  const name = productName(testInfo)
  const gestor = await browser.newContext()
  await ensureLowProduct(await gestor.newPage(), name)
  await gestor.close()

  await login(page, 'funcionario', '/app/stock')
  const consulta = page.getByRole('region', { name: 'Stock atual' })
  // O limiar aparece no grupo "Stock baixo"; o produto, na lista por produto.
  await expect(consulta.getByText(`${name}: 0,000 kg para vender (limiar: 5,000 kg)`)).toBeVisible()
  const row = consulta
    .getByRole('listitem')
    .filter({ hasText: name })
    .filter({ hasText: /Para vender:/ })
  await expect(row).toContainText('Para vender: 0,000 kg')
  await expect(row.getByText('Stock baixo', { exact: true })).toBeVisible()
  await expect(row.getByText('Sem stock', { exact: true })).toBeVisible()

  await page.getByRole('link', { name: 'Painel' }).click()
  const resumo = page.getByRole('region', { name: 'Stock', exact: true })
  await expect(resumo.getByText('Stock baixo', { exact: true })).toBeVisible()
  await resumo.getByRole('link', { name: 'Ver o stock' }).click()
  await expect(page.getByRole('heading', { name: 'Stock', level: 1 })).toBeVisible()
})

test('o job dos alertas recusa quem não tem o token, mesmo um gestor', async ({ page }) => {
  // Nunca envia nada: sem o token, a API recusa antes de olhar para o stock.
  const anonymous = await page.request.post('/api/v1/jobs/stock-alerts')
  expect(anonymous.status()).toBe(401)
  const wrong = await page.request.post('/api/v1/jobs/stock-alerts', {
    headers: { 'X-Job-Token': 'x'.repeat(40) },
  })
  expect(wrong.status()).toBe(401)

  const session = page.waitForResponse((r) => r.url().includes('/api/v1/auth/login'))
  await login(page, 'gestor', '/app/painel')
  const { accessToken } = (await (await session).json()) as { accessToken: string }
  const gestor = await page.request.post('/api/v1/jobs/stock-alerts', {
    headers: { Authorization: `Bearer ${accessToken}` },
  })
  expect(gestor.status()).toBe(401)
})
