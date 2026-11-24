import { useState } from 'react'
import { Link } from 'react-router'

import { errorMessage } from '../../api/http'
import { formatSeason, formatUnitPrice, lisbonMonth } from '../../shared/lib/format'
import { Alert, Chip, SeasonTip, StockPill } from '../../shared/ui/ui'
import './CatalogoPage.css'
import { Thumb } from './Thumb'
import { usePublicCatalog } from './usePublicCatalog'

/** Catálogo público (UC-10): produtos da época, sem login. */
export default function CatalogoPage() {
  const { query, items } = usePublicCatalog()
  const [category, setCategory] = useState<string | null>(null)

  const categories = items ? [...new Set(items.map((i) => i.categoryName))] : []
  // Uma categoria escolhida que entretanto deixou de existir volta a "Todas".
  const active = category !== null && categories.includes(category) ? category : null
  const shown = items?.filter((i) => active === null || i.categoryName === active)

  const month = lisbonMonth()
  const inSeason = (items ?? []).filter((i) => i.seasonMonths.includes(month)).slice(0, 3)

  return (
    <div className="catalogo">
      <h1 className="page-title">Produtos da época</h1>

      {inSeason.length > 0 && <SeasonTip title="Da época este mês">{inSeason.map((i) => i.name).join(', ')}</SeasonTip>}

      {query.isPending && (
        <p role="status" className="catalogo__state">
          A carregar os produtos…
        </p>
      )}
      {query.isError && <Alert>{errorMessage(query.error)}</Alert>}

      {items && items.length === 0 && <p className="catalogo__state">Ainda não há produtos no catálogo.</p>}

      {items && items.length > 0 && (
        <>
          <div role="group" aria-label="Filtrar por categoria" className="catalogo__filters">
            <Chip selected={active === null} onClick={() => setCategory(null)}>
              Todas
            </Chip>
            {categories.map((name) => (
              <Chip key={name} selected={active === name} onClick={() => setCategory(name)}>
                {name}
              </Chip>
            ))}
          </div>

          <ul className="catalogo__grid" aria-label="Produtos">
            {shown?.map((item) => (
              <li key={item.id} className="catalogo__item">
                <Link to={`/produtos/${item.id}`} className="catalog-card">
                  <Thumb item={item} size="thumb" />
                  <span className="catalog-card__body">
                    <span className="catalog-card__name">{item.name}</span>
                    <span className="catalog-card__price">{formatUnitPrice(item.priceCents, item.unit)}</span>
                    <span className="catalog-card__season">Época: {formatSeason(item.seasonMonths)}</span>
                  </span>
                  <StockPill level={item.availability} />
                </Link>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  )
}
