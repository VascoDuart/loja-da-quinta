import { screen, waitFor } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { describe, expect, it } from 'vitest'

import { renderWithQuery } from '../test/render'
import { backofficeNav, routes } from './routes'

function renderAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  renderWithQuery(<RouterProvider router={router} />)
  return router
}

describe('rotas públicas', () => {
  it.each([
    ['/', 'Lista de produtos da época'],
    ['/produtos/123', 'Detalhe do produto'],
    ['/entrar', 'Início de sessão'],
  ])('%s mostra a página certa', async (path, text) => {
    renderAt(path)
    expect(await screen.findByText(new RegExp(text))).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Loja da Quinta' })).toBeInTheDocument()
  })
})

describe('rotas do backoffice', () => {
  it('/app redireciona para o painel', async () => {
    const router = renderAt('/app')
    expect(await screen.findByRole('heading', { name: 'Painel' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/app/painel')
  })

  it.each([
    ['/app/venda', 'Venda'],
    ['/app/stock', 'Stock'],
    ['/app/produtos', 'Produtos'],
    ['/app/relatorios', 'Relatórios'],
    ['/app/utilizadores', 'Utilizadores'],
    ['/app/auditoria', 'Auditoria'],
  ])('%s tem o título "%s" e o link ativo no menu', async (path, title) => {
    renderAt(path)
    expect(await screen.findByRole('heading', { name: title })).toBeInTheDocument()
    const nav = screen.getByRole('navigation', { name: 'Backoffice' })
    expect(nav.querySelector('[aria-current="page"]')).toHaveTextContent(title)
    await waitFor(() => expect(document.title).toBe(`${title} · Loja da Quinta`))
  })

  it('o menu tem as sete páginas, com os papéis certos', () => {
    expect(backofficeNav.map((n) => n.title)).toEqual([
      'Painel',
      'Venda',
      'Stock',
      'Produtos',
      'Relatórios',
      'Utilizadores',
      'Auditoria',
    ])
    const soGestor = backofficeNav.filter((n) => !n.roles.includes('funcionario')).map((n) => n.title)
    expect(soGestor).toEqual(['Produtos', 'Relatórios', 'Utilizadores', 'Auditoria'])
  })
})

it('um endereço desconhecido mostra a página 404', async () => {
  renderAt('/nao-existe')
  expect(await screen.findByRole('heading', { name: 'Página não encontrada' })).toBeInTheDocument()
})
