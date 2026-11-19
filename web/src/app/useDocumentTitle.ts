import { useEffect } from 'react'
import { useMatches } from 'react-router'

import type { RouteHandle } from './routes'

/** Título da rota mais específica, também usado no separador do browser. */
export function usePageTitle(): string | undefined {
  const matches = useMatches()
  for (let i = matches.length - 1; i >= 0; i--) {
    const handle = matches[i]?.handle as RouteHandle | undefined
    if (handle?.title) return handle.title
  }
  return undefined
}

export function useDocumentTitle(): void {
  const title = usePageTitle()
  useEffect(() => {
    document.title = title ? `${title} · Loja da Quinta` : 'Loja da Quinta'
  }, [title])
}
