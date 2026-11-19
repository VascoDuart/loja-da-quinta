import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'

import { routes } from './app/routes'
import './app/base.css'

const router = createBrowserRouter(routes)

const root = document.getElementById('root')
if (!root) throw new Error('Falta o elemento #root no index.html')

createRoot(root).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
)
