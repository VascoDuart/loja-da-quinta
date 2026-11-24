import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { ApiError, apiFetch } from '../../api/http'
import { problem, server, sessionBody, signedInAs } from '../../test/msw'
import { getSession, refresh, restore, signIn, signOut } from './session'

/** Um endpoint protegido de teste que só aceita o token indicado. */
function protectedEndpoint(validToken: string, onCall?: (auth: string | null) => void) {
  return http.get('*/api/v1/me', ({ request }) => {
    const auth = request.headers.get('Authorization')
    onCall?.(auth)
    return auth === `Bearer ${validToken}` ? HttpResponse.json({ ok: true }) : problem(401, 'A sessão expirou.')
  })
}

describe('sessão', () => {
  it('o login guarda o token em memória e envia-o como Bearer, com X-CSRF', async () => {
    let csrf: string | null = null
    server.use(
      http.post('*/api/v1/auth/login', ({ request }) => {
        csrf = request.headers.get('X-CSRF')
        return HttpResponse.json(sessionBody('gestor', 'abc'))
      }),
      protectedEndpoint('abc'),
    )
    const user = await signIn('maria@quinta.pt', 'palavra-passe')
    expect(user.name).toBe('Maria')
    expect(csrf).toBe('1')
    expect(getSession()).toMatchObject({ status: 'authenticated', user: { role: 'gestor' } })
    await expect(apiFetch('/api/v1/me')).resolves.toMatchObject({ status: 200 })
    // eslint-disable-next-line no-restricted-globals -- confirmar que o token nunca vai para o localStorage
    expect(JSON.stringify(localStorage)).not.toContain('abc')
  })

  it('ao abrir a app, recupera a sessão pelo cookie', async () => {
    server.use(signedInAs('funcionario'))
    await restore()
    expect(getSession()).toMatchObject({ status: 'authenticated', user: { role: 'funcionario' } })
  })

  it('ao abrir a app sem cookie fica sem sessão (sem a marcar como expirada)', async () => {
    await restore()
    expect(getSession()).toEqual({ status: 'anonymous' })
  })

  it('sem ligação ao abrir a app fica sem sessão', async () => {
    server.use(http.post('*/api/v1/auth/refresh', () => HttpResponse.error()))
    await restore()
    expect(getSession()).toEqual({ status: 'anonymous' })
  })

  it('num 401, renova uma vez e repete o pedido com o token novo', async () => {
    const seen: (string | null)[] = []
    server.use(
      http.post('*/api/v1/auth/login', () => HttpResponse.json(sessionBody('gestor', 'velho'))),
      http.post('*/api/v1/auth/refresh', () => HttpResponse.json(sessionBody('gestor', 'novo'))),
      protectedEndpoint('novo', (a) => seen.push(a)),
    )
    await signIn('maria@quinta.pt', 'x')
    await expect(apiFetch('/api/v1/me')).resolves.toMatchObject({ status: 200 })
    expect(seen).toEqual(['Bearer velho', 'Bearer novo'])
  })

  it('vários 401 em simultâneo partilham uma só renovação', async () => {
    let refreshes = 0
    server.use(
      http.post('*/api/v1/auth/login', () => HttpResponse.json(sessionBody('gestor', 'velho'))),
      http.post('*/api/v1/auth/refresh', async () => {
        refreshes++
        await new Promise((r) => setTimeout(r, 20))
        return HttpResponse.json(sessionBody('gestor', 'novo'))
      }),
      protectedEndpoint('novo'),
    )
    await signIn('maria@quinta.pt', 'x')
    await Promise.all([apiFetch('/api/v1/me'), apiFetch('/api/v1/me'), apiFetch('/api/v1/me')])
    // Dois refreshes com o mesmo token seriam tratados como roubo pela API.
    expect(refreshes).toBe(1)
  })

  it('se a renovação falhar, a sessão termina como expirada e o erro original sobe', async () => {
    server.use(
      http.post('*/api/v1/auth/login', () => HttpResponse.json(sessionBody('gestor', 'velho'))),
      protectedEndpoint('nenhum'),
    )
    await signIn('maria@quinta.pt', 'x')
    const err = await apiFetch('/api/v1/me').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(401)
    expect(getSession()).toEqual({ status: 'anonymous', reason: 'expired' })
  })

  it('sem sessão, um 401 não tenta renovar', async () => {
    let refreshes = 0
    server.use(
      http.post('*/api/v1/auth/refresh', () => {
        refreshes++
        return problem(401, 'x')
      }),
      protectedEndpoint('qualquer'),
    )
    await expect(apiFetch('/api/v1/me')).rejects.toBeInstanceOf(ApiError)
    expect(refreshes).toBe(0)
  })

  it('o logout chama a API com X-CSRF e esquece o token', async () => {
    let csrf: string | null = null
    server.use(
      signedInAs('gestor'),
      http.post('*/api/v1/auth/logout', ({ request }) => {
        csrf = request.headers.get('X-CSRF')
        return new HttpResponse(null, { status: 204 })
      }),
      protectedEndpoint('token-gestor'),
    )
    await refresh()
    await signOut()
    expect(csrf).toBe('1')
    expect(getSession()).toEqual({ status: 'anonymous', reason: 'logout' })
    await expect(apiFetch('/api/v1/me')).rejects.toBeInstanceOf(ApiError)
  })

  it('o logout termina a sessão mesmo sem ligação à API', async () => {
    server.use(
      signedInAs('gestor'),
      http.post('*/api/v1/auth/logout', () => HttpResponse.error()),
    )
    await refresh()
    await signOut()
    expect(getSession().status).toBe('anonymous')
  })

  it('um login durante uma renovação lenta não é desfeito por ela', async () => {
    // Abrir a app com a API a acordar: a renovação inicial demora e acaba em
    // 401. A pessoa entra entretanto. A sessão nova tem de ficar.
    const order: string[] = []
    server.use(
      http.post('*/api/v1/auth/refresh', async () => {
        order.push('refresh:início')
        await new Promise((r) => setTimeout(r, 50))
        order.push('refresh:fim')
        return problem(401, 'A sessão terminou.')
      }),
      http.post('*/api/v1/auth/login', () => {
        order.push('login')
        return HttpResponse.json(sessionBody('gestor', 'novo'))
      }),
    )
    const restoring = restore()
    await signIn('maria@quinta.pt', 'x')
    await restoring
    expect(order).toEqual(['refresh:início', 'refresh:fim', 'login'])
    expect(getSession()).toMatchObject({ status: 'authenticated', user: { role: 'gestor' } })
  })
})
