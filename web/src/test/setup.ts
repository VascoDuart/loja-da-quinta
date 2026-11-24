import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@testing-library/react'
import { afterAll, afterEach, beforeAll, vi } from 'vitest'

import { resetSessionForTests } from '../features/auth/session'
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

// As páginas do backoffice carregam o código à parte (lazy), recuperam a
// sessão e só depois pedem os dados: 1 s por omissão é pouco com os testes
// em paralelo, sobretudo na primeira execução (transformação a frio, como no CI).
configure({ asyncUtilTimeout: 5000 })

// Pedidos sem resposta definida falham o teste: nada vai à rede real.
beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterAll(() => server.close())

afterEach(() => {
  cleanup()
  server.resetHandlers()
  resetSessionForTests()
  delete document.documentElement.dataset.theme
  // eslint-disable-next-line no-restricted-globals -- limpar a preferência de tema entre testes
  localStorage.clear()
})
