import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { beforeEach, expect, it } from 'vitest'

import type { Category, Product } from '../../api/generated/model'
import { routes } from '../../app/routes'
import { problem, server, signedInAs } from '../../test/msw'
import { renderApp } from '../../test/render'

const at = '2026-11-20T10:00:00Z'
const fruta: Category = { id: 'c-fruta', name: 'Fruta', productCount: 1, createdAt: at, updatedAt: at }
const laranja: Product = {
  id: 'p-laranja',
  categoryId: 'c-fruta',
  categoryName: 'Fruta',
  name: 'Laranja do Algarve',
  unit: 'kg',
  priceCents: 120,
  vatRate: 6,
  seasonMonths: [1, 2, 12],
  lowStockMilli: 5000,
  isPublic: true,
  active: true,
  image: { version: 'a'.repeat(64) },
  version: '1764000000000000',
  createdAt: at,
  updatedAt: at,
}

let posted: unknown
let ifMatch: string | null

beforeEach(() => {
  posted = undefined
  ifMatch = null
  server.use(
    signedInAs('gestor'),
    http.get('*/api/v1/categories', () => HttpResponse.json([fruta])),
    http.get('*/api/v1/products', () => HttpResponse.json([laranja])),
    http.post('*/api/v1/products', async ({ request }) => {
      posted = await request.json()
      return HttpResponse.json({ ...laranja, id: 'p-novo', ...(posted as object) }, { status: 201 })
    }),
    http.put('*/api/v1/products/:id', async ({ request }) => {
      ifMatch = request.headers.get('If-Match')
      posted = await request.json()
      return HttpResponse.json({ ...laranja, ...(posted as object), version: '2' })
    }),
  )
})

function renderPage() {
  renderApp(<RouterProvider router={createMemoryRouter(routes, { initialEntries: ['/app/produtos'] })} />)
}

const productList = () => screen.findByRole('list', { name: 'Produtos' })

it('mostra os produtos com preço por unidade, IVA, época e miniatura com a versão no URL', async () => {
  renderPage()
  const item = within(await productList()).getByRole('listitem')
  expect(item).toHaveTextContent('Laranja do Algarve')
  expect(item).toHaveTextContent('1,20 € / kg')
  expect(item).toHaveTextContent('IVA 6 %')
  expect(item).toHaveTextContent('dez, jan, fev')
  expect(item.querySelector('img')?.getAttribute('src')).toBe(
    `/api/v1/public/products/p-laranja/image?size=thumb&v=${'a'.repeat(64)}`,
  )
  expect(screen.getByText(/Fruta/, { selector: '.categories__name' })).toHaveTextContent('(1 produto)')
})

it('cria um produto convertendo o preço e o limiar para inteiros', async () => {
  const user = userEvent.setup()
  renderPage()
  await productList()
  await user.click(screen.getByRole('button', { name: 'Novo produto' }))
  await user.type(screen.getByLabelText('Nome'), 'Mel de rosmaninho')
  await user.selectOptions(screen.getByLabelText('Unidade de venda'), 'un')
  await user.type(screen.getByLabelText(/Preço por unidade/), '7,5')
  await user.click(screen.getByRole('radio', { name: '23 %' }))
  await user.click(screen.getByRole('button', { name: 'mai' }))
  await user.click(screen.getByRole('button', { name: 'abr' }))
  const low = screen.getByLabelText(/Stock baixo abaixo de/)
  await user.clear(low)
  await user.type(low, '4')
  await user.click(screen.getByRole('checkbox', { name: 'Visível no catálogo público' }))
  await user.click(screen.getByRole('button', { name: 'Criar produto' }))

  expect(await screen.findByRole('status')).toHaveTextContent('Produto Mel de rosmaninho criado.')
  expect(posted).toEqual({
    name: 'Mel de rosmaninho',
    categoryId: 'c-fruta',
    unit: 'un',
    priceCents: 750,
    vatRate: 23,
    seasonMonths: [4, 5],
    lowStockMilli: 4000,
    isPublic: false,
    active: true,
  })
})

it('não envia um preço ou um limiar inválidos', async () => {
  const user = userEvent.setup()
  renderPage()
  await productList()
  await user.click(screen.getByRole('button', { name: 'Novo produto' }))
  await user.type(screen.getByLabelText('Nome'), 'Ovos')
  await user.type(screen.getByLabelText(/Preço por kg/), '1,234')
  await user.click(screen.getByRole('button', { name: 'Criar produto' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('O preço tem de ser um valor em euros')

  await user.selectOptions(screen.getByLabelText('Unidade de venda'), 'duzia')
  const price = screen.getByLabelText(/Preço por dúzia/)
  await user.clear(price)
  await user.type(price, '2,40')
  const low = screen.getByLabelText(/Stock baixo abaixo de/)
  await user.clear(low)
  await user.type(low, '0,5')
  await user.click(screen.getByRole('button', { name: 'Criar produto' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('tem de ser um número inteiro')
  expect(posted).toBeUndefined()
})

it('editar envia o If-Match com a versão lida', async () => {
  const user = userEvent.setup()
  renderPage()
  await productList()
  await user.click(screen.getByRole('button', { name: 'Editar Laranja do Algarve' }))
  const price = screen.getByLabelText(/Preço por kg/)
  expect(price).toHaveValue('1,20')
  await user.clear(price)
  await user.type(price, '1,50')
  await user.click(screen.getByRole('button', { name: 'Guardar' }))

  expect(await screen.findByRole('status')).toHaveTextContent('Alterações a Laranja do Algarve guardadas.')
  expect(ifMatch).toBe('"1764000000000000"')
  expect(posted).toMatchObject({ priceCents: 150, seasonMonths: [1, 2, 12], lowStockMilli: 5000 })
})

it('se outra pessoa gravou entretanto, mostra o aviso da API', async () => {
  server.use(
    http.put('*/api/v1/products/:id', () =>
      problem(412, 'Outra pessoa alterou este produto entretanto. Recarrega para veres a versão atual.'),
    ),
  )
  const user = userEvent.setup()
  renderPage()
  await productList()
  await user.click(screen.getByRole('button', { name: 'Editar Laranja do Algarve' }))
  await user.clear(screen.getByLabelText(/Preço por kg/))
  await user.type(screen.getByLabelText(/Preço por kg/), '1,99')
  await user.click(screen.getByRole('button', { name: 'Guardar' }))
  // O aviso fica à vista, com o que a pessoa escreveu, mesmo depois de a
  // lista ser recarregada.
  expect(await screen.findByRole('alert')).toHaveTextContent('Outra pessoa alterou este produto')
  expect(screen.getByLabelText(/Preço por kg/)).toHaveValue('1,99')

  // "Carregar a versão atual" volta a abrir o formulário com os dados da API.
  await user.click(screen.getByRole('button', { name: 'Carregar a versão atual' }))
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  expect(screen.getByLabelText(/Preço por kg/)).toHaveValue('1,20')
})

it('uma categoria com produtos não mostra o botão de apagar', async () => {
  server.use(
    http.get('*/api/v1/categories', () =>
      HttpResponse.json([fruta, { ...fruta, id: 'c-vazia', name: 'Vazia', productCount: 0 }]),
    ),
  )
  renderPage()
  await productList()
  expect(screen.queryByRole('button', { name: 'Apagar Fruta' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Apagar Vazia' })).toBeInTheDocument()
})
