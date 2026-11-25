import { fireEvent, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { delay, http, HttpResponse } from 'msw'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { beforeEach, expect, it } from 'vitest'

import type { Product, StockLotInput } from '../../api/generated/model'
import { routes } from '../../app/routes'
import { addDays, lisbonToday } from '../../shared/lib/format'
import { problem, server, signedInAs } from '../../test/msw'
import { renderApp } from '../../test/render'

const at = '2026-11-20T10:00:00Z'
const base: Product = {
  id: 'p-laranja',
  categoryId: 'c-fruta',
  categoryName: 'Fruta',
  name: 'Laranja do Algarve',
  unit: 'kg',
  priceCents: 120,
  vatRate: 6,
  seasonMonths: [],
  lowStockMilli: 5000,
  isPublic: true,
  active: true,
  version: '1',
  createdAt: at,
  updatedAt: at,
}
const laranja = base
const ovos: Product = { ...base, id: 'p-ovos', name: 'Ovos', unit: 'duzia', lowStockMilli: 0 }
const marmelo: Product = { ...base, id: 'p-marmelo', name: 'Marmelo', active: false }

const today = lisbonToday()
let posted: StockLotInput[]

beforeEach(() => {
  posted = []
  server.use(
    signedInAs('funcionario'),
    http.get('*/api/v1/products', () => HttpResponse.json([laranja, marmelo, ovos])),
    http.post('*/api/v1/stock/lots', async ({ request }) => {
      const body = (await request.json()) as StockLotInput
      posted.push(body)
      await delay(50)
      return HttpResponse.json(
        {
          id: `lote-${posted.length}`,
          productId: body.productId,
          quantityReceivedMilli: body.quantityMilli,
          quantityLeftMilli: body.quantityMilli,
          harvestedOn: body.harvestedOn,
          expiresOn: body.expiresOn,
          createdAt: at,
        },
        { status: 201 },
      )
    }),
  )
})

function renderPage() {
  renderApp(<RouterProvider router={createMemoryRouter(routes, { initialEntries: ['/app/stock'] })} />)
}

const setDate = (label: string, value: string) => fireEvent.change(screen.getByLabelText(label), { target: { value } })

async function fill(user: ReturnType<typeof userEvent.setup>, quantity: string, expires: string, product?: string) {
  const select = await screen.findByLabelText('Produto')
  if (product) await user.selectOptions(select, product)
  const field = screen.getByLabelText(/^Quantidade/)
  await user.clear(field)
  await user.type(field, quantity)
  setDate('Validade', expires)
}

it('o funcionário vê só os produtos ativos e a colheita começa em hoje', async () => {
  renderPage()
  const select = await screen.findByLabelText('Produto')
  const options = within(select)
    .getAllByRole('option')
    .map((o) => o.textContent)
  expect(options).toEqual(['Laranja do Algarve', 'Ovos'])
  expect(screen.getByLabelText('Data de colheita')).toHaveValue(today)
  expect(screen.getByLabelText('Data de colheita')).toHaveAttribute('max', today)
  expect(screen.getByLabelText('Quantidade (kg)')).toBeInTheDocument()
})

it('regista um lote em milésimos e mostra o que ficou registado', async () => {
  const user = userEvent.setup()
  renderPage()
  const expires = addDays(today, 7)
  await fill(user, '12,5', expires)
  await user.click(screen.getByRole('button', { name: 'Registar entrada' }))

  expect(await screen.findByRole('status')).toHaveTextContent(/Entrada registada: 12,500 kg de Laranja do Algarve/)
  expect(posted).toEqual([{ productId: 'p-laranja', quantityMilli: 12_500, harvestedOn: today, expiresOn: expires }])
  const list = screen.getByRole('list', { name: 'Entradas registadas' })
  expect(within(list).getAllByRole('listitem')).toHaveLength(1)
  // A quantidade e a validade ficam vazias para a entrada seguinte.
  expect(screen.getByLabelText('Quantidade (kg)')).toHaveValue('')
  expect(screen.getByLabelText('Validade')).toHaveValue('')
})

it('um duplo toque no botão regista o lote uma só vez', async () => {
  const user = userEvent.setup()
  renderPage()
  await fill(user, '3', addDays(today, 3))
  await user.dblClick(screen.getByRole('button', { name: 'Registar entrada' }))
  // Enter no campo enquanto o pedido está em curso também não envia de novo.
  fireEvent.submit(screen.getByLabelText('Quantidade (kg)').closest('form') as HTMLFormElement)

  expect(await screen.findByRole('status')).toHaveTextContent('Entrada registada')
  expect(posted).toHaveLength(1)
})

it('dúzias só aceitam números inteiros', async () => {
  const user = userEvent.setup()
  renderPage()
  await fill(user, '1,5', addDays(today, 3), 'Ovos')
  expect(screen.getByLabelText('Quantidade (dúzias)')).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Registar entrada' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('A quantidade tem de ser um número inteiro.')
  expect(posted).toHaveLength(0)
})

it('não envia datas incoerentes nem quantidade 0', async () => {
  const user = userEvent.setup()
  renderPage()
  const cases: [string, () => void, string][] = [
    ['0', () => setDate('Validade', addDays(today, 3)), 'maior do que 0'],
    ['2', () => setDate('Validade', ''), 'Indica a validade.'],
    [
      '2',
      () => {
        setDate('Data de colheita', addDays(today, 1))
        setDate('Validade', addDays(today, 3))
      },
      'não pode ser no futuro',
    ],
    [
      '2',
      () => {
        setDate('Data de colheita', addDays(today, -2))
        setDate('Validade', addDays(today, -3))
      },
      'anterior à data de colheita',
    ],
    [
      '2',
      () => {
        setDate('Data de colheita', addDays(today, -5))
        setDate('Validade', addDays(today, -1))
      },
      'A validade já passou',
    ],
  ]
  await screen.findByLabelText('Produto')
  for (const [quantity, dates, message] of cases) {
    const field = screen.getByLabelText(/^Quantidade/)
    await user.clear(field)
    await user.type(field, quantity)
    dates()
    await user.click(screen.getByRole('button', { name: 'Registar entrada' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(message)
  }
  expect(posted).toHaveLength(0)
})

it('mostra a mensagem da API quando o lote é recusado', async () => {
  server.use(
    http.post('*/api/v1/stock/lots', () =>
      problem(422, 'Este produto está desativado: não se pode registar stock dele.'),
    ),
  )
  const user = userEvent.setup()
  renderPage()
  await fill(user, '2', addDays(today, 3))
  await user.click(screen.getByRole('button', { name: 'Registar entrada' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('Este produto está desativado')
  // O que a pessoa escreveu fica, para poder corrigir.
  expect(screen.getByLabelText('Quantidade (kg)')).toHaveValue('2')
})

it('sem produtos ativos, explica que o gestor os tem de criar', async () => {
  server.use(http.get('*/api/v1/products', () => HttpResponse.json([marmelo])))
  renderPage()
  expect(await screen.findByText(/Ainda não há produtos ativos/)).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Registar entrada' })).not.toBeInTheDocument()
})
