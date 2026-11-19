import { NavLink, Outlet } from 'react-router'

import { backofficeNav } from '../routes'
import { useDocumentTitle, usePageTitle } from '../useDocumentTitle'

/**
 * Layout do backoffice: menu de navegação e área de conteúdo.
 * A posição do menu (lateral, topo ou barra de ícones) depende do design
 * escolhido; a estrutura (nav + main) é a mesma nas três propostas.
 */
export function BackofficeLayout() {
  useDocumentTitle()
  const title = usePageTitle()
  return (
    <div className="layout layout--backoffice">
      <nav aria-label="Backoffice" className="layout__nav">
        <span className="layout__brand">Loja da Quinta</span>
        <ul>
          {backofficeNav.map((item) => (
            <li key={item.to}>
              <NavLink to={item.to}>{item.title}</NavLink>
            </li>
          ))}
        </ul>
      </nav>
      <main className="layout__main">
        {title && <h1>{title}</h1>}
        <Outlet />
      </main>
    </div>
  )
}
