import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, vi } from 'vitest'

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

afterEach(() => {
  cleanup()
  delete document.documentElement.dataset.theme
  // eslint-disable-next-line no-restricted-globals -- limpar a preferência de tema entre testes
  localStorage.clear()
})
