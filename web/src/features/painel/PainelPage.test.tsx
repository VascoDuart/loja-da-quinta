import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { MemoryRouter } from 'react-router'
import { expect, it } from 'vitest'

import { problem, server } from '../../test/msw'
import { renderWithQuery } from '../../test/render'
import { stockOverview } from '../../test/stock'
import PainelPage from './PainelPage'

const renderPage = () =>
  renderWithQuery(
    <MemoryRouter>
      <PainelPage />
    </MemoryRouter>,
  )

it('mostra o servidor pronto quando o /readyz responde 200', async () => {
  renderPage()
  expect(await screen.findByText(/Servidor pronto/)).toBeInTheDocument()
})

it('com a base de dados em baixo mostra uma mensagem simples, a referência e permite tentar de novo', async () => {
  let calls = 0
  server.use(
    http.get('*/api/v1/readyz', () => {
      calls++
      return calls <= 2
        ? problem(503, 'A base de dados não está acessível.', 'ref-42')
        : HttpResponse.json({ status: 'ok', version: 'test' })
    }),
  )
  renderPage()

  expect(await screen.findByText(/O servidor teve um problema/)).toBeInTheDocument()
  expect(screen.getByText('Referência: ref-42')).toBeInTheDocument()
  expect(calls).toBe(2) // um 5xx repete-se uma vez antes de mostrar o erro

  await userEvent.click(screen.getByRole('button', { name: 'Tentar de novo' }))
  expect(await screen.findByText(/Servidor pronto/)).toBeInTheDocument()
})

it('resume o stock que pede atenção, com texto em cada destaque', async () => {
  server.use(http.get('*/api/v1/stock', () => HttpResponse.json(stockOverview)))
  renderPage()

  const text = (await screen.findAllByRole('listitem')).map((li) => li.textContent)
  expect(text).toEqual([
    'Fora de validade 1 lote para dar como quebra: Laranja do Algarve',
    'A expirar 1 lote expira nos próximos 3 dias: Laranja do Algarve',
    'Stock baixo 2 produtos: Laranja do Algarve, Salsa',
  ])
  expect(screen.getByRole('link', { name: 'Ver o stock' })).toHaveAttribute('href', '/app/stock')
})

it('sem nada a tratar no stock, diz isso', async () => {
  renderPage()
  expect(await screen.findByText('Nada a tratar no stock.')).toBeInTheDocument()
})
