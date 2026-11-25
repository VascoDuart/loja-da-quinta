import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { delay, http, HttpResponse } from 'msw'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { beforeEach, expect, it } from 'vitest'

import type { Product, StockAdjustmentInput, StockLot } from '../../api/generated/model'
import { routes } from '../../app/routes'
import { addDays, lisbonToday } from '../../shared/lib/format'
import { problem, server, signedInAs } from '../../test/msw'
import { renderApp } from '../../test/render'

const at = '2026-11-20T10:00:00Z'
const today = lisbonToday()
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
const ovos: Product = { ...base, id: 'p-ovos', name: 'Ovos', unit: 'duzia', lowStockMilli: 0, active: false }

const lot = (id: string, productId: string, unit: Product['unit'], left: number, expiresIn: number): StockLot => ({
  id,
  productId,
  unit,
  quantityReceivedMilli: 10_000,
  quantityLeftMilli: left,
  harvestedOn: addDays(today, -10),
  expiresOn: addDays(today, expiresIn),
  createdAt: at,
})

let lots: Record<string, StockLot[]>
let posted: StockAdjustmentInput[]
let lotReads: number

beforeEach(() => {
  lots = {
    'p-laranja': [lot('l-velho', 'p-laranja', 'kg', 2000, -1), lot('l-novo', 'p-laranja', 'kg', 8500, 5)],
    'p-ovos': [lot('l-ovos', 'p-ovos', 'duzia', 3000, 2)],
  }
  posted = []
  lotReads = 0
  server.use(
    signedInAs('gestor'),
    http.get('*/api/v1/products', () => HttpResponse.json([base, ovos])),
    http.get('*/api/v1/stock/lots', ({ request }) => {
      lotReads++
      return HttpResponse.json(lots[new URL(request.url).searchParams.get('productId') ?? ''] ?? [])
    }),
    http.post('*/api/v1/stock/adjustments', async ({ request }) => {
      const body = (await request.json()) as StockAdjustmentInput
      posted.push(body)
      await delay(30)
      const target = Object.values(lots)
        .flat()
        .find((l) => l.id === body.lotId)
      if (!target) return problem(404, 'Este lote não existe.')
      const updated = { ...target, quantityLeftMilli: target.quantityLeftMilli + body.quantityMilli }
      lots[target.productId] = (lots[target.productId] ?? []).map((l) => (l.id === target.id ? updated : l))
      return HttpResponse.json(
        {
          movement: {
            id: posted.length,
            lotId: body.lotId,
            type: body.type,
            quantityMilli: body.quantityMilli,
            reason: body.reason,
            createdAt: at,
          },
          lot: updated,
        },
        { status: 201 },
      )
    }),
  )
})

function renderAt(path = '/app/stock/ajustes') {
  renderApp(<RouterProvider router={createMemoryRouter(routes, { initialEntries: [path] })} />)
}

const lotRadio = (name: RegExp) => screen.findByRole('radio', { name })

/** Escreve num campo de uma vez (colar), como quem escreve depressa. */
async function fill(user: ReturnType<typeof userEvent.setup>, field: HTMLElement, text: string) {
  await user.clear(field)
  await user.click(field)
  await user.paste(text)
}

it('o gestor regista uma quebra no lote escolhido: quantidade negativa, motivo sem espaços nas pontas', async () => {
  const user = userEvent.setup()
  renderAt()
  expect(await screen.findByRole('heading', { name: 'Ajustes e quebras', level: 1 })).toBeInTheDocument()
  // Os lotes vêm pela ordem FEFO; o fora de validade aparece marcado.
  expect(await lotRadio(/fora de validade · restam 2,000 kg/)).toBeChecked()

  await user.click(await lotRadio(/restam 8,500 kg/))
  await fill(user, screen.getByLabelText(/^Quantidade/), '1,5')
  await fill(user, screen.getByLabelText('Motivo'), '  Caixa caiu  ')
  await user.click(screen.getByRole('button', { name: 'Registar quebra' }))

  expect(await screen.findByRole('status')).toHaveTextContent(
    'Quebra registada: 1,500 kg de Laranja do Algarve. O lote fica com 7,000 kg.',
  )
  expect(posted).toEqual([{ lotId: 'l-novo', type: 'quebra', quantityMilli: -1500, reason: 'Caixa caiu' }])
  // A lista volta a ser lida: o lote mostra o que resta agora.
  expect(await lotRadio(/restam 7,000 kg/)).toBeInTheDocument()
  expect(screen.getByLabelText(/^Quantidade/)).toHaveValue('')
  expect(screen.getByLabelText('Motivo')).toHaveValue('')
})

it('um ajuste para mais envia a quantidade positiva', async () => {
  const user = userEvent.setup()
  renderAt()
  await lotRadio(/restam 2,000 kg/)
  await user.click(screen.getByRole('radio', { name: 'Ajuste para mais' }))
  await fill(user, screen.getByLabelText(/^Quantidade/), '0,25')
  await fill(user, screen.getByLabelText('Motivo'), 'Contagem de inventário')
  await user.click(screen.getByRole('button', { name: 'Registar ajuste' }))
  expect(await screen.findByRole('status')).toHaveTextContent('Ajuste registado: mais 0,250 kg')
  expect(posted[0]).toMatchObject({ lotId: 'l-velho', type: 'ajuste', quantityMilli: 250 })
})

it('as regras são verificadas antes de enviar: o que resta e o motivo', async () => {
  const user = userEvent.setup()
  renderAt()
  await lotRadio(/restam 2,000 kg/)
  const quantity = screen.getByLabelText(/^Quantidade/)
  const send = screen.getByRole('button', { name: 'Registar quebra' })

  await fill(user, quantity, '2,001')
  await fill(user, screen.getByLabelText('Motivo'), 'Estragou')
  await user.click(send)
  expect(await screen.findByRole('alert')).toHaveTextContent('Este lote só tem 2,000 kg')

  await user.clear(quantity)
  await fill(user, quantity, '1')
  await user.clear(screen.getByLabelText('Motivo'))
  await fill(user, screen.getByLabelText('Motivo'), ' ')
  await user.click(send)
  expect(await screen.findByRole('alert')).toHaveTextContent('Indica o motivo.')
  expect(posted).toEqual([])
})

it('um produto desativado ainda se corrige, mas dúzias só inteiras', async () => {
  const user = userEvent.setup()
  renderAt()
  await lotRadio(/restam 2,000 kg/)
  await user.selectOptions(screen.getByLabelText('Produto'), 'Ovos (desativado)')
  expect(await lotRadio(/restam 3 dúzias/)).toBeChecked()
  await fill(user, screen.getByLabelText('Quantidade (dúzias)'), '1,5')
  await fill(user, screen.getByLabelText('Motivo'), 'Partiram-se')
  await user.click(screen.getByRole('button', { name: 'Registar quebra' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('tem de ser um número inteiro')
  expect(posted).toEqual([])
})

it('se outra pessoa mexeu no lote entretanto (409), mostra a mensagem da API e relê os lotes', async () => {
  server.use(
    http.post('*/api/v1/stock/adjustments', () => {
      lots['p-laranja'] = [lot('l-velho', 'p-laranja', 'kg', 500, -1), lot('l-novo', 'p-laranja', 'kg', 8500, 5)]
      return problem(409, 'Este lote só tem 0,5 kg: não se pode tirar mais do que isso.')
    }),
  )
  const user = userEvent.setup()
  renderAt()
  await lotRadio(/restam 2,000 kg/)
  const reads = lotReads
  await fill(user, screen.getByLabelText(/^Quantidade/), '2')
  await fill(user, screen.getByLabelText('Motivo'), 'Estragou')
  await user.click(screen.getByRole('button', { name: 'Registar quebra' }))

  expect(await screen.findByRole('alert')).toHaveTextContent('Este lote só tem 0,5 kg')
  expect(await lotRadio(/restam 0,500 kg/)).toBeInTheDocument()
  expect(lotReads).toBeGreaterThan(reads)
  // O que a pessoa escreveu fica, para corrigir.
  expect(screen.getByLabelText('Motivo')).toHaveValue('Estragou')
})

it('um produto sem lotes diz que não há nada para ajustar', async () => {
  lots['p-laranja'] = []
  renderAt()
  expect(await screen.findByText(/não tem lotes com stock nem dentro da validade/)).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /Registar/ })).not.toBeInTheDocument()
})

it('o funcionário não tem acesso aos ajustes', async () => {
  server.use(signedInAs('funcionario'))
  renderAt()
  expect(await screen.findByRole('heading', { name: 'Sem permissão' })).toBeInTheDocument()
})

it('o funcionário não vê o link dos ajustes no ecrã de stock', async () => {
  server.use(signedInAs('funcionario'))
  renderAt('/app/stock')
  expect(await screen.findByRole('button', { name: 'Registar entrada' })).toBeInTheDocument()
  expect(screen.queryByRole('link', { name: 'Registar um ajuste ou uma quebra' })).not.toBeInTheDocument()
})

it('o gestor chega aos ajustes pelo ecrã de stock', async () => {
  const user = userEvent.setup()
  renderAt('/app/stock')
  await user.click(await screen.findByRole('link', { name: 'Registar um ajuste ou uma quebra' }))
  expect(await screen.findByRole('heading', { name: 'Ajustes e quebras', level: 1 })).toBeInTheDocument()
})
