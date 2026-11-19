import { Link, Outlet } from 'react-router'

import { ThemeToggle } from '../../shared/theme/ThemeToggle'
import { useDocumentTitle } from '../useDocumentTitle'

/** Área pública: cabeçalho verde, conteúdo numa coluna legível em telemóvel. */
export function PublicLayout() {
  useDocumentTitle()
  return (
    <div className="public">
      <header className="public__header">
        <Link to="/" className="brand brand--on-green">
          <span className="brand__mark" aria-hidden="true" />
          Loja da Quinta
        </Link>
        <nav aria-label="Principal" className="public__nav">
          <Link to="/entrar" className="public__link">
            Entrar
          </Link>
          <ThemeToggle tone="side" compact />
        </nav>
      </header>
      <main className="public__main">
        <Outlet />
      </main>
      <footer className="public__footer">
        <small>Encomendas online em breve.</small>
      </footer>
    </div>
  )
}
