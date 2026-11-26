import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { beforeEach, expect, it } from 'vitest'

import type { Product, Sale, SaleInput, StockOverview } from '../../api/generated/model'
import { routes } from '../../app/routes'
import { problem, server, signedInAs } from '../../test/msw'
import { renderApp } from '../../test/render'
import { buildSaleInput, lineTotalCents, searchKey } from './cart'

const at = '2026-11-20T10:00:00Z'
const tomate: Product = {
  id: 'p-tomate',
  categoryId: 'c-horta',
  categoryName: 'Horta',
  name: 'Tomate',
  unit: 'kg',
  priceCents: 349,
  vatRate: 6,
  seasonMonths: [],
  lowStockMilli: 0,
  isPublic: true,
  active: true,
  version: '1',
  createdAt: at,
  updatedAt: at,
}
const ovos: Product = { ...tomate, id: 'p-ovos', name: 'Ovos', unit: 'duzia', priceCents: 280 }
const maca: Product = { ...tomate, id: 'p-maca', name: 'Maçã', priceCents: 150 }

const overview: StockOverview = {
  today: '2026-11-20',
  expiringUntil: '2026-11-23',
  products: [tomate, ovos, maca].map((p) => ({
    productId: p.id,
    name: p.name,
    unit: p.unit,
    active: true,
    sellableMilli: p.id === 'p-maca' ? 0 : 10_000,
    lowStockMilli: 0,
    low: false,
    lots: [],
  })),
}

type Posted = { body: SaleInput; key: string | null }
let posted: Posted[]
let reply: () => Response

function lastBody(): SaleInput {
  const last = posted.at(-1)
  if (!last) throw new Error('nothing was posted')
  return last.body
}

function sale(body: SaleInput): Sale {
  return {
    id: 's-1',
    number: 'V-000042',
    payment: body.payment,
    status: 'concluida',
    totalCents: 1153,
    vatCents: 65,
    createdAt: at,
    items: [
      {
        productId: 'p-tomate',
        productName: 'Tomate',
        unit: 'kg',
        quantityMilli: 2500,
        unitPriceCents: 349,
        vatRate: 6,
        lineTotalCents: 873,
        vatCents: 49,
      },
    ],
  }
}

beforeEach(() => {
  posted = []
  reply = () => HttpResponse.json(sale(lastBody()), { status: 201 })
  server.use(
    signedInAs('funcionario'),
    http.get('*/api/v1/products', () => HttpResponse.json([tomate, ovos, maca])),
    http.get('*/api/v1/stock', () => HttpResponse.json(overview)),
    http.post('*/api/v1/sales', async ({ request }) => {
      posted.push({ body: (await request.json()) as SaleInput, key: request.headers.get('Idempotency-Key') })
      return reply()
    }),
  )
})

function renderPage() {
  renderApp(<RouterProvider router={createMemoryRouter(routes, { initialEntries: ['/app/venda'] })} />)
}

async function buildCart(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: /^Tomate.*Juntar$/ }))
  await user.type(screen.getByLabelText('Tomate (kg)'), '2,5')
  await user.click(screen.getByRole('button', { name: /^Ovos.*Juntar$/ }))
  await user.click(screen.getByLabelText('Numerário'))
}

it('sends only products and quantities, with an idempotency key, and shows the record', async () => {
  const user = userEvent.setup()
  renderPage()
  await buildCart(user)
  // Preview: 2,5 kg × 3,49 € = 8,73 € (rounded per line) + 2,80 €.
  expect(screen.getByTestId('venda-total')).toHaveTextContent('11,53')

  await user.click(screen.getByRole('button', { name: 'Confirmar registo de venda' }))

  expect(await screen.findByRole('heading', { name: 'Venda V-000042 registada' })).toBeInTheDocument()
  expect(posted).toHaveLength(1)
  expect(posted[0]?.body).toEqual({
    payment: 'numerario',
    items: [
      { productId: 'p-tomate', quantityMilli: 2500 },
      { productId: 'p-ovos', quantityMilli: 1000 },
    ],
  })
  expect(posted[0]?.key).toMatch(/^[A-Za-z0-9_-]{16,64}$/)
})

it('on a shortage keeps the cart; the same cart reuses the key, a changed one gets a new key', async () => {
  reply = () => problem(409, 'Não há stock que chegue de Tomate: só há 2 kg à venda.')
  const user = userEvent.setup()
  renderPage()
  await buildCart(user)
  const confirm = screen.getByRole('button', { name: 'Confirmar registo de venda' })

  await user.click(confirm)
  expect(await screen.findByRole('alert')).toHaveTextContent('só há 2 kg')
  expect(screen.getByLabelText('Tomate (kg)')).toHaveValue('2,5')

  await user.click(confirm) // a retry of the same cart
  const field = screen.getByLabelText('Tomate (kg)')
  await user.clear(field)
  await user.type(field, '2')
  reply = () => HttpResponse.json(sale(lastBody()), { status: 201 })
  await user.click(confirm)

  expect(await screen.findByRole('heading', { name: /registada/ })).toBeInTheDocument()
  expect(posted).toHaveLength(3)
  expect(posted[1]?.key).toBe(posted[0]?.key)
  expect(posted[2]?.key).not.toBe(posted[0]?.key)
  expect(posted[2]?.body.items[0]).toEqual({ productId: 'p-tomate', quantityMilli: 2000 })
})

it('a sold-out product cannot be added and the search ignores accents', async () => {
  const user = userEvent.setup()
  renderPage()
  await user.type(await screen.findByLabelText('Pesquisar produto'), 'maca')
  const results = screen.getByRole('list', { name: 'Resultados da pesquisa' })
  const apple = within(results).getByRole('button', { name: /^Maçã/ })
  expect(apple).toBeDisabled()
  expect(within(results).queryByRole('button', { name: /^Tomate.*Juntar$/ })).not.toBeInTheDocument()
})

it('rounds a line half up and refuses carts that are not ready', () => {
  expect(lineTotalCents(1, 500)).toBe(1)
  expect(lineTotalCents(1, 499)).toBe(0)
  expect(lineTotalCents(349, 2500)).toBe(873)
  const products = new Map([
    [tomate.id, tomate],
    [ovos.id, ovos],
  ])
  expect(buildSaleInput([{ productId: 'p-ovos', quantity: '1,5' }], products, 'mbway')).toMatch(/número inteiro/)
  expect(buildSaleInput([{ productId: 'p-tomate', quantity: '0' }], products, 'mbway')).toMatch(/quantidade de Tomate/)
  expect(buildSaleInput([{ productId: 'p-tomate', quantity: '1' }], products, null)).toMatch(/pagamento/)
  expect(searchKey('  Maçã ')).toBe('maca')
})
