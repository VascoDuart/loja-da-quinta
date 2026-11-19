import { describe, expect, it } from 'vitest'

import { formatDate, formatDateTime, formatEuro, formatQuantity, formatUnitPrice } from './format'

// O Intl usa espaços inseparáveis (que \s apanha); normalizam-se para comparar.
const norm = (s: string) => s.replace(/\s/g, ' ')

describe('formatEuro', () => {
  it.each([
    [1143, '11,43 €'],
    [0, '0,00 €'],
    [5, '0,05 €'],
    [123456, '1234,56 €'],
  ])('%i cêntimos → %s', (cents, expected) => {
    expect(norm(formatEuro(cents)).replace(/(\d)\s(\d)/, '$1$2')).toBe(expected)
  })

  it('recusa valores que não são cêntimos inteiros (ADR-0009)', () => {
    expect(() => formatEuro(11.43)).toThrow()
  })
})

describe('formatQuantity', () => {
  it.each([
    [1.25, 'kg', '1,250 kg'],
    [0.75, 'l', '0,750 L'],
    [1, 'duzia', '1 dúzia'],
    [2, 'duzia', '2 dúzias'],
    [1, 'molho', '1 molho'],
    [3, 'un', '3 unidades'],
  ] as const)('%d %s → %s', (value, unit, expected) => {
    expect(formatQuantity(value, unit)).toBe(expected)
  })
})

it('formatUnitPrice junta o preço e a unidade', () => {
  expect(norm(formatUnitPrice(120, 'kg'))).toBe('1,20 € / kg')
  expect(norm(formatUnitPrice(950, 'l'))).toBe('9,50 € / L')
})

describe('datas na hora de Lisboa', () => {
  it('no inverno, Lisboa está em UTC', () => {
    expect(formatDateTime('2026-01-15T11:30:00Z')).toBe('15/01/2026, 11:30')
  })

  it('no verão, Lisboa está em UTC+1', () => {
    expect(formatDateTime('2026-07-01T11:30:00Z')).toBe('01/07/2026, 12:30')
  })

  it('uma venda às 23:30 UTC de verão já é no dia seguinte em Lisboa', () => {
    expect(formatDate('2026-07-31T23:30:00Z')).toBe('01/08/2026')
  })

  it('uma data sem hora (validade de um lote) nunca muda de dia', () => {
    expect(formatDate('2026-11-19')).toBe('19/11/2026')
  })
})
