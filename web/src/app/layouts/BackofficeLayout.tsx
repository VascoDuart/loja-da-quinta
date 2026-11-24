import type { ReactNode } from 'react'
import { Navigate, NavLink, Outlet, useLocation, useMatches, useNavigate } from 'react-router'

import { signOut, useSession } from '../../features/auth/session'
import { ThemeToggle } from '../../shared/theme/ThemeToggle'
import {
  IconAuditoria,
  IconPainel,
  IconProdutos,
  IconRelatorios,
  IconStock,
  IconUtilizadores,
  IconVenda,
} from '../../shared/ui/icons'
import { backofficeNav, type Role, type RouteHandle } from '../routes'
import { useDocumentTitle, usePageTitle } from '../useDocumentTitle'

const icons: Record<string, ReactNode> = {
  '/app/painel': <IconPainel />,
  '/app/venda': <IconVenda />,
  '/app/stock': <IconStock />,
  '/app/produtos': <IconProdutos />,
  '/app/relatorios': <IconRelatorios />,
  '/app/utilizadores': <IconUtilizadores />,
  '/app/auditoria': <IconAuditoria />,
}

const roleLabel: Record<Role, string> = { gestor: 'Gestor', funcionario: 'Funcionário' }

/** Papéis da rota mais específica que os declara. */
function useRouteRoles(): readonly Role[] | undefined {
  const matches = useMatches()
  for (let i = matches.length - 1; i >= 0; i--) {
    const roles = (matches[i]?.handle as RouteHandle | undefined)?.roles
    if (roles) return roles
  }
  return undefined
}

/**
 * Backoffice: só com sessão. Sem sessão, vai para o login e volta depois à
 * mesma página. A guarda por papel só evita mostrar ecrãs inúteis: quem
 * decide é sempre a API.
 */
export function BackofficeLayout() {
  useDocumentTitle()
  const title = usePageTitle()
  const roles = useRouteRoles()
  const session = useSession()
  const location = useLocation()
  const navigate = useNavigate()

  if (session.status === 'loading') {
    return (
      <div className="session-check" role="status">
        A verificar a sessão…
      </div>
    )
  }
  if (session.status === 'anonymous') {
    const next = encodeURIComponent(location.pathname + location.search)
    return <Navigate to={`/entrar?next=${next}`} replace state={{ expired: session.reason === 'expired' }} />
  }

  const { user } = session
  const allowed = !roles || roles.includes(user.role)
  const nav = backofficeNav.filter((item) => item.roles.includes(user.role))

  return (
    <div className="backoffice">
      <nav aria-label="Backoffice" className="sidebar">
        <span className="brand brand--on-green">
          <span className="brand__mark" aria-hidden="true" />
          Loja da Quinta
        </span>
        <ul className="sidebar__list">
          {nav.map((item) => (
            <li key={item.to}>
              <NavLink to={item.to} className="sidebar__link">
                {icons[item.to]}
                {item.title}
              </NavLink>
            </li>
          ))}
        </ul>
        <div className="sidebar__footer">
          <p className="sidebar__user">
            <span className="sidebar__user-name">{user.name}</span>
            <span className="sidebar__user-role">{roleLabel[user.role]}</span>
          </p>
          <ThemeToggle tone="side" />
          <button
            type="button"
            className="sidebar__signout"
            onClick={() => {
              void signOut().then(() => navigate('/entrar', { replace: true }))
            }}
          >
            Terminar sessão
          </button>
        </div>
      </nav>
      <main className="backoffice__main">
        {allowed ? (
          <>
            {title && <h1 className="page-title">{title}</h1>}
            <Outlet />
          </>
        ) : (
          <section aria-labelledby="sem-permissao">
            <h1 id="sem-permissao" className="page-title">
              Sem permissão
            </h1>
            <p>Esta página é só para o gestor. Se precisas dela, fala com o gestor da loja.</p>
          </section>
        )}
      </main>
    </div>
  )
}
