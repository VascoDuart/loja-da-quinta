import { defineConfig } from 'orval'

// Gera o cliente da API a partir do mesmo contrato que o backend usa
// (ADR-0003). Corre com: npm run gen. Os ficheiros gerados não se editam.
export default defineConfig({
  api: {
    input: '../openapi/openapi.yaml',
    output: {
      mode: 'tags-split',
      target: 'src/api/generated/endpoints',
      schemas: 'src/api/generated/model',
      client: 'react-query',
      httpClient: 'fetch',
      baseUrl: '/api/v1',
      clean: true,
      prettier: false,
      override: {
        mutator: { path: 'src/api/http.ts', name: 'apiFetch' },
        query: { useSuspenseQuery: false },
      },
    },
  },
  zod: {
    input: '../openapi/openapi.yaml',
    output: {
      mode: 'tags-split',
      target: 'src/api/generated/zod',
      client: 'zod',
      fileExtension: '.zod.ts',
      clean: true,
      prettier: false,
    },
  },
})
