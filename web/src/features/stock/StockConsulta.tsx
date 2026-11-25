import { Link } from 'react-router'

import { useGetStockOverview } from '../../api/generated/endpoints/inventory/inventory'
import type { LotStatus, StockOverview, StockProduct } from '../../api/generated/model'
import { errorMessage } from '../../api/http'
import { formatDate } from '../../shared/lib/format'
import { Alert, Button, Pill } from '../../shared/ui/ui'
import { milli, type ProductLot, summarize } from './summary'

// O estado nunca vai só na cor: o texto diz o mesmo (WCAG 1.4.1).
const lotPill: Record<Exclude<LotStatus, 'valido'>, { tone: 'warn' | 'bad'; text: string }> = {
  expira_em_breve: { tone: 'warn', text: 'Expira em breve' },
  fora_de_validade: { tone: 'bad', text: 'Fora de validade' },
}

const lotLine = ({ lot, product }: ProductLot) =>
  `${product.name}: ${milli(lot.quantityLeftMilli, lot.unit)}, validade ${formatDate(lot.expiresOn)}`

/** Os três grupos que pedem atenção, cada um só quando tem alguma coisa. */
function Attention({ overview, isGestor }: { overview: StockOverview; isGestor: boolean }) {
  const { low, expiring, expired } = summarize(overview)
  if (low.length + expiring.length + expired.length === 0) {
    return <p className="stock__lead">Nada a tratar: sem stock baixo, nem lotes a expirar ou fora de validade.</p>
  }
  // Os títulos não dão nome acessível às listas (sem aria-labelledby): um
  // nome com "validade" confundia-se com o campo Validade do formulário.
  return (
    <>
      {expired.length > 0 && (
        <div className="stock__group">
          <h3 className="stock__subtitle">
            <Pill tone="bad">Fora de validade</Pill> Para dar como quebra ({expired.length})
          </h3>
          <p className="stock__lead">
            Já não se vendem nem contam para o stock.{!isGestor && ' Avisa o gestor para registar a quebra.'}
          </p>
          <ul className="stock__list">
            {expired.map((pl) => (
              <li key={pl.lot.id}>
                {lotLine(pl)}
                {isGestor && (
                  <>
                    {' · '}
                    <Link to={`/app/stock/ajustes?produto=${pl.product.productId}&lote=${pl.lot.id}`}>
                      Dar como quebra <span className="stock__hidden">({lotLine(pl)})</span>
                    </Link>
                  </>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
      {expiring.length > 0 && (
        <div className="stock__group">
          <h3 className="stock__subtitle">
            <Pill tone="warn">A expirar</Pill> Até {formatDate(overview.expiringUntil)} ({expiring.length})
          </h3>
          <ul className="stock__list">
            {expiring.map((pl) => (
              <li key={pl.lot.id}>{lotLine(pl)}</li>
            ))}
          </ul>
        </div>
      )}
      {low.length > 0 && (
        <div className="stock__group">
          <h3 className="stock__subtitle">
            <Pill tone="warn">Stock baixo</Pill> Abaixo do limiar ({low.length})
          </h3>
          <ul className="stock__list">
            {low.map((p) => (
              <li key={p.productId}>
                {p.name}: {milli(p.sellableMilli, p.unit)} para vender (limiar: {milli(p.lowStockMilli, p.unit)})
              </li>
            ))}
          </ul>
        </div>
      )}
    </>
  )
}

function ProductRow({ product }: { product: StockProduct }) {
  const { unit, lots } = product
  return (
    <li className="stock__product">
      <p className="stock__name">
        <strong>{product.name}</strong>
        {product.low && <Pill tone="warn">Stock baixo</Pill>}
        {product.sellableMilli === 0 && <Pill tone="bad">Sem stock</Pill>}
        {!product.active && <Pill>Desativado</Pill>}
      </p>
      <p className="stock__lead">Para vender: {milli(product.sellableMilli, unit)}</p>
      {lots.length > 0 && (
        <ul className="stock__list">
          {lots.map((lot) => (
            <li key={lot.id}>
              Validade {formatDate(lot.expiresOn)} · restam {milli(lot.quantityLeftMilli, lot.unit)}
              {lot.status !== 'valido' && (
                <>
                  {' '}
                  <Pill tone={lotPill[lot.status].tone}>{lotPill[lot.status].text}</Pill>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </li>
  )
}

/** Consulta do stock por produto e por lote (UC-09, RF-11). */
export function StockConsulta({ isGestor }: { isGestor: boolean }) {
  const query = useGetStockOverview()
  const overview = query.data?.status === 200 ? query.data.data : undefined

  return (
    <section className="stock__card" aria-labelledby="consulta-titulo">
      <h2 id="consulta-titulo" className="stock__title">
        Stock atual
      </h2>
      {query.isPending && <p role="status">A carregar o stock…</p>}
      {query.isError && (
        <>
          <Alert>{errorMessage(query.error)}</Alert>
          <Button variant="secondary" onClick={() => void query.refetch()}>
            Tentar de novo
          </Button>
        </>
      )}
      {overview?.products.length === 0 && <p className="stock__lead">Ainda não há stock para mostrar.</p>}
      {overview && overview.products.length > 0 && (
        <>
          <Attention overview={overview} isGestor={isGestor} />
          <h3 className="stock__subtitle">Por produto e por lote</h3>
          <ul className="stock__products">
            {overview.products.map((p) => (
              <ProductRow key={p.productId} product={p} />
            ))}
          </ul>
        </>
      )}
    </section>
  )
}
