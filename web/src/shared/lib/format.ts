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

// --- Formulários: texto escrito pela pessoa ⇄ inteiros da API -------------
// Sem vírgula flutuante pelo meio (ADR-0009): "1,2" vira 120 cêntimos a
// partir dos dígitos, nunca de 1.2 * 100.

/** "1,20", "1.2" ou "3" → cêntimos; null se não for um preço válido. */
export function parseEuroToCents(text: string): number | null {
  const m = /^(\d{1,6})(?:[.,](\d{1,2}))?$/.exec(text.trim().replace(/\s*€$/, ''))
  if (!m) return null
  return Number(m[1]) * 100 + Number((m[2] ?? '').padEnd(2, '0'))
}

/** 120 → "1,20" (para pôr num campo de texto). */
export function centsToInput(cents: number): string {
  return `${Math.trunc(cents / 100)},${String(cents % 100).padStart(2, '0')}`
}

/**
 * Quantidade escrita → milésimos. kg e litros aceitam até 3 casas decimais;
 * unidades, molhos e dúzias só inteiros. null se não for válida.
 */
export function parseQuantityToMilli(text: string, unit: Unit): number | null {
  const decimal = unit === 'kg' || unit === 'l'
  const m = (decimal ? /^(\d{1,9})(?:[.,](\d{1,3}))?$/ : /^(\d{1,9})$/).exec(text.trim())
  if (!m) return null
  return Number(m[1]) * 1000 + Number((m[2] ?? '').padEnd(3, '0'))
}

/** 2500, 'kg' → "2,5"; 3000, 'un' → "3" (para pôr num campo de texto). */
export function milliToInput(milli: number): string {
  const whole = Math.trunc(milli / 1000)
  const frac = String(milli % 1000)
    .padStart(3, '0')
    .replace(/0+$/, '')
  return frac ? `${whole},${frac}` : String(whole)
}

const monthNames = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez']

/** Nome curto do mês (1 = jan). */
export function monthShort(m: number): string {
  return monthNames[m - 1] ?? String(m)
}

/** [] → "Todo o ano"; [12, 1, 2] → "dez, jan, fev" (pela ordem da época). */
export function formatSeason(months: readonly number[]): string {
  if (months.length === 0 || months.length === 12) return 'Todo o ano'
  const sorted = [...months].sort((a, b) => a - b)
  // Uma época que passa a passagem de ano (dez → fev) lê-se a partir de dez.
  const gap = sorted.findIndex((m, i) => i > 0 && m - (sorted[i - 1] ?? m) > 1)
  const ordered =
    sorted[0] === 1 && sorted.at(-1) === 12 && gap > 0 ? [...sorted.slice(gap), ...sorted.slice(0, gap)] : sorted
  return ordered.map(monthShort).join(', ')
}

/** Mês atual (1 = jan) na hora de Lisboa, para saber o que está na época. */
export function lisbonMonth(now: Date = new Date()): number {
  return Number(new Intl.DateTimeFormat('en-US', { timeZone: TIME_ZONE, month: 'numeric' }).format(now))
}

/**
 * Dia de hoje na hora de Lisboa, como "2026-11-25" (o formato dos campos de
 * data e da API). Perto da meia-noite, o dia de Lisboa pode não ser o do
 * computador nem o de UTC.
 */
export function lisbonToday(now: Date = new Date()): string {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(now)
}

/** "2026-11-25", 7 → "2026-12-02" (dias do calendário, sem fusos pelo meio). */
export function addDays(day: string, n: number): string {
  const d = new Date(`${day}T12:00:00Z`)
  d.setUTCDate(d.getUTCDate() + n)
  return d.toISOString().slice(0, 10)
}
