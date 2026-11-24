import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { describe, expect, it } from 'vitest'

import { server, signedInAs } from '../test/msw'
import { renderApp } from '../test/render'
import { backofficeNav, routes } from './routes'

function renderAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  renderApp(<RouterProvider router={router} />)
  return router
}

describe('rotas públicas', () => {
  it.each([
    ['/', 'Produtos da época'],
    ['/produtos/123', 'Produto não encontrado'],
    ['/entrar', 'Entrar no backoffice'],
  ])('%s mostra a página certa', async (path, text) => {
    renderAt(path)
    expect(await screen.findByRole('heading', { name: text })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Loja da Quinta' })).toBeInTheDocument()
  })
})

describe('rotas do backoffice', () => {
  it('sem sessão vai para o login, que depois volta à mesma página', async () => {
    const router = renderAt('/app/stock?lote=3')
    expect(await screen.findByRole('heading', { name: 'Entrar no backoffice' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/entrar')
    expect(router.state.location.search).toBe('?next=%2Fapp%2Fstock%3Flote%3D3')
  })

  it('/app redireciona para o painel', async () => {
    server.use(signedInAs('gestor'))
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
  ])('%s tem o título "%s" e o link ativo no menu (gestor)', async (path, title) => {
    server.use(signedInAs('gestor'))
    renderAt(path)
    expect(await screen.findByRole('heading', { name: title })).toBeInTheDocument()
    const nav = screen.getByRole('navigation', { name: 'Backoffice' })
    expect(nav.querySelector('[aria-current="page"]')).toHaveTextContent(title)
    await waitFor(() => expect(document.title).toBe(`${title} · Loja da Quinta`))
  })

  it('o funcionário só vê no menu as páginas dele', async () => {
    server.use(signedInAs('funcionario'))
    renderAt('/app/venda')
    const nav = await screen.findByRole('navigation', { name: 'Backoffice' })
    expect(
      within(nav)
        .getAllByRole('link')
        .map((l) => l.textContent),
    ).toEqual(['Painel', 'Venda', 'Stock'])
    expect(within(nav).getByText('Zé')).toBeInTheDocument()
    expect(within(nav).getByText('Funcionário')).toBeInTheDocument()
  })

  it('o funcionário numa página do gestor vê "Sem permissão"', async () => {
    server.use(signedInAs('funcionario'))
    renderAt('/app/relatorios')
    expect(await screen.findByRole('heading', { name: 'Sem permissão' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Relatórios' })).not.toBeInTheDocument()
  })

  it('terminar sessão leva ao login', async () => {
    server.use(signedInAs('gestor'))
    const router = renderAt('/app/painel')
    await userEvent.setup().click(await screen.findByRole('button', { name: 'Terminar sessão' }))
    expect(await screen.findByRole('heading', { name: 'Entrar no backoffice' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/entrar')
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
