import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { beforeEach, expect, it } from 'vitest'

import type { ManagedUser } from '../../api/generated/model'
import { routes } from '../../app/routes'
import { problem, server, signedInAs, testUsers } from '../../test/msw'
import { renderApp } from '../../test/render'

const at = '2026-11-20T10:00:00Z'
let users: ManagedUser[]
let lastBody: unknown

beforeEach(() => {
  users = [
    { ...testUsers.gestor, active: true, createdAt: at, updatedAt: at },
    { ...testUsers.funcionario, active: true, createdAt: at, updatedAt: at },
  ]
  lastBody = undefined
  server.use(
    signedInAs('gestor'),
    http.get('*/api/v1/users', () => HttpResponse.json(users)),
    http.post('*/api/v1/users', async ({ request }) => {
      const body = (await request.json()) as { email: string; name: string; role: 'gestor' | 'funcionario' }
      lastBody = body
      const created: ManagedUser = { id: 'novo', ...body, active: true, createdAt: at, updatedAt: at }
      users = [...users, created]
      return HttpResponse.json(created, { status: 201 })
    }),
    http.patch('*/api/v1/users/:id', async ({ request, params }) => {
      lastBody = await request.json()
      const changes = lastBody as Partial<ManagedUser>
      users = users.map((u) => (u.id === params.id ? { ...u, ...changes } : u))
      return HttpResponse.json(users.find((u) => u.id === params.id))
    }),
  )
})

function renderPage() {
  renderApp(<RouterProvider router={createMemoryRouter(routes, { initialEntries: ['/app/utilizadores'] })} />)
}

const list = () => screen.findByRole('list', { name: 'Utilizadores' })

it('lista os utilizadores com papel, estado e a indicação da própria conta', async () => {
  renderPage()
  const items = within(await list()).getAllByRole('listitem')
  expect(items).toHaveLength(2)
  expect(items[0]).toHaveTextContent('Maria (tu)')
  expect(items[0]).toHaveTextContent('Gestor')
  expect(items[1]).toHaveTextContent('Funcionário')
  expect(items[1]).toHaveTextContent('Ativo')
})

it('cria um utilizador e mostra-o na lista', async () => {
  const user = userEvent.setup()
  renderPage()
  await list()
  await user.click(screen.getByRole('button', { name: 'Novo utilizador' }))
  await user.type(screen.getByLabelText('E-mail'), 'ana@quinta.pt')
  await user.type(screen.getByLabelText('Nome'), 'Ana')
  await user.click(screen.getByRole('radio', { name: 'Gestor' }))
  await user.type(screen.getByLabelText('Palavra-passe'), 'uma-frase-comprida')
  await user.click(screen.getByRole('button', { name: 'Criar utilizador' }))

  expect(await screen.findByRole('status')).toHaveTextContent('Utilizador ana@quinta.pt criado.')
  expect(lastBody).toEqual({ email: 'ana@quinta.pt', name: 'Ana', role: 'gestor', password: 'uma-frase-comprida' })
  expect(within(await list()).getAllByRole('listitem')).toHaveLength(3)
})

it('desativar envia só o que mudou', async () => {
  const user = userEvent.setup()
  renderPage()
  await list()
  await user.click(screen.getByRole('button', { name: 'Editar Zé' }))
  await user.click(screen.getByRole('checkbox', { name: 'Conta ativa' }))
  await user.click(screen.getByRole('button', { name: 'Guardar' }))

  expect(await screen.findByRole('status')).toHaveTextContent('Alterações a Zé guardadas.')
  expect(lastBody).toEqual({ active: false })
  expect(within(await list()).getAllByRole('listitem')[1]).toHaveTextContent('Desativado')
})

it('na própria conta não deixa mudar o papel nem desativar', async () => {
  const user = userEvent.setup()
  renderPage()
  await list()
  await user.click(screen.getByRole('button', { name: 'Editar Maria' }))
  expect(screen.getByRole('radio', { name: 'Funcionário' })).toBeDisabled()
  expect(screen.getByRole('checkbox', { name: 'Conta ativa' })).toBeDisabled()
  expect(screen.getByText('Não podes desativar a tua própria conta.')).toBeInTheDocument()
})

it('mostra o erro da API, por exemplo um e-mail repetido', async () => {
  server.use(http.post('*/api/v1/users', () => problem(409, 'Já existe um utilizador com este e-mail.')))
  const user = userEvent.setup()
  renderPage()
  await list()
  await user.click(screen.getByRole('button', { name: 'Novo utilizador' }))
  await user.type(screen.getByLabelText('E-mail'), 'ze@quinta.pt')
  await user.type(screen.getByLabelText('Nome'), 'Outro Zé')
  await user.type(screen.getByLabelText('Palavra-passe'), 'uma-frase-comprida')
  await user.click(screen.getByRole('button', { name: 'Criar utilizador' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('Já existe um utilizador com este e-mail.')
  expect(screen.getByRole('button', { name: 'Criar utilizador' })).toBeEnabled()
})
