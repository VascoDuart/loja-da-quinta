import { Link, Outlet } from 'react-router'

import { useDocumentTitle } from '../useDocumentTitle'

/**
 * Layout da área pública (catálogo). Só estrutura: o aspeto visual chega
 * com o design system escolhido.
 */
export function PublicLayout() {
  useDocumentTitle()
  return (
    <div className="layout layout--public">
      <header className="layout__header">
        <Link to="/" className="layout__brand">
          Loja da Quinta
        </Link>
        <nav aria-label="Principal">
          <Link to="/entrar">Entrar</Link>
        </nav>
      </header>
      <main className="layout__main">
        <Outlet />
      </main>
      <footer className="layout__footer">
        <small>Encomendas online em breve.</small>
      </footer>
    </div>
  )
}
