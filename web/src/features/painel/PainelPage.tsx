import { useGetReadiness } from '../../api/generated/endpoints/health/health'
import { errorMessage } from '../../api/http'
import { Button } from '../../shared/ui/ui'
import './PainelPage.css'

/**
 * Resumo do dia no backoffice. Por agora mostra só o estado do servidor,
 * que é o primeiro uso real do cliente gerado pela OpenAPI.
 */
export default function PainelPage() {
  const readiness = useGetReadiness()

  let state: 'loading' | 'ok' | 'error' = 'loading'
  if (readiness.isSuccess) state = 'ok'
  else if (readiness.isError) state = 'error'

  return (
    <section className="card" aria-labelledby="estado-servidor">
      <h2 id="estado-servidor" className="card__title">
        Estado do servidor
      </h2>
      {state === 'loading' && <p className="card__text">A verificar…</p>}
      {state === 'ok' && <p className="card__text status status--ok">Servidor pronto. A base de dados responde.</p>}
      {state === 'error' && readiness.error && (
        <>
          <p className="card__text status status--bad">{errorMessage(readiness.error)}</p>
          {readiness.error.requestId && <p className="card__meta">Referência: {readiness.error.requestId}</p>}
          <Button variant="secondary" onClick={() => void readiness.refetch()}>
            Tentar de novo
          </Button>
        </>
      )}
    </section>
  )
}
