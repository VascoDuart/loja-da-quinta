import type { StockOverview, StockOverviewLot, StockProduct, Unit } from '../api/generated/model'

// Uma consulta de stock de exemplo, para o ecrã de stock e o painel.

const lot = (
  id: string,
  productId: string,
  unit: Unit,
  left: number,
  expiresOn: string,
  status: StockOverviewLot['status'],
): StockOverviewLot => ({
  id,
  productId,
  unit,
  quantityReceivedMilli: 10_000,
  quantityLeftMilli: left,
  harvestedOn: '2026-11-20',
  expiresOn,
  createdAt: '2026-11-20T10:00:00Z',
  status,
})

const product = (productId: string, name: string, unit: Unit, extra: Partial<StockProduct> = {}): StockProduct => ({
  productId,
  name,
  unit,
  active: true,
  sellableMilli: 0,
  lowStockMilli: 0,
  low: false,
  lots: [],
  ...extra,
})

export const stockOverview: StockOverview = {
  today: '2026-11-25',
  expiringUntil: '2026-11-28',
  products: [
    product('p-laranja', 'Laranja do Algarve', 'kg', {
      sellableMilli: 2500,
      lowStockMilli: 5000,
      low: true,
      lots: [
        lot('l-velho', 'p-laranja', 'kg', 1500, '2026-11-24', 'fora_de_validade'),
        lot('l-breve', 'p-laranja', 'kg', 2500, '2026-11-27', 'expira_em_breve'),
      ],
    }),
    product('p-ovos', 'Ovos', 'duzia', {
      sellableMilli: 3000,
      lots: [lot('l-ovos', 'p-ovos', 'duzia', 3000, '2026-12-10', 'valido')],
    }),
    product('p-salsa', 'Salsa', 'molho', { lowStockMilli: 2000, low: true }),
  ],
}

export const calmOverview: StockOverview = {
  ...stockOverview,
  products: stockOverview.products.filter((p) => p.productId === 'p-ovos'),
}
