import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { expect, it } from 'vitest'

import { routes } from '../../app/routes'
import { problem, server, signedInAs } from '../../test/msw'
import { renderApp } from '../../test/render'
import { calmOverview, stockOverview } from '../../test/stock'

function renderPage(role: 'funcionario' | 'gestor' = 'funcionario') {
  server.use(
    signedInAs(role),
    http.get('*/api/v1/stock', () => HttpResponse.json(stockOverview)),
  )
  renderApp(<RouterProvider router={createMemoryRouter(routes, { initialEntries: ['/app/stock'] })} />)
}

/** O grupo (título e lista) com este título. */
async function group(title: RegExp) {
  const heading = await screen.findByRole('heading', { level: 3, name: title })
  return within(heading.parentElement as HTMLElement)
}

const items = (g: Awaited<ReturnType<typeof group>>) => g.getAllByRole('listitem').map((li) => li.textContent)

it('junta o que pede atenção, sempre com a cor acompanhada de texto', async () => {
  renderPage()

  const expired = await group(/Fora de validade Para dar como quebra \(1\)/)
  expect(items(expired)[0]).toContain('Laranja do Algarve: 1,500 kg, validade 24/11/2026')
  expect(expired.getByText(/Avisa o gestor/)).toBeInTheDocument()

  const expiring = await group(/A expirar Até 28\/11\/2026 \(1\)/)
  expect(items(expiring)).toEqual(['Laranja do Algarve: 2,500 kg, validade 27/11/2026'])

  const low = await group(/Stock baixo Abaixo do limiar \(2\)/)
  expect(items(low)).toEqual([
    'Laranja do Algarve: 2,500 kg para vender (limiar: 5,000 kg)',
    'Salsa: 0 molhos para vender (limiar: 2 molhos)',
  ])
})

it('mostra o stock de cada produto e os lotes, com o estado de cada um', async () => {
  renderPage()
  const list = (await screen.findByText('Para vender: 3 dúzias')).closest('ul') as HTMLElement
  const [laranja, ovos, salsa] = [...list.children].map((li) => within(li as HTMLElement))
  if (!laranja || !ovos || !salsa) throw new Error('faltam produtos')

  expect(laranja.getByText('Para vender: 2,500 kg')).toBeInTheDocument()
  expect(laranja.getByText('Stock baixo')).toBeInTheDocument()
  expect(laranja.getAllByRole('listitem').map((li) => li.textContent)).toEqual([
    'Validade 24/11/2026 · restam 1,500 kg Fora de validade',
    'Validade 27/11/2026 · restam 2,500 kg Expira em breve',
  ])
  expect(ovos.queryByText('Stock baixo')).not.toBeInTheDocument()
  expect(ovos.getByText('Validade 10/12/2026 · restam 3 dúzias')).toBeInTheDocument()
  expect(salsa.getByText('Sem stock')).toBeInTheDocument()
  expect(salsa.queryByRole('list')).not.toBeInTheDocument()
})

it('só o gestor tem a ligação para dar um lote fora de validade como quebra', async () => {
  renderPage('gestor')
  const g = await group(/Fora de validade/)
  const link = g.getByRole('link')
  expect(link).toHaveAccessibleName('Dar como quebra (Laranja do Algarve: 1,500 kg, validade 24/11/2026)')
  expect(link).toHaveAttribute('href', '/app/stock/ajustes?produto=p-laranja&lote=l-velho')
})

it('o funcionário não vê a ligação das quebras', async () => {
  renderPage()
  await group(/Fora de validade/)
  expect(screen.queryByRole('link', { name: /Dar como quebra/ })).not.toBeInTheDocument()
})

it('sem nada a tratar, diz isso e continua a mostrar o stock', async () => {
  server.use(signedInAs('funcionario'))
  renderPage()
  server.use(http.get('*/api/v1/stock', () => HttpResponse.json(calmOverview)))
  expect(await screen.findByText(/Nada a tratar/)).toBeInTheDocument()
  expect(screen.getByText('Para vender: 3 dúzias')).toBeInTheDocument()
})

it('se a consulta falhar, mostra o erro e deixa tentar de novo', async () => {
  let calls = 0
  renderPage()
  server.use(
    http.get('*/api/v1/stock', () => {
      calls++
      return calls <= 2 ? problem(500, 'Ocorreu um erro inesperado.') : HttpResponse.json(calmOverview)
    }),
  )
  await userEvent.click(await screen.findByRole('button', { name: 'Tentar de novo' }))
  expect(await screen.findByText(/Nada a tratar/)).toBeInTheDocument()
})
