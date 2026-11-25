import { type Browser, expect, type Locator, type Page } from '@playwright/test'

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

/** "… restam 10,000 kg …" → "10,000" (para escrever no campo da quantidade). */
export async function remaining(radio: Locator): Promise<string> {
  const label = (await radio.getAttribute('aria-label')) ?? (await radio.locator('xpath=..').innerText())
  const left = /restam ([\d,]+) kg/.exec(label)?.[1]
  if (!left) throw new Error(`Lote sem quantidade: ${label}`)
  return left
}

/** Abre os ajustes no produto de teste e espera pelos lotes (ou pela lista vazia). */
export async function openAdjustments(page: Page, name: string) {
  await page.goto('/app/stock/ajustes')
  await expect(page.getByRole('heading', { name: 'Ajustes e quebras', level: 1 })).toBeVisible()
  await page.getByLabel('Produto', { exact: true }).selectOption({ label: name })
  await expect(page.getByRole('radio').first().or(page.getByText('não tem lotes com stock'))).toBeVisible()
}

/** Preenche e envia o formulário dos ajustes para o lote escolhido. */
export async function submitAdjustment(
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
 * Dá como quebra o que resta nos lotes fora de validade de um produto de
 * teste em kg, para não se acumularem na consulta do stock nem nos alertas
 * (os lotes nunca se apagam). A página tem de ter sessão de gestor, e fica
 * nos ajustes desse produto. Recusa produtos que não sejam de teste.
 */
export async function writeOffExpiredLots(page: Page, name: string) {
  if (!name.startsWith('E2E ')) throw new Error(`Não é um produto de teste: ${name}`)
  const expired = page.getByRole('radio', { name: /fora de validade · restam/ })
  await openAdjustments(page, name)
  while ((await expired.count()) > 0) {
    const lot = expired.first()
    const left = await remaining(lot)
    await lot.check()
    await submitAdjustment(page, 'Quebra', left, 'E2E: lote de teste fora de validade')
    await expect(page.getByRole('status').filter({ hasText: 'Quebra registada' })).toBeVisible()
    // Reabrir: o lote a zero sai da lista.
    await openAdjustments(page, name)
  }
}

/**
 * Garante que o produto público de teste tem stock. Os lotes nunca se apagam
 * e estes testes correm em cada deploy de staging: só se regista um lote
 * quando o produto está sem stock, e esse lote dura 7 dias. Assim fica no
 * máximo um lote por dispositivo e por semana, por mais deploys que haja. Os
 * que já passaram da validade levam primeiro uma quebra (writeOffExpiredLots).
 */
export async function ensureInStock(browser: Browser, name: string) {
  const gestor = await browser.newContext()
  const admin = await gestor.newPage()
  await login(admin, 'gestor', '/app/stock/ajustes')
  await writeOffExpiredLots(admin, name)
  await gestor.close()

  const context = await browser.newContext()
  const page = await context.newPage()
  if ((await publicAvailability(page, name)) !== 'disponivel') {
    await registerLot(page, name, '10', addDays(lisbonToday(), 7))
    await expect.poll(() => publicAvailability(page, name)).toBe('disponivel')
  }
  await context.close()
}
