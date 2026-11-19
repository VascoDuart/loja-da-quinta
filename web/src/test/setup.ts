import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll, vi } from 'vitest'

import { server } from './msw'

// O jsdom não implementa matchMedia; o tema do sistema conta como claro.
window.matchMedia ??= vi.fn().mockImplementation((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  addListener: vi.fn(),
  removeListener: vi.fn(),
  dispatchEvent: vi.fn(),
}))

// Pedidos sem resposta definida falham o teste: nada vai à rede real.
beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterAll(() => server.close())

afterEach(() => {
  cleanup()
  server.resetHandlers()
  delete document.documentElement.dataset.theme
  // eslint-disable-next-line no-restricted-globals -- limpar a preferência de tema entre testes
  localStorage.clear()
})
