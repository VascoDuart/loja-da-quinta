import { Navigate, type RouteObject } from 'react-router'

import { BackofficeLayout } from './layouts/BackofficeLayout'
import { PublicLayout } from './layouts/PublicLayout'
import { NotFoundPage } from './NotFoundPage'

/** Papéis do backoffice (docs/security/authz-matrix.md). */
export type Role = 'funcionario' | 'gestor'

/**
 * Metadados de cada rota.
 * - `title`: título da página (separador do browser e cabeçalho).
 * - `roles`: quem pode ver a página. A guarda que o aplica chega com a
 *   autenticação (#8). A autorização real é sempre da API; isto só evita
 *   mostrar ecrãs inúteis.
 * - `nav`: aparece no menu do backoffice.
 */
export type RouteHandle = {
  title: string
  roles?: readonly Role[]
  nav?: boolean
}

const todos: readonly Role[] = ['funcionario', 'gestor']
const soGestor: readonly Role[] = ['gestor']

// Cada página é carregada só quando é visitada (lazy loading): o catálogo
// público não descarrega o código do backoffice.
const page = (load: () => Promise<{ default: React.ComponentType }>) => async () => ({
  Component: (await load()).default,
})

export const routes: RouteObject[] = [
  {
    element: <PublicLayout />,
    children: [
      {
        index: true,
        lazy: page(() => import('../features/catalogo/CatalogoPage')),
        handle: { title: 'Produtos da época' } satisfies RouteHandle,
      },
      {
        path: 'produtos/:id',
        lazy: page(() => import('../features/catalogo/ProdutoPage')),
        handle: { title: 'Produto' } satisfies RouteHandle,
      },
      {
        path: 'entrar',
        lazy: page(() => import('../features/auth/EntrarPage')),
        handle: { title: 'Entrar' } satisfies RouteHandle,
      },
    ],
  },
  {
    path: 'app',
    element: <BackofficeLayout />,
    children: [
      { index: true, element: <Navigate to="painel" replace /> },
      {
        path: 'painel',
        lazy: page(() => import('../features/painel/PainelPage')),
        handle: { title: 'Painel', roles: todos, nav: true } satisfies RouteHandle,
      },
      {
        path: 'venda',
        lazy: page(() => import('../features/venda/VendaPage')),
        handle: { title: 'Venda', roles: todos, nav: true } satisfies RouteHandle,
      },
      {
        path: 'stock',
        lazy: page(() => import('../features/stock/StockPage')),
        handle: { title: 'Stock', roles: todos, nav: true } satisfies RouteHandle,
      },
      {
        path: 'produtos',
        lazy: page(() => import('../features/produtos/ProdutosPage')),
        handle: { title: 'Produtos', roles: soGestor, nav: true } satisfies RouteHandle,
      },
      {
        path: 'relatorios',
        lazy: page(() => import('../features/relatorios/RelatoriosPage')),
        handle: { title: 'Relatórios', roles: soGestor, nav: true } satisfies RouteHandle,
      },
      {
        path: 'utilizadores',
        lazy: page(() => import('../features/utilizadores/UtilizadoresPage')),
        handle: { title: 'Utilizadores', roles: soGestor, nav: true } satisfies RouteHandle,
      },
      {
        path: 'auditoria',
        lazy: page(() => import('../features/auditoria/AuditoriaPage')),
        handle: { title: 'Auditoria', roles: soGestor, nav: true } satisfies RouteHandle,
      },
    ],
  },
  { path: '*', element: <NotFoundPage />, handle: { title: 'Página não encontrada' } satisfies RouteHandle },
]

/** Entradas do menu do backoffice, pela ordem das rotas. */
export const backofficeNav = (routes.find((r) => r.path === 'app')?.children ?? []).flatMap((r) => {
  const handle = r.handle as RouteHandle | undefined
  return handle?.nav && r.path ? [{ to: `/app/${r.path}`, title: handle.title, roles: handle.roles ?? [] }] : []
})
