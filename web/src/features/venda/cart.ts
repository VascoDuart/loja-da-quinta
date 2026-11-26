import type { Product, SaleInput, Unit } from '../../api/generated/model'
import { parseQuantityToMilli } from '../../shared/lib/format'

/** One cart line as typed: the quantity stays text until it is sent. */
export type CartLine = { productId: string; quantity: string }

export const isDecimal = (unit: Unit) => unit === 'kg' || unit === 'l'

/**
 * Price × quantity, rounded to the cent half up, as the server does (ADR-0009).
 * Only for showing the total while the clerk builds the cart: the server always
 * prices the sale again from the catalog.
 */
export function lineTotalCents(priceCents: number, quantityMilli: number): number {
  return Math.floor((priceCents * quantityMilli + 500) / 1000)
}

/** The quantity of a line in thousandths, or null if it is not valid yet. */
export function lineMilli(line: CartLine, product: Product): number | null {
  const milli = parseQuantityToMilli(line.quantity, product.unit)
  return milli !== null && milli > 0 ? milli : null
}

/** Total of the valid lines, in cents. */
export function cartTotalCents(lines: readonly CartLine[], products: ReadonlyMap<string, Product>): number {
  let total = 0
  for (const line of lines) {
    const product = products.get(line.productId)
    const milli = product && lineMilli(line, product)
    if (product && milli) total += lineTotalCents(product.priceCents, milli)
  }
  return total
}

/**
 * Builds what goes to POST /sales: only products and quantities (prices never
 * leave the browser). A string is the message to show when the cart is not
 * ready to be confirmed.
 */
export function buildSaleInput(
  lines: readonly CartLine[],
  products: ReadonlyMap<string, Product>,
  payment: SaleInput['payment'] | null,
): SaleInput | string {
  if (lines.length === 0) return 'Junta pelo menos um produto.'
  const items: SaleInput['items'] = []
  for (const line of lines) {
    const product = products.get(line.productId)
    if (!product) return 'Um dos produtos já não está disponível. Tira-o da venda.'
    const milli = lineMilli(line, product)
    if (milli === null) {
      return isDecimal(product.unit)
        ? `Indica a quantidade de ${product.name}: um número com até 3 casas decimais (por exemplo 1,5).`
        : `Indica a quantidade de ${product.name}: um número inteiro.`
    }
    items.push({ productId: product.id, quantityMilli: milli })
  }
  if (!payment) return 'Escolhe o pagamento: numerário ou MB Way.'
  return { payment, items }
}

/** Lower case without accents, so "maca" finds "Maçã". */
export function searchKey(text: string): string {
  return text
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .toLowerCase()
    .trim()
}
