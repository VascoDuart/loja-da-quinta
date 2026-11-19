import { QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactElement } from 'react'

import { createQueryClient } from '../api/queryClient'

/** Render com um QueryClient novo por teste, para não partilhar cache. */
export function renderWithQuery(ui: ReactElement) {
  const client = createQueryClient()
  // Mesma política de novas tentativas, mas sem esperar entre elas.
  const defaults = client.getDefaultOptions()
  client.setDefaultOptions({ ...defaults, queries: { ...defaults.queries, retryDelay: 0 } })
  return { client, ...render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>) }
}
