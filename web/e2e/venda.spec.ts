import AxeBuilder from '@axe-core/playwright'
import { type Browser, expect, type Page, test, type TestInfo } from '@playwright/test'

import { addDays, lisbonToday } from '../src/shared/lib/format'
import { login } from './accounts'
import { ensureProduct } from './products'
import { openAdjustments, registerLot, remaining, submitAdjustment, writeOffExpiredLots } from './stock'

/**
 * Counter sale (UC-06) end to end. It also runs against staging, where sales,
 * lots and movements are never deleted, so:
 *
 * - each device always uses the same internal product ("E2E Venda <device>",
 *   2,50 € / kg), never the public one the catalog tests check;
 * - each run sells 1 kg and the manager puts it back with an adjustment, so
 *   the lot never runs out; only one sale record per run is left behind;
 * - a lot is only registered when there is none within its expiry date with
 *   at least 1 kg, and it lasts 90 days; expired test lots are written off.
 */
test.describe.configure({ mode: 'serial' })

const productName = (info: TestInfo) => `E2E Venda ${info.project.name}`

const validLots = (page: Page) => page.getByRole('radio', { name: /^Validade [\d/]+ · restam/ })

/**
 * Leaves the test product with a sellable lot holding at least 1 kg (the one
 * FEFO sells first). The page ends signed in as the manager, on the product's
 * adjustments.
 */
async function prepare(browser: Browser, page: Page, info: TestInfo) {
  const name = productName(info)
  // Its own category: a product card shows its category, and products.spec
  // looks for its product by the text "E2E <device>".
  await ensureProduct(page, name, name)
  await writeOffExpiredLots(page, name)
  const valid = validLots(page)
  const usable = (await valid.count()) > 0 && Number((await remaining(valid.first())).replace(',', '.')) >= 1
  if (!usable) {
    const clerk = await browser.newContext()
    await registerLot(await clerk.newPage(), name, '10', addDays(lisbonToday(), 90))
    await clerk.close()
    await openAdjustments(page, name)
  }
  await expect(validLots(page).first()).toBeVisible()
  return name
}

/** The clerk's sale screen, with the products loaded. */
async function openSale(browser: Browser) {
  const context = await browser.newContext()
  const page = await context.newPage()
  await login(page, 'funcionario', '/app/venda')
  await expect(page.getByRole('list', { name: 'Resultados da pesquisa' }).getByRole('button').first()).toBeVisible()
  return { context, page }
}

test('the clerk records a sale of 1 kg and the manager puts the stock back', async ({ browser, page }, testInfo) => {
  const name = await prepare(browser, page, testInfo)
  const before = await remaining(validLots(page).first())

  const clerk = await openSale(browser)
  const sale = clerk.page
  await sale.getByLabel('Pesquisar produto').fill(name)
  await sale.getByRole('button', { name: new RegExp(`^${name}.*Juntar$`) }).click()
  await sale.getByLabel(`${name} (kg)`).fill('1')
  await sale.getByLabel('Numerário').check()
  await expect(sale.getByTestId('venda-total')).toHaveText('2,50 €')
  await sale.getByRole('button', { name: 'Confirmar registo de venda' }).click()

  await expect(sale.getByRole('heading', { name: /^Venda V-\d+ registada$/ })).toBeVisible()
  await expect(sale.getByRole('table')).toContainText(name)
  await expect(sale.getByRole('table')).toContainText('2,50 €')
  await sale.getByRole('button', { name: 'Nova venda' }).click()
  await expect(sale.getByRole('button', { name: 'Confirmar registo de venda' })).toBeDisabled()
  await clerk.context.close()

  // FEFO took the kilo from the lot that expires first: put it back.
  await openAdjustments(page, name)
  const lot = validLots(page).first()
  await expect(lot).not.toHaveAccessibleName(new RegExp(`restam ${before} kg`))
  await lot.check()
  await submitAdjustment(page, 'Ajuste para mais', '1', 'E2E: repor a venda de teste')
  await expect(page.getByRole('status').filter({ hasText: 'Ajuste registado' })).toContainText(
    `O lote fica com ${before} kg.`,
  )
})

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`${colorScheme} mode`, () => {
    test.use({ colorScheme })

    test('/app/venda has no WCAG 2.1 AA problems, also with a line and an error showing', async ({
      browser,
      page,
    }, testInfo) => {
      const name = await prepare(browser, page, testInfo)
      const clerk = await openSale(browser)
      const sale = clerk.page
      await sale.emulateMedia({ colorScheme })
      const axe = async () => {
        await sale.waitForLoadState('networkidle')
        const results = await new AxeBuilder({ page: sale })
          .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
          .analyze()
        expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([])
      }
      await axe()
      await sale.getByLabel('Pesquisar produto').fill(name)
      await sale.getByRole('button', { name: new RegExp(`^${name}.*Juntar$`) }).click()
      // No quantity and no payment: the screen refuses without sending anything.
      await sale.getByRole('button', { name: 'Confirmar registo de venda' }).click()
      await expect(sale.getByRole('alert')).toBeVisible()
      await axe()
      await clerk.context.close()
    })
  })
}
