/**
 * Cliente HTTP usado por todo o código gerado pelo orval.
 *
 * - Mesma origem: os pedidos vão para /api/v1 e a Vercel (ou o proxy do
 *   Vite) encaminha para a API (ADR-0007). Sem CORS, com cookies Strict.
 * - Pedido lento: ao fim de 3 s avisa o ecrã "a acordar o servidor"; ao fim
 *   de 75 s desiste (o proxy da Vercel corta entre 91 e 120 s).
 * - Erros: qualquer resposta fora de 2xx vira um ApiError com o problem+json
 *   da API, incluindo o request ID para encontrar o pedido nos logs.
 * - Sessão: envia o access token (que só existe em memória) e, num 401,
 *   renova a sessão uma vez e repete o pedido. Quem fornece o token e a
 *   renovação é o módulo de sessão (configureAuth), para este ficheiro não
 *   depender dele.
 */
import { slowRequests } from './slowRequests'

export const SLOW_AFTER_MS = 3_000
export const TIMEOUT_MS = 75_000

export type Problem = {
  type?: string
  title?: string
  status?: number
  detail?: string
  instance?: string
}

export class ApiError extends Error {
  readonly status: number
  readonly problem: Problem | undefined
  readonly requestId: string | undefined

  constructor(status: number, problem?: Problem, requestId?: string) {
    super(problem?.detail ?? problem?.title ?? `Erro ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.problem = problem
    this.requestId = requestId ?? problem?.instance
  }

  /** Erros do pedido (4xx) não se repetem; falhas de rede e 5xx sim. */
  get retryable(): boolean {
    return this.status === 0 || this.status >= 500
  }
}

/** Mensagem para mostrar a quem usa a app. Nunca mostra detalhes técnicos. */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 0) return 'Sem ligação ao servidor. Verifica a internet e tenta de novo.'
    if (err.status === 408) return 'O servidor demorou demasiado a responder. Tenta de novo.'
    if (err.status === 429) return 'Demasiados pedidos seguidos. Espera um momento e tenta de novo.'
    if (err.status >= 500) return 'O servidor teve um problema. Tenta de novo daqui a pouco.'
    return err.problem?.detail ?? 'O pedido não foi aceite.'
  }
  return 'Ocorreu um erro inesperado.'
}

/** Tipo de erro que o orval usa nos hooks gerados: é sempre um ApiError. */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export type ErrorType<_Error> = ApiError

/** Forma da resposta que o código gerado pelo orval espera. */
export type ApiResponse<T> = { data: T; status: number; headers: Headers }

/** O que o apiFetch precisa da sessão. */
export type AuthHooks = {
  /** Access token atual, ou null sem sessão. */
  token: () => string | null
  /** Renova a sessão. true se houver um access token novo. */
  refresh: () => Promise<boolean>
}

let auth: AuthHooks | null = null

export function configureAuth(hooks: AuthHooks | null): void {
  auth = hooks
}

/** Os endpoints da sessão nunca levam Bearer nem disparam uma renovação. */
const isAuthEndpoint = (url: string) => url.includes('/api/v1/auth/')

export async function apiFetch<T>(url: string, init: RequestInit = {}): Promise<T> {
  try {
    return await send<T>(url, init)
  } catch (err) {
    if (!(err instanceof ApiError) || err.status !== 401 || !auth || isAuthEndpoint(url) || !auth.token()) throw err
    const renewed = await auth.refresh().catch(() => false)
    if (!renewed) throw err
    return send<T>(url, init)
  }
}

async function send<T>(url: string, init: RequestInit): Promise<T> {
  const token = isAuthEndpoint(url) ? null : (auth?.token() ?? null)
  const controller = new AbortController()
  const onCallerAbort = () => controller.abort(init.signal?.reason)
  init.signal?.addEventListener('abort', onCallerAbort, { once: true })

  const done = slowRequests.track(SLOW_AFTER_MS)
  const timeout = setTimeout(() => controller.abort(new DOMException('timeout', 'TimeoutError')), TIMEOUT_MS)

  let res: Response
  try {
    res = await fetch(url, {
      ...init,
      signal: controller.signal,
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json, application/problem+json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...init.headers,
      },
    })
  } catch (err) {
    if (init.signal?.aborted) throw err // cancelado pelo TanStack Query: não é um erro
    const timedOut =
      controller.signal.reason instanceof DOMException && controller.signal.reason.name === 'TimeoutError'
    throw new ApiError(timedOut ? 408 : 0)
  } finally {
    clearTimeout(timeout)
    init.signal?.removeEventListener('abort', onCallerAbort)
    done()
  }

  const requestId = res.headers.get('X-Request-ID') ?? undefined
  const body = await readBody(res)

  if (!res.ok) {
    throw new ApiError(res.status, isProblem(body) ? body : undefined, requestId)
  }
  return { data: body, status: res.status, headers: res.headers } as T
}

async function readBody(res: Response): Promise<unknown> {
  if (res.status === 204) return undefined
  const type = res.headers.get('Content-Type') ?? ''
  if (type.includes('json')) return res.json().catch(() => undefined)
  return res.text()
}

function isProblem(v: unknown): v is Problem {
  return typeof v === 'object' && v !== null && ('title' in v || 'status' in v)
}
