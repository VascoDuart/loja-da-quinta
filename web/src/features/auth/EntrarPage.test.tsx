import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { expect, it } from 'vitest'

import { routes } from '../../app/routes'
import { problem, server, sessionBody } from '../../test/msw'
import { renderApp } from '../../test/render'

function renderAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  renderApp(<RouterProvider router={router} />)
  return router
}

async function fillAndSubmit(email: string, password: string) {
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('E-mail'), email)
  await user.type(screen.getByLabelText('Palavra-passe'), password)
  await user.click(screen.getByRole('button', { name: 'Entrar' }))
}

it('o formulário tem etiquetas e autocomplete para o gestor de palavras-passe', async () => {
  renderAt('/entrar')
  expect(await screen.findByRole('heading', { name: 'Entrar no backoffice' })).toBeInTheDocument()
  expect(screen.getByLabelText('E-mail')).toHaveAttribute('autocomplete', 'username')
  expect(screen.getByLabelText('Palavra-passe')).toHaveAttribute('autocomplete', 'current-password')
  expect(screen.getByLabelText('Palavra-passe')).toHaveAttribute('type', 'password')
})

it('com credenciais certas entra e volta à página pedida', async () => {
  let body: unknown
  server.use(
    http.post('*/api/v1/auth/login', async ({ request }) => {
      body = await request.json()
      return HttpResponse.json(sessionBody('funcionario'))
    }),
  )
  const router = renderAt('/entrar?next=%2Fapp%2Fvenda')
  await fillAndSubmit('ze@quinta.pt', 'palavra-passe-certa')

  expect(await screen.findByRole('heading', { name: 'Venda', level: 1 })).toBeInTheDocument()
  expect(router.state.location.pathname).toBe('/app/venda')
  expect(body).toEqual({ email: 'ze@quinta.pt', password: 'palavra-passe-certa' })
})

it('com credenciais erradas mostra a mensagem genérica da API e fica na página', async () => {
  server.use(http.post('*/api/v1/auth/login', () => problem(401, 'E-mail ou palavra-passe incorretos.')))
  const router = renderAt('/entrar')
  await fillAndSubmit('ze@quinta.pt', 'errada')

  expect(await screen.findByRole('alert')).toHaveTextContent('E-mail ou palavra-passe incorretos.')
  expect(router.state.location.pathname).toBe('/entrar')
  expect(screen.getByRole('button', { name: 'Entrar' })).toBeEnabled()
})

it('com a conta bloqueada mostra quanto tempo esperar', async () => {
  server.use(
    http.post('*/api/v1/auth/login', () =>
      problem(429, 'Demasiadas tentativas falhadas. Espera 15 minutos e tenta de novo.'),
    ),
  )
  renderAt('/entrar')
  await fillAndSubmit('ze@quinta.pt', 'errada')
  expect(await screen.findByRole('alert')).toHaveTextContent('Espera 15 minutos')
})

it('com um next para outro site vai para o painel', async () => {
  server.use(http.post('*/api/v1/auth/login', () => HttpResponse.json(sessionBody('gestor'))))
  const router = renderAt('/entrar?next=https%3A%2F%2Fmau.example')
  await fillAndSubmit('maria@quinta.pt', 'palavra-passe-certa')
  expect(await screen.findByRole('heading', { name: 'Painel', level: 1 })).toBeInTheDocument()
  expect(router.state.location.pathname).toBe('/app/painel')
})
