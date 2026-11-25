import AxeBuilder from '@axe-core/playwright'
import { type Browser, expect, type Locator, type Page, test, type TestInfo } from '@playwright/test'

import { addDays, lisbonToday } from '../src/shared/lib/format'
import { login } from './accounts'
import { ensureProduct } from './products'
import { registerLot } from './stock'

/**
 * Ajustes e quebras (RF-06) de ponta a ponta. Também corre contra staging,
 * onde os lotes e os movimentos nunca se apagam, por isso:
 *
 * - cada dispositivo usa sempre o mesmo produto, só interno ("E2E Ajustes
 *   <dispositivo>"), para não mexer no stock do produto público que os testes
 *   do catálogo verificam;
 * - cada execução tira 1 kg e volta a pô-lo: o lote não se esgota;
 * - só se regista um lote quando não há nenhum dentro da validade, e esse
 *   dura 90 dias; os lotes de teste que passam da validade levam uma quebra
 *   do que resta e saem da lista. Fica no máximo um lote à vista.
 */
// Os testes mexem no mesmo lote: correm um de cada vez.
test.describe.configure({ mode: 'serial' })

const productName = (info: TestInfo) => `E2E Ajustes ${info.project.name}`

/** Lotes dentro da validade ("Validade 02/12/2026 · restam …"). */
const validLots = (page: Page) => page.getByRole('radio', { name: /^Validade [\d/]+ · restam/ })
const expiredLots = (page: Page) => page.getByRole('radio', { name: /fora de validade · restam/ })

/** "… restam 10,000 kg …" → "10,000" (para escrever no campo da quantidade). */
async function remaining(radio: Locator): Promise<string> {
  const label = (await radio.getAttribute('aria-label')) ?? (await radio.locator('xpath=..').innerText())
  const left = /restam ([\d,]+) kg/.exec(label)?.[1]
  if (!left) throw new Error(`Lote sem quantidade: ${label}`)
  return left
}

/** Abre os ajustes no produto de teste e espera pelos lotes (ou pela lista vazia). */
async function openAdjustments(page: Page, name: string) {
  await page.goto('/app/stock/ajustes')
  await expect(page.getByRole('heading', { name: 'Ajustes e quebras', level: 1 })).toBeVisible()
  await page.getByLabel('Produto', { exact: true }).selectOption({ label: name })
  await expect(page.getByRole('radio').first().or(page.getByText('não tem lotes com stock'))).toBeVisible()
}

async function submit(
  page: Page,
  kind: 'Quebra' | 'Ajuste para menos' | 'Ajuste para mais',
  quantity: string,
  reason: string,
) {
  await page.getByRole('radio', { name: kind, exact: true }).check()
  await page.getByLabel(/^Quantidade/).fill(quantity)
  await page.getByLabel('Motivo').fill(reason)
  await page.getByRole('button', { name: kind === 'Quebra' ? 'Registar quebra' : 'Registar ajuste' }).click()
}

/**
 * Deixa o produto de teste com um lote dentro da validade e com pelo menos
 * 1 kg, e a página aberta nos ajustes desse produto, como gestor.
 */
async function prepare(browser: Browser, page: Page, info: TestInfo) {
  const name = productName(info)
  // Categoria própria: o cartão de um produto mostra a categoria, e o
  // products.spec procura o produto dele pelo texto "E2E <dispositivo>".
  await ensureProduct(page, name, name)
  await openAdjustments(page, name)

  // Os lotes de teste fora de validade: quebra do que resta (saem da lista).
  while ((await expiredLots(page).count()) > 0) {
    const lot = expiredLots(page).first()
    const left = await remaining(lot)
    await lot.check()
    await submit(page, 'Quebra', left, 'E2E: lote de teste fora de validade')
    await expect(page.getByRole('status').filter({ hasText: 'Quebra registada' })).toBeVisible()
    await openAdjustments(page, name)
  }

  const valid = validLots(page)
  const usable = (await valid.count()) > 0 && Number((await remaining(valid.first())).replace(',', '.')) >= 1
  if (!usable) {
    const funcionario = await browser.newContext()
    await registerLot(await funcionario.newPage(), name, '10', addDays(lisbonToday(), 90))
    await funcionario.close()
    await openAdjustments(page, name)
  }
  await expect(validLots(page).first()).toBeVisible()
  await validLots(page).first().check()
  return name
}

test('o gestor regista uma quebra e um ajuste que a repõe', async ({ browser, page }, testInfo) => {
  const name = await prepare(browser, page, testInfo)
  const lot = validLots(page).first()
  const before = await remaining(lot)

  await submit(page, 'Quebra', '1', 'E2E: quebra de teste')
  await expect(page.getByRole('status').filter({ hasText: 'Quebra registada' })).toContainText(
    `Quebra registada: 1,000 kg de ${name}.`,
  )
  await expect(validLots(page).first()).not.toHaveAccessibleName(new RegExp(`restam ${before} kg`))

  await validLots(page).first().check()
  await submit(page, 'Ajuste para mais', '1', 'E2E: repor a quebra de teste')
  await expect(page.getByRole('status').filter({ hasText: 'Ajuste registado' })).toContainText(
    `Ajuste registado: mais 1,000 kg de ${name}. O lote fica com ${before} kg.`,
  )
  await expect(validLots(page).first()).toHaveAccessibleName(new RegExp(`restam ${before} kg`))
})

test('o ecrã recusa tirar mais do que resta sem enviar nada', async ({ browser, page }, testInfo) => {
  await prepare(browser, page, testInfo)
  const posts: string[] = []
  page.on('request', (r) => {
    if (r.method() === 'POST' && r.url().includes('/api/v1/stock/adjustments')) posts.push(r.url())
  })
  await submit(page, 'Quebra', '100000', 'E2E: nunca enviado')
  await expect(page.getByRole('alert')).toContainText('Este lote só tem')
  expect(posts).toEqual([])
})

test('o funcionário não tem acesso aos ajustes', async ({ page }) => {
  await login(page, 'funcionario', '/app/stock')
  await expect(
    page.getByRole('button', { name: 'Registar entrada' }).or(page.getByText('Ainda não há produtos')),
  ).toBeVisible()
  await expect(page.getByRole('link', { name: 'Registar um ajuste ou uma quebra' })).toHaveCount(0)
  await page.goto('/app/stock/ajustes')
  await expect(page.getByRole('heading', { name: 'Sem permissão' })).toBeVisible()
})

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`modo ${colorScheme === 'light' ? 'claro' : 'escuro'}`, () => {
    test.use({ colorScheme })

    test('/app/stock/ajustes não tem problemas de acessibilidade WCAG 2.1 AA, também com um erro à vista', async ({
      browser,
      page,
    }, testInfo) => {
      await prepare(browser, page, testInfo)
      const axe = async () => {
        await page.waitForLoadState('networkidle')
        const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
        expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([])
      }
      await axe()
      await page.getByRole('button', { name: 'Registar quebra' }).click()
      await expect(page.getByRole('alert')).toBeVisible()
      await axe()
    })
  })
}
