import { Link } from 'react-router'

import { useDocumentTitle } from './useDocumentTitle'

export function NotFoundPage() {
  useDocumentTitle()
  return (
    <main className="public__main">
      <h1 className="page-title">Página não encontrada</h1>
      <p>Este endereço não existe.</p>
      <Link to="/">Voltar ao catálogo</Link>
    </main>
  )
}
