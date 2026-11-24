import { describe, expect, it } from 'vitest'

import { safeNext } from './redirect'

describe('safeNext', () => {
  it.each(['/app/venda', '/app/stock?lote=3', '/app', '/app?x=1'])('aceita %s', (next) => {
    expect(safeNext(next)).toBe(next)
  })

  it.each([
    null,
    '',
    'https://outro-site.pt',
    '//outro-site.pt',
    '/\\outro-site.pt',
    '/app\\..\\..\\x',
    '/application',
    '/entrar',
    'javascript:alert(1)',
  ])('recusa %s e vai para o painel', (next) => {
    expect(safeNext(next)).toBe('/app/painel')
  })
})
