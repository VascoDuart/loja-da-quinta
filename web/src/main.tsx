import '@fontsource-variable/plus-jakarta-sans'
import './shared/styles/tokens.css'
import './app/layouts.css'

import { QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'

import { createQueryClient } from './api/queryClient'
import { routes } from './app/routes'
import { AuthProvider } from './features/auth/AuthProvider'
import { initTheme } from './shared/theme/theme'
import { WakingServer } from './shared/ui/WakingServer'

// Antes do primeiro render, para a página não piscar no tema errado.
initTheme()

const router = createBrowserRouter(routes)
const queryClient = createQueryClient()

const root = document.getElementById('root')
if (!root) throw new Error('Falta o elemento #root no index.html')

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
      <WakingServer />
    </QueryClientProvider>
  </StrictMode>,
)
