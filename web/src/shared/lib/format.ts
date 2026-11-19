/**
 * Formatação para pt-PT (RNF-16).
 *
 * - Dinheiro chega da API em cêntimos inteiros (ADR-0009) e só aqui vira euros.
 * - Quantidades ao kg e ao litro têm 3 casas decimais (gramas e mililitros).
 * - Datas chegam em UTC e mostram-se na hora de Lisboa, incluindo a mudança
 *   de hora de verão.
 */
export type Unit = 'kg' | 'l' | 'un' | 'molho' | 'duzia'

const LOCALE = 'pt-PT'
const TIME_ZONE = 'Europe/Lisbon'

const euro = new Intl.NumberFormat(LOCALE, { style: 'currency', currency: 'EUR' })

/** 1143 → "11,43 €" */
export function formatEuro(cents: number): string {
  if (!Number.isInteger(cents)) throw new Error(`Valor em cêntimos tem de ser inteiro: ${cents}`)
  return euro.format(cents / 100)
}

const unitLabel: Record<Unit, { one: string; many: string }> = {
  kg: { one: 'kg', many: 'kg' },
  l: { one: 'L', many: 'L' },
  un: { one: 'unidade', many: 'unidades' },
  molho: { one: 'molho', many: 'molhos' },
  duzia: { one: 'dúzia', many: 'dúzias' },
}

const decimal = new Intl.NumberFormat(LOCALE, { minimumFractionDigits: 3, maximumFractionDigits: 3 })
const integer = new Intl.NumberFormat(LOCALE, { maximumFractionDigits: 0 })

/** 1.25, 'kg' → "1,250 kg"; 2, 'duzia' → "2 dúzias" */
export function formatQuantity(value: number, unit: Unit): string {
  const label = unitLabel[unit]
  if (unit === 'kg' || unit === 'l') return `${decimal.format(value)} ${label.many}`
  return `${integer.format(value)} ${value === 1 ? label.one : label.many}`
}

/** 120, 'kg' → "1,20 € / kg" */
export function formatUnitPrice(cents: number, unit: Unit): string {
  return `${formatEuro(cents)} / ${unitLabel[unit].one}`
}

const dateParts = { day: '2-digit', month: '2-digit', year: 'numeric' } as const
const dateTime = new Intl.DateTimeFormat(LOCALE, {
  timeZone: TIME_ZONE,
  ...dateParts,
  hour: '2-digit',
  minute: '2-digit',
})
const dateOnly = new Intl.DateTimeFormat(LOCALE, { timeZone: TIME_ZONE, ...dateParts })

/** "2026-07-01T11:30:00Z" → "01/07/2026, 12:30" (hora de Lisboa) */
export function formatDateTime(iso: string): string {
  return dateTime.format(new Date(iso))
}

/** "2026-11-19" ou ISO com hora → "19/11/2026" */
export function formatDate(iso: string): string {
  // Uma data sem hora ("2026-11-19") é um dia do calendário, não um instante:
  // lê-se ao meio-dia UTC para nunca mudar de dia por causa do fuso.
  const d = /^\d{4}-\d{2}-\d{2}$/.test(iso) ? new Date(`${iso}T12:00:00Z`) : new Date(iso)
  return dateOnly.format(d)
}
