import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { expect, it } from 'vitest'

import { problem, server } from '../../test/msw'
import { renderWithQuery } from '../../test/render'
import PainelPage from './PainelPage'

it('mostra o servidor pronto quando o /readyz responde 200', async () => {
  renderWithQuery(<PainelPage />)
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
  renderWithQuery(<PainelPage />)

  expect(await screen.findByText(/O servidor teve um problema/)).toBeInTheDocument()
  expect(screen.getByText('Referência: ref-42')).toBeInTheDocument()
  expect(calls).toBe(2) // um 5xx repete-se uma vez antes de mostrar o erro

  await userEvent.click(screen.getByRole('button', { name: 'Tentar de novo' }))
  expect(await screen.findByText(/Servidor pronto/)).toBeInTheDocument()
})
