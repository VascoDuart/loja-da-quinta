import type { StockOverview, StockOverviewLot, StockProduct, Unit } from '../../api/generated/model'
import { formatQuantity } from '../../shared/lib/format'

/** Milésimos da unidade → "12,500 kg", "3 dúzias" (ADR-0009). */
export const milli = (value: number, unit: Unit) => formatQuantity(value / 1000, unit)

export type ProductLot = { product: StockProduct; lot: StockOverviewLot }

/**
 * O que pede atenção no stock: o que a consulta e o painel destacam. Os
 * lotes vêm pela validade, o mais urgente primeiro. Os estados são os da API,
 * que sabe que dia é em Lisboa.
 */
export function summarize(overview: StockOverview) {
  const lots = (status: StockOverviewLot['status']): ProductLot[] =>
    overview.products
      .flatMap((product) => product.lots.filter((l) => l.status === status).map((lot) => ({ product, lot })))
      .sort((a, b) => a.lot.expiresOn.localeCompare(b.lot.expiresOn))

  return {
    low: overview.products.filter((p) => p.low),
    expiring: lots('expira_em_breve'),
    expired: lots('fora_de_validade'),
  }
}

/** "Laranja, Ovos e mais 2": nomes sem repetições, no máximo `max`. */
export function names(products: readonly StockProduct[], max = 4): string {
  const unique = [...new Set(products.map((p) => p.name))]
  const shown = unique.slice(0, max).join(', ')
  return unique.length > max ? `${shown} e mais ${unique.length - max}` : shown
}
