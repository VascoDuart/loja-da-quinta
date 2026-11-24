/**
 * Sessão do backoffice (ADR-0005, docs/security/controls.md).
 *
 * - O access token (15 min) fica só nesta variável em memória: nunca em
 *   localStorage, onde um XSS o encontraria.
 * - O refresh token está num cookie HttpOnly que o JavaScript não lê. Ao
 *   abrir a app, uma renovação recupera a sessão.
 * - Cada refresh token só pode ser usado uma vez: a API trata um segundo uso
 *   como roubo e termina a sessão. Por isso, as renovações em simultâneo
 *   partilham o mesmo pedido e, entre separadores, esperam umas pelas outras
 *   com um Web Lock.
 */
import { useSyncExternalStore } from 'react'

import {
  login as loginRequest,
  logout as logoutRequest,
  refreshSession as refreshRequest,
} from '../../api/generated/endpoints/auth/auth'
import type { Session, User } from '../../api/generated/model'
import { ApiError, configureAuth } from '../../api/http'

export type SessionState =
  | { status: 'loading' }
  | { status: 'anonymous'; reason?: 'expired' | 'logout' }
  | { status: 'authenticated'; user: User }

/** Cabeçalho anti-CSRF que a API exige nos pedidos de sessão. */
const CSRF = { 'X-CSRF': '1' }
const LOCK_NAME = 'loja-da-quinta:refresh'

let accessToken: string | null = null
let state: SessionState = { status: 'loading' }
let refreshing: Promise<boolean> | null = null
const listeners = new Set<() => void>()

function setState(next: SessionState) {
  state = next
  listeners.forEach((l) => l())
}

function start(s: Session) {
  accessToken = s.accessToken
  setState({ status: 'authenticated', user: s.user })
}

function end(reason: 'expired' | 'logout') {
  accessToken = null
  setState({ status: 'anonymous', reason })
}

export function getSession(): SessionState {
  return state
}

export function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/** Estado da sessão para os componentes. */
export function useSession(): SessionState {
  return useSyncExternalStore(subscribe, getSession)
}

/** Inicia sessão. Lança ApiError (401 credenciais, 429 bloqueio). */
export async function signIn(email: string, password: string): Promise<User> {
  const res = await loginRequest({ email, password }, { headers: CSRF })
  if (res.status !== 200) throw new ApiError(res.status)
  start(res.data)
  return res.data.user
}

/**
 * Renova a sessão. Devolve false (e termina a sessão) se a API a recusar;
 * lança se não for possível falar com a API.
 */
export function refresh(): Promise<boolean> {
  refreshing ??= withLock(async () => {
    try {
      const res = await refreshRequest({ headers: CSRF })
      if (res.status !== 200) throw new ApiError(res.status)
      start(res.data)
      return true
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        end('expired')
        return false
      }
      throw err
    }
  }).finally(() => {
    refreshing = null
  })
  return refreshing
}

/** Ao abrir a app: tenta recuperar a sessão pelo cookie. */
export async function restore(): Promise<void> {
  if (state.status !== 'loading') return
  try {
    await refresh()
  } catch {
    // Sem ligação à API: fica sem sessão; o login mostra o erro se falhar.
    if (getSession().status === 'loading') setState({ status: 'anonymous' })
  }
  // Um 401 já pôs a sessão como "expirada", mas ao abrir a app não houve
  // sessão nenhuma que expirasse.
  const current = getSession()
  if (current.status === 'anonymous' && current.reason === 'expired') setState({ status: 'anonymous' })
}

/** Termina a sessão na API e na memória. Funciona mesmo sem ligação. */
export async function signOut(): Promise<void> {
  try {
    await logoutRequest({ headers: CSRF })
  } catch {
    // A sessão termina aqui de qualquer forma; o refresh expira em 7 dias.
  }
  end('logout')
}

function withLock<T>(fn: () => Promise<T>): Promise<T> {
  const locks = typeof navigator !== 'undefined' ? navigator.locks : undefined
  return locks ? locks.request(LOCK_NAME, fn) : fn()
}

configureAuth({ token: () => accessToken, refresh })

/** Só para os testes: volta ao estado inicial. */
export function resetSessionForTests(): void {
  accessToken = null
  refreshing = null
  state = { status: 'loading' }
  listeners.forEach((l) => l())
}
