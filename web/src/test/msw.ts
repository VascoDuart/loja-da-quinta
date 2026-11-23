import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

/** Respostas por omissão: a API está saudável. Cada teste pode substituí-las. */
export const handlers = [
  http.get('*/api/v1/healthz', () => HttpResponse.json({ status: 'ok', version: 'test' })),
  http.get('*/api/v1/readyz', () => HttpResponse.json({ status: 'ok', version: 'test' })),
]

export const server = setupServer(...handlers)

/** Resposta de erro igual à da API (problem+json com o request ID). */
export function problem(status: number, detail: string, requestId = 'req-123') {
  return HttpResponse.json(
    { type: 'about:blank', title: 'Erro', status, detail, instance: requestId },
    { status, headers: { 'Content-Type': 'application/problem+json', 'X-Request-ID': requestId } },
  )
}
