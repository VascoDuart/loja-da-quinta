import { Link } from 'react-router'

import { useDocumentTitle } from './useDocumentTitle'

export function NotFoundPage() {
  useDocumentTitle()
  return (
    <main className="layout__main">
      <h1>Página não encontrada</h1>
      <p>Este endereço não existe.</p>
      <Link to="/">Voltar ao catálogo</Link>
    </main>
  )
}
