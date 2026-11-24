const DEFAULT_DESTINATION = '/app/painel'

/**
 * Para onde ir depois do login. Só se volta para páginas do backoffice: um
 * ?next=https://outro-site ou //outro-site levaria o utilizador para fora
 * (open redirect).
 */
export function safeNext(next: string | null): string {
  if (!next || !/^\/app(\/|\?|$)/.test(next) || next.includes('\\')) return DEFAULT_DESTINATION
  return next
}
