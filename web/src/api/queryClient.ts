import { QueryClient } from '@tanstack/react-query'

import { ApiError } from './http'

// Todos os erros dos hooks gerados são ApiError (ver http.ts).
declare module '@tanstack/react-query' {
  interface Register {
    defaultError: ApiError
  }
}

/**
 * Política de cache e de novas tentativas para toda a app.
 * - Repete uma vez falhas de rede e 5xx; nunca repete 4xx (o pedido está
 *   errado ou não é permitido, repetir não ajuda).
 * - Nunca repete escritas automaticamente: uma venda repetida às cegas seria
 *   uma venda em duplicado (a idempotência chega com a #15).
 */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: (failures, err) => failures < 1 && err instanceof ApiError && err.retryable,
        refetchOnWindowFocus: false,
      },
      mutations: {
        retry: false,
      },
    },
  })
}
