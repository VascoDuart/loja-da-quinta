import '@fontsource-variable/plus-jakarta-sans'
import './shared/styles/tokens.css'
import './app/layouts.css'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'

import { routes } from './app/routes'
import { initTheme } from './shared/theme/theme'

// Antes do primeiro render, para a página não piscar no tema errado.
initTheme()

const router = createBrowserRouter(routes)

const root = document.getElementById('root')
if (!root) throw new Error('Falta o elemento #root no index.html')

createRoot(root).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
)
