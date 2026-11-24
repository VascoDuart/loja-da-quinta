import { Link, useParams } from 'react-router'

import { errorMessage } from '../../api/http'
import { formatSeason, formatUnitPrice } from '../../shared/lib/format'
import { Alert, StockPill } from '../../shared/ui/ui'
import './CatalogoPage.css'
import { Thumb } from './Thumb'
import { usePublicCatalog } from './usePublicCatalog'

/** Detalhe de um produto do catálogo (UC-10). Usa a mesma lista (e a mesma cache) do catálogo. */
export default function ProdutoPage() {
  const { id } = useParams()
  const { query, items } = usePublicCatalog()
  const item = items?.find((i) => i.id === id)

  return (
    <div className="produto">
      <Link to="/" className="produto__back">
        ← Voltar ao catálogo
      </Link>

      {query.isPending && (
        <p role="status" className="catalogo__state">
          A carregar o produto…
        </p>
      )}
      {query.isError && <Alert>{errorMessage(query.error)}</Alert>}

      {items && !item && (
        <>
          <h1 className="page-title">Produto não encontrado</h1>
          <p>Este produto não existe ou já não está no catálogo.</p>
        </>
      )}

      {item && (
        <article className="produto__detail" aria-labelledby="produto-nome">
          <Thumb item={item} size="large" />
          <div className="produto__info">
            <p className="produto__category">{item.categoryName}</p>
            <h1 id="produto-nome" className="page-title">
              {item.name}
            </h1>
            <p className="produto__price">{formatUnitPrice(item.priceCents, item.unit)}</p>
            <p>
              <StockPill level={item.availability} />
            </p>
            <dl className="produto__facts">
              <dt>Época</dt>
              <dd>{formatSeason(item.seasonMonths)}</dd>
            </dl>
          </div>
        </article>
      )}
    </div>
  )
}
