import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

import type { Role, Session, User } from '../api/generated/model'

export const testUsers: Record<Role, User> = {
  gestor: { id: '00000000-0000-4000-8000-000000000001', email: 'maria@quinta.pt', name: 'Maria', role: 'gestor' },
  funcionario: { id: '00000000-0000-4000-8000-000000000002', email: 'ze@quinta.pt', name: 'Zé', role: 'funcionario' },
}

export function sessionBody(role: Role, accessToken = `token-${role}`): Session {
  return { accessToken, tokenType: 'Bearer', expiresIn: 900, user: testUsers[role] }
}

/** Faz a renovação devolver uma sessão (a app abre já com sessão). */
export function signedInAs(role: Role) {
  return http.post('*/api/v1/auth/refresh', () => HttpResponse.json(sessionBody(role)))
}

/**
 * Respostas por omissão: a API está saudável e não há sessão (o refresh dá
 * 401). Cada teste pode substituí-las.
 */
export const handlers = [
  http.get('*/api/v1/healthz', () => HttpResponse.json({ status: 'ok', version: 'test' })),
  http.get('*/api/v1/readyz', () => HttpResponse.json({ status: 'ok', version: 'test' })),
  http.post('*/api/v1/auth/refresh', () => problem(401, 'A sessão terminou. Inicia sessão de novo.')),
  http.post('*/api/v1/auth/logout', () => new HttpResponse(null, { status: 204 })),
  // Listas vazias por omissão: cada teste de página define as suas.
  http.get('*/api/v1/users', () => HttpResponse.json([])),
  http.get('*/api/v1/categories', () => HttpResponse.json([])),
  http.get('*/api/v1/products', () => HttpResponse.json([])),
]

export const server = setupServer(...handlers)

/** Resposta de erro igual à da API (problem+json com o request ID). */
export function problem(status: number, detail: string, requestId = 'req-123') {
  return HttpResponse.json(
    { type: 'about:blank', title: 'Erro', status, detail, instance: requestId },
    { status, headers: { 'Content-Type': 'application/problem+json', 'X-Request-ID': requestId } },
  )
}
