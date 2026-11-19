import type { ReactNode } from 'react'
import { NavLink, Outlet } from 'react-router'

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
import { backofficeNav } from '../routes'
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

/** Backoffice: barra lateral verde com letras brancas e área de conteúdo. */
export function BackofficeLayout() {
  useDocumentTitle()
  const title = usePageTitle()
  return (
    <div className="backoffice">
      <nav aria-label="Backoffice" className="sidebar">
        <span className="brand brand--on-green">
          <span className="brand__mark" aria-hidden="true" />
          Loja da Quinta
        </span>
        <ul className="sidebar__list">
          {backofficeNav.map((item) => (
            <li key={item.to}>
              <NavLink to={item.to} className="sidebar__link">
                {icons[item.to]}
                {item.title}
              </NavLink>
            </li>
          ))}
        </ul>
        <div className="sidebar__footer">
          <ThemeToggle tone="side" />
        </div>
      </nav>
      <main className="backoffice__main">
        {title && <h1 className="page-title">{title}</h1>}
        <Outlet />
      </main>
    </div>
  )
}
