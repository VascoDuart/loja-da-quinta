import { expect, test } from '@playwright/test'

// Só faz sentido contra um ambiente publicado: os cabeçalhos vêm da Vercel
// (vercel.json), não do servidor de desenvolvimento.
test.skip(!process.env.E2E_BASE_URL, 'só contra um ambiente publicado (E2E_BASE_URL)')

test('a Vercel envia os cabeçalhos de segurança', async ({ request }) => {
  const res = await request.get('/')
  const h = res.headers()
  expect(h['content-security-policy']).toContain("default-src 'self'")
  expect(h['content-security-policy']).not.toContain('unsafe-inline')
  expect(h['strict-transport-security']).toContain('max-age=')
  expect(h['x-content-type-options']).toBe('nosniff')
  expect(h['x-frame-options']).toBe('DENY')
})

test('a versão publicada do frontend e da API é a esperada', async ({ page, request }) => {
  const expected = process.env.E2E_EXPECTED_VERSION
  test.skip(!expected, 'sem E2E_EXPECTED_VERSION')

  await page.goto('/')
  await expect(page.locator('meta[name="app-version"]')).toHaveAttribute('content', expected as string)

  const health = await request.get('/api/v1/healthz')
  expect(health.ok()).toBe(true)
  expect((await health.json()) as { version: string }).toMatchObject({ version: expected })
})
