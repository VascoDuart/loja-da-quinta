import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { expect, it } from 'vitest'

import type { CatalogItem } from '../../api/generated/model'
import { routes } from '../../app/routes'
import { problem, server } from '../../test/msw'
import { renderApp } from '../../test/render'

const version = 'a'.repeat(64)
const todoOAno = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]

const laranja: CatalogItem = {
  id: 'p-laranja',
  name: 'Laranja do Algarve',
  categoryName: 'Fruta',
  unit: 'kg',
  priceCents: 120,
  seasonMonths: [12, 1, 2],
  image: { version },
  availability: 'disponivel',
}
const alface: CatalogItem = {
  id: 'p-alface',
  name: 'Alface',
  categoryName: 'Hortícolas',
  unit: 'un',
  priceCents: 90,
  seasonMonths: todoOAno,
  availability: 'esgotado',
}
const ovos: CatalogItem = {
  id: 'p-ovos',
  name: 'Ovos',
  categoryName: 'Hortícolas',
  unit: 'duzia',
  priceCents: 350,
  seasonMonths: [],
  availability: 'pouco',
}

function catalog(items: CatalogItem[]) {
  server.use(http.get('*/api/v1/public/catalog', () => HttpResponse.json(items)))
}

function renderAt(path: string) {
  renderApp(<RouterProvider router={createMemoryRouter(routes, { initialEntries: [path] })} />)
}

it('mostra os produtos com preço, época, disponibilidade e miniatura com versão', async () => {
  catalog([laranja, alface, ovos])
  renderAt('/')

  expect(await screen.findByRole('heading', { name: 'Produtos da época', level: 1 })).toBeInTheDocument()
  const list = await screen.findByRole('list', { name: 'Produtos' })
  const cards = within(list).getAllByRole('listitem')
  expect(cards).toHaveLength(3)

  const card = cards[0] as HTMLElement
  expect(card).toHaveTextContent('Laranja do Algarve')
  expect(card).toHaveTextContent('1,20 € / kg')
  expect(card).toHaveTextContent('Época: dez, jan, fev')
  expect(card).toHaveTextContent('Disponível')
  expect(within(card).getByRole('link')).toHaveAttribute('href', '/produtos/p-laranja')
  expect(card.querySelector('img')).toHaveAttribute(
    'src',
    `/api/v1/public/products/p-laranja/image?size=thumb&v=${version}`,
  )

  expect(cards[1]).toHaveTextContent('Esgotado')
  expect(cards[2]).toHaveTextContent('Pouco')
  expect(cards[2]).toHaveTextContent('Época: Todo o ano')
  expect(cards[1]?.querySelector('img')).toBeNull()
})

it('filtra por categoria com os Chip e volta a Todas', async () => {
  catalog([laranja, alface, ovos])
  const user = userEvent.setup()
  renderAt('/')
  await screen.findByRole('list', { name: 'Produtos' })

  const todas = screen.getByRole('button', { name: 'Todas' })
  expect(todas).toHaveAttribute('aria-pressed', 'true')

  await user.click(screen.getByRole('button', { name: 'Hortícolas' }))
  expect(screen.getByRole('button', { name: 'Hortícolas' })).toHaveAttribute('aria-pressed', 'true')
  const cards = within(screen.getByRole('list', { name: 'Produtos' })).getAllByRole('listitem')
  expect(cards).toHaveLength(2)
  expect(cards[0]).toHaveTextContent('Alface')
  expect(cards[1]).toHaveTextContent('Ovos')

  await user.click(todas)
  expect(within(screen.getByRole('list', { name: 'Produtos' })).getAllByRole('listitem')).toHaveLength(3)
})

it('a dica da época destaca o que está na época este mês', async () => {
  catalog([laranja, alface, ovos])
  renderAt('/')
  const tip = await screen.findByRole('complementary', { name: 'Dica da época' })
  // A alface é de todo o ano; os ovos não têm época; a laranja só de dez a fev.
  expect(tip).toHaveTextContent('Alface')
  expect(tip).not.toHaveTextContent('Ovos')
})

it('sem produtos, diz que o catálogo está vazio', async () => {
  catalog([])
  renderAt('/')
  expect(await screen.findByText('Ainda não há produtos no catálogo.')).toBeInTheDocument()
  expect(screen.queryByRole('group', { name: 'Filtrar por categoria' })).not.toBeInTheDocument()
})

it('com a API em baixo, mostra o erro em português', async () => {
  server.use(http.get('*/api/v1/public/catalog', () => problem(503, 'O servidor não está disponível.')))
  renderAt('/')
  expect(await screen.findByRole('alert')).toHaveTextContent('O servidor teve um problema.')
})

it('o detalhe mostra o produto, a imagem grande e a época', async () => {
  catalog([laranja, alface])
  renderAt('/produtos/p-laranja')
  expect(await screen.findByRole('heading', { name: 'Laranja do Algarve', level: 1 })).toBeInTheDocument()
  const article = screen.getByRole('article')
  expect(article).toHaveTextContent('Fruta')
  expect(article).toHaveTextContent('1,20 € / kg')
  expect(article).toHaveTextContent('Disponível')
  expect(article).toHaveTextContent('dez, jan, fev')
  expect(article.querySelector('img')).toHaveAttribute(
    'src',
    `/api/v1/public/products/p-laranja/image?size=large&v=${version}`,
  )
  expect(screen.getByRole('link', { name: /Voltar ao catálogo/ })).toHaveAttribute('href', '/')
})

it('um produto que não existe (ou já não é público) diz isso', async () => {
  catalog([alface])
  renderAt('/produtos/nao-existe')
  expect(await screen.findByRole('heading', { name: 'Produto não encontrado' })).toBeInTheDocument()
})
