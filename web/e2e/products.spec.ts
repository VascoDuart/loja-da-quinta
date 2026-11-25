import { fileURLToPath } from 'node:url'

import { expect, test } from '@playwright/test'

import { login } from './accounts'
import { ensureProduct } from './products'

/**
 * Catálogo do backoffice (UC-03) de ponta a ponta. Também corre contra
 * staging, por isso cada dispositivo usa sempre a mesma categoria e o mesmo
 * produto: cria-os na primeira vez e depois só os edita. São produtos só
 * internos, que nunca aparecem no catálogo público.
 */
// Os dois testes mexem no mesmo produto: correm um de cada vez.
test.describe.configure({ mode: 'serial' })

const photo = fileURLToPath(new URL('./fixtures/laranja.jpg', import.meta.url))

test('o gestor cria ou edita um produto e troca a imagem', async ({ page }, testInfo) => {
  const name = `E2E ${testInfo.project.name}`
  const row = await ensureProduct(page, `E2E ${testInfo.project.name}`, name)

  // Preço novo em cada execução, para provar que a gravação chega à API.
  const cents = 100 + Math.floor(Math.random() * 900)
  const text = `${Math.floor(cents / 100)},${String(cents % 100).padStart(2, '0')}`
  await page.getByLabel(/^Preço por kg/).fill(text)
  await page.getByRole('button', { name: 'Guardar' }).click()
  await expect(page.getByRole('status')).toContainText(`Alterações a ${name} guardadas.`)
  await expect(row).toContainText(`${text} € / kg`)

  // Imagem: escolher, pré-visualizar, enviar. A miniatura usa o URL com a versão.
  await row.getByRole('button', { name: `Editar ${name}` }).click()
  await page.locator('input[type="file"]').setInputFiles(photo)
  await expect(page.getByRole('img', { name: /Pré-visualização/ })).toBeVisible()
  await page.getByRole('button', { name: 'Enviar imagem' }).click()
  await expect(page.getByRole('img', { name })).toBeVisible()
  const thumb = row.locator('img')
  await expect(thumb).toHaveAttribute('src', /\/image\?size=thumb&v=[0-9a-f]{64}$/)
  // A imagem carregou de facto (largura natural > 0), através do rewrite.
  await expect.poll(() => thumb.evaluate((img: { naturalWidth: number }) => img.naturalWidth)).toBeGreaterThan(0)
})

test('duas pessoas a editar o mesmo produto: a segunda é avisada, nada se perde', async ({
  browser,
  page,
}, testInfo) => {
  const name = `E2E ${testInfo.project.name}`
  await ensureProduct(page, `E2E ${testInfo.project.name}`, name)
  // A primeira pessoa tem o formulário aberto (versão lida agora).

  // A segunda pessoa, noutro browser, grava primeiro.
  const other = await browser.newContext()
  const otherPage = await other.newPage()
  await login(otherPage, 'gestor', '/app/produtos')
  await otherPage.getByRole('button', { name: `Editar ${name}` }).click()
  await otherPage.getByLabel(/^Preço por kg/).fill('9,99')
  await otherPage.getByRole('button', { name: 'Guardar' }).click()
  await expect(otherPage.getByRole('status')).toContainText('guardadas')
  await other.close()

  // A primeira tenta gravar a versão antiga: a API recusa (412).
  await page.getByLabel(/^Preço por kg/).fill('1,11')
  await page.getByRole('button', { name: 'Guardar' }).click()
  await expect(page.getByRole('alert')).toContainText('Outra pessoa alterou este produto')

  // Tentar de novo continua a ser recusado: a alteração da outra pessoa nunca
  // é apagada sem aviso. "Carregar a versão atual" mostra o preço dela.
  await page.getByRole('button', { name: 'Guardar' }).click()
  await expect(page.getByRole('alert')).toContainText('Outra pessoa alterou este produto')
  await page.getByRole('button', { name: 'Carregar a versão atual' }).click()
  await expect(page.getByLabel(/^Preço por kg/)).toHaveValue('9,99')
})
