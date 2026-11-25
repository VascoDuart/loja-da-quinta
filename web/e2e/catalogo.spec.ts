import { fileURLToPath } from 'node:url'

import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'

import { login } from './accounts'
import { ensureInStock } from './stock'

/**
 * Catálogo público (UC-10) de ponta a ponta. Também corre contra staging, por
 * isso cada dispositivo usa sempre a mesma categoria e o mesmo produto de
 * teste: cria-os na primeira vez (públicos, ao contrário dos de products.spec)
 * e depois só os reutiliza. Nunca se apagam. Para aparecer "Disponível", o
 * produto precisa de stock: o funcionário regista um lote quando não há
 * (ensureInStock).
 */
// Os testes partilham o mesmo produto: correm um de cada vez (como em products.spec).
test.describe.configure({ mode: 'serial' })

const photo = fileURLToPath(new URL('./fixtures/laranja.jpg', import.meta.url))

/** Garante que existe um produto público de teste, com imagem. */
async function ensurePublicProduct(page: Page, name: string) {
  await login(page, 'gestor', '/app/produtos')
  await expect(page.getByRole('heading', { name: 'Categorias' })).toBeVisible()

  if ((await page.locator('.categories__name', { hasText: name }).count()) === 0) {
    await page.getByLabel('Nova categoria').fill(name)
    await page.getByRole('button', { name: 'Adicionar' }).click()
    await expect(page.locator('.categories__name', { hasText: name })).toBeVisible()
  }

  // Espera pela lista (ou pela mensagem de lista vazia) antes de decidir.
  const list = page.getByRole('list', { name: 'Produtos' })
  await expect(list.or(page.getByText('Ainda não há produtos.'))).toBeVisible()
  const row = list.getByRole('listitem').filter({ hasText: name })
  if ((await row.count()) === 0) {
    await page.getByRole('button', { name: 'Novo produto' }).click()
    await page.getByLabel('Nome', { exact: true }).fill(name)
    await page.getByLabel('Categoria', { exact: true }).selectOption({ label: name })
    await page.getByLabel(/^Preço por kg/).fill('3,20')
    await page.getByRole('checkbox', { name: 'Visível no catálogo público' }).check()
    await page.getByRole('button', { name: 'Criar produto' }).click()
    await expect(page.getByRole('heading', { name: `Editar ${name}` })).toBeVisible()
  } else if ((await row.locator('img').count()) === 0) {
    // Existe, mas a execução que o criou parou antes da imagem.
    await row.getByRole('button', { name: `Editar ${name}` }).click()
  } else {
    return
  }

  await page.locator('input[type="file"]').setInputFiles(photo)
  await page.getByRole('button', { name: 'Enviar imagem' }).click()
  // Só acaba quando a lista mostra a miniatura: antes disso o envio ainda pode estar a decorrer.
  await expect(row.locator('img')).toBeVisible()
}

async function expectNoAxeViolations(page: Page) {
  await page.waitForLoadState('networkidle')
  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
  expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([])
}

test('qualquer pessoa vê o produto público no catálogo e abre o detalhe, sem login', async ({ browser }, testInfo) => {
  const name = `E2E Catálogo ${testInfo.project.name}`
  const admin = await browser.newContext()
  await ensurePublicProduct(await admin.newPage(), name)
  await admin.close()
  await ensureInStock(browser, name)

  // Um contexto novo: sem sessão e sem a cache de 60 s do catálogo.
  const visitor = await browser.newContext()
  const page = await visitor.newPage()
  await page.goto('/')
  const card = page.getByRole('list', { name: 'Produtos' }).getByRole('listitem').filter({ hasText: name })
  // O catálogo pode estar em cache (60 s) na Vercel ou no browser: na primeira
  // execução, o produto ou o stock acabaram de ser criados e podem demorar a
  // aparecer.
  await expect(async () => {
    await page.reload()
    await expect(card).toContainText('Disponível', { timeout: 3_000 })
  }).toPass({ timeout: 90_000, intervals: [2_000, 5_000, 10_000] })

  await expect(card).toContainText('€ / kg')
  await expect(card).toContainText('Época: Todo o ano')
  const thumb = card.locator('img')
  await expect(thumb).toHaveAttribute('src', /\/image\?size=thumb&v=[0-9a-f]{64}$/)
  await expect.poll(() => thumb.evaluate((img: { naturalWidth: number }) => img.naturalWidth)).toBeGreaterThan(0)

  // O filtro por categoria deixa só a categoria do produto.
  await page.getByRole('button', { name }).click()
  await expect(page.getByRole('list', { name: 'Produtos' }).getByRole('listitem')).toHaveCount(1)

  await card.getByRole('link').click()
  await expect(page).toHaveURL(/\/produtos\/[0-9a-f-]{36}$/)
  await expect(page.getByRole('heading', { name, level: 1 })).toBeVisible()
  await expect(page).toHaveTitle('Produto · Loja da Quinta')
  await expect(page.getByRole('article')).toContainText('Disponível')
  const large = page.getByRole('article').locator('img')
  await expect(large).toHaveAttribute('src', /\/image\?size=large&v=[0-9a-f]{64}$/)

  // A API nunca devolve quantidades nem campos internos (sem sessão).
  const body = await (await page.request.get('/api/v1/public/catalog')).text()
  for (const forbidden of ['lowStock', 'vatRate', 'isPublic', 'createdAt', 'updatedAt']) {
    expect(body).not.toContain(forbidden)
  }
  await visitor.close()
})

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`modo ${colorScheme === 'light' ? 'claro' : 'escuro'}`, () => {
    test.use({ colorScheme })

    test('o catálogo com produtos e o detalhe não têm problemas de acessibilidade WCAG 2.1 AA', async ({
      browser,
    }, testInfo) => {
      const name = `E2E Catálogo ${testInfo.project.name}`
      const admin = await browser.newContext({ colorScheme })
      await ensurePublicProduct(await admin.newPage(), name)
      await admin.close()
      await ensureInStock(browser, name)

      const visitor = await browser.newContext({ colorScheme })
      const page = await visitor.newPage()
      await page.goto('/')
      const card = page.getByRole('list', { name: 'Produtos' }).getByRole('listitem').filter({ hasText: name })
      await expect(async () => {
        await page.reload()
        await expect(card).toBeVisible({ timeout: 3_000 })
      }).toPass({ timeout: 90_000, intervals: [2_000, 5_000, 10_000] })
      await expectNoAxeViolations(page)

      await card.getByRole('link').click()
      await expect(page.getByRole('heading', { name, level: 1 })).toBeVisible()
      await expectNoAxeViolations(page)
      await visitor.close()
    })
  })
}
