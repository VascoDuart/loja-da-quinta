import { describe, expect, it } from 'vitest'

import {
  centsToInput,
  formatDate,
  formatDateTime,
  formatEuro,
  formatQuantity,
  formatSeason,
  formatUnitPrice,
  milliToInput,
  parseEuroToCents,
  parseQuantityToMilli,
} from './format'

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

describe('formulários: texto ⇄ inteiros', () => {
  it.each([
    ['1,20', 120],
    ['1.2', 120],
    ['3', 300],
    [' 0,05 ', 5],
    ['12,5 €', 1250],
    ['999999,99', 99999999],
  ])('parseEuroToCents(%j) = %d', (text, cents) => {
    expect(parseEuroToCents(text)).toBe(cents)
  })

  it.each(['', 'abc', '1,234', '-1', '1,2,3', '1e3', '0x10'])('parseEuroToCents(%j) é inválido', (text) => {
    expect(parseEuroToCents(text)).toBeNull()
  })

  it('0,1 + 0,2 dá exatamente 30 cêntimos (sem vírgula flutuante)', () => {
    expect((parseEuroToCents('0,1') ?? 0) + (parseEuroToCents('0,2') ?? 0)).toBe(30)
  })

  it.each([
    [120, '1,20'],
    [5, '0,05'],
    [100000, '1000,00'],
  ])('centsToInput(%d) = %j', (cents, text) => {
    expect(centsToInput(cents)).toBe(text)
  })

  it.each([
    ['2,5', 'kg', 2500],
    ['0,125', 'l', 125],
    ['3', 'un', 3000],
    ['12', 'duzia', 12000],
  ] as const)('parseQuantityToMilli(%j, %s) = %d', (text, unit, milli) => {
    expect(parseQuantityToMilli(text, unit)).toBe(milli)
  })

  it.each([
    ['1,5', 'un'],
    ['0,5', 'duzia'],
    ['1,2345', 'kg'],
    ['-1', 'kg'],
  ] as const)('parseQuantityToMilli(%j, %s) é inválido', (text, unit) => {
    expect(parseQuantityToMilli(text, unit)).toBeNull()
  })

  it.each([
    [2500, '2,5'],
    [125, '0,125'],
    [3000, '3'],
    [0, '0'],
  ])('milliToInput(%d) = %j', (milli, text) => {
    expect(milliToInput(milli)).toBe(text)
  })

  it.each([
    [[], 'Todo o ano'],
    [[3, 4, 5], 'mar, abr, mai'],
    [[12, 1, 2], 'dez, jan, fev'],
    [[1, 2, 11, 12], 'nov, dez, jan, fev'],
  ])('formatSeason(%j) = %j', (months, text) => {
    expect(formatSeason(months)).toBe(text)
  })
})
