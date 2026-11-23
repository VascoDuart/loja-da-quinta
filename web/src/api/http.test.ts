import { http, HttpResponse } from 'msw'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { problem, server } from '../test/msw'
import { ApiError, apiFetch, errorMessage, TIMEOUT_MS } from './http'
import { slowRequests } from './slowRequests'

afterEach(() => {
  vi.useRealTimers()
})

describe('apiFetch', () => {
  it('devolve dados, status e cabeçalhos no formato do orval', async () => {
    const res = await apiFetch<{ data: { status: string }; status: number }>('/api/v1/healthz')
    expect(res.status).toBe(200)
    expect(res.data).toEqual({ status: 'ok', version: 'test' })
  })

  it('transforma problem+json num ApiError com o request ID', async () => {
    server.use(http.get('*/api/v1/readyz', () => problem(503, 'A base de dados não está acessível.', 'abc-123')))
    const err = await apiFetch('/api/v1/readyz').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 503, requestId: 'abc-123', retryable: true })
  })

  it('um 4xx não se repete', async () => {
    server.use(http.get('*/api/v1/readyz', () => problem(403, 'Sem permissão.')))
    const err = (await apiFetch('/api/v1/readyz').catch((e: unknown) => e)) as ApiError
    expect(err.retryable).toBe(false)
  })

  it('falha de rede vira ApiError com status 0', async () => {
    server.use(http.get('*/api/v1/readyz', () => HttpResponse.error()))
    const err = (await apiFetch('/api/v1/readyz').catch((e: unknown) => e)) as ApiError
    expect(err.status).toBe(0)
    expect(errorMessage(err)).toMatch(/Sem ligação/)
  })

  it(`desiste ao fim de ${TIMEOUT_MS / 1000} s com 408`, async () => {
    vi.useFakeTimers()
    server.use(http.get('*/api/v1/readyz', () => new Promise<Response>(() => {})))
    const pending = apiFetch('/api/v1/readyz').catch((e: unknown) => e)
    await vi.advanceTimersByTimeAsync(TIMEOUT_MS)
    const err = (await pending) as ApiError
    expect(err.status).toBe(408)
    expect(slowRequests.count()).toBe(0)
  })
})

describe('errorMessage', () => {
  it('nunca mostra detalhes técnicos de erros 5xx', () => {
    const err = new ApiError(500, { detail: 'pq: relation "x" does not exist' })
    expect(errorMessage(err)).not.toContain('relation')
  })

  it('mostra o detalhe da API nos 4xx, que é escrito para pessoas', () => {
    expect(errorMessage(new ApiError(409, { detail: 'Stock insuficiente de Laranja.' }))).toBe(
      'Stock insuficiente de Laranja.',
    )
  })
})
