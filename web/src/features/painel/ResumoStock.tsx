import { Link } from 'react-router'

import { useGetStockOverview } from '../../api/generated/endpoints/inventory/inventory'
import type { StockOverview } from '../../api/generated/model'
import { errorMessage } from '../../api/http'
import { Button, Pill } from '../../shared/ui/ui'
import { names, summarize } from '../stock/summary'

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

function Summary({ overview }: { overview: StockOverview }) {
  const { low, expiring, expired } = summarize(overview)
  if (low.length + expiring.length + expired.length === 0) {
    return <p className="card__text status status--ok">Nada a tratar no stock.</p>
  }
  // Cor sempre com texto (WCAG 1.4.1): a etiqueta diz o que a cor diz.
  return (
    <ul className="card__list">
      {expired.length > 0 && (
        <li>
          <Pill tone="bad">Fora de validade</Pill>{' '}
          {plural(expired.length, 'lote para dar como quebra', 'lotes para dar como quebra')}:{' '}
          {names(expired.map((l) => l.product))}
        </li>
      )}
      {expiring.length > 0 && (
        <li>
          <Pill tone="warn">A expirar</Pill> {plural(expiring.length, 'lote expira', 'lotes expiram')} nos próximos 3
          dias: {names(expiring.map((l) => l.product))}
        </li>
      )}
      {low.length > 0 && (
        <li>
          <Pill tone="warn">Stock baixo</Pill> {plural(low.length, 'produto', 'produtos')}: {names(low)}
        </li>
      )}
    </ul>
  )
}

/** Resumo do stock no painel (UC-09): o que pede atenção hoje. */
export function ResumoStock() {
  const query = useGetStockOverview()
  const overview = query.data?.status === 200 ? query.data.data : undefined

  return (
    <section className="card" aria-labelledby="resumo-stock">
      <h2 id="resumo-stock" className="card__title">
        Stock
      </h2>
      {query.isPending && <p className="card__text">A verificar o stock…</p>}
      {query.isError && (
        <>
          <p className="card__text status status--bad">Não foi possível ver o stock. {errorMessage(query.error)}</p>
          <Button variant="secondary" onClick={() => void query.refetch()}>
            Tentar de novo
          </Button>
        </>
      )}
      {overview && <Summary overview={overview} />}
      <Link to="/app/stock">Ver o stock</Link>
    </section>
  )
}
