import { type Browser, expect, type Page } from '@playwright/test'

import { addDays, lisbonToday } from '../src/shared/lib/format'
import { login } from './accounts'

/**
 * Disponibilidade de um produto no catálogo público, sem passar pela cache
 * de 60 s (o parâmetro a mais muda o URL; a API ignora-o).
 */
export async function publicAvailability(page: Page, name: string): Promise<string | undefined> {
  const res = await page.request.get(`/api/v1/public/catalog?e2e=${Date.now()}`)
  expect(res.ok()).toBe(true)
  const items = (await res.json()) as { name: string; availability: string }[]
  return items.find((it) => it.name === name)?.availability
}

/** O funcionário regista uma entrada de stock pelo ecrã /app/stock. */
export async function registerLot(page: Page, name: string, quantity: string, expiresOn: string) {
  await login(page, 'funcionario', '/app/stock')
  await page.getByLabel('Produto', { exact: true }).selectOption({ label: name })
  await page.getByLabel(/^Quantidade/).fill(quantity)
  await page.getByLabel('Validade').fill(expiresOn)
  await page.getByRole('button', { name: 'Registar entrada' }).click()
  // filter: o aviso "a acordar o servidor" também pode estar à vista (role status).
  await expect(page.getByRole('status').filter({ hasText: 'Entrada registada' })).toContainText(
    `Entrada registada: ${quantity},000 kg de ${name}`,
  )
  await expect(page.getByRole('list', { name: 'Entradas registadas' })).toContainText(name)
}

/**
 * Garante que o produto público de teste tem stock. Os lotes nunca se apagam
 * e estes testes correm em cada deploy de staging: só se regista um lote
 * quando o produto está sem stock, e esse lote dura 7 dias. Assim fica no
 * máximo um lote por dispositivo e por semana, por mais deploys que haja.
 */
export async function ensureInStock(browser: Browser, name: string) {
  const context = await browser.newContext()
  const page = await context.newPage()
  if ((await publicAvailability(page, name)) !== 'disponivel') {
    await registerLot(page, name, '10', addDays(lisbonToday(), 7))
    await expect.poll(() => publicAvailability(page, name)).toBe('disponivel')
  }
  await context.close()
}
