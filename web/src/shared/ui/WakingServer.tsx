import { useSyncExternalStore } from 'react'

import { slowRequests } from '../../api/slowRequests'
import './WakingServer.css'

/**
 * Aviso mostrado enquanto um pedido à API demora mais de 3 s, que é o que
 * acontece quando a API do plano gratuito está a acordar (RNF-04).
 * Usa role="status" para os leitores de ecrã o anunciarem sem interromper.
 */
export function WakingServer() {
  const slow = useSyncExternalStore(slowRequests.subscribe, slowRequests.count, slowRequests.count)
  return (
    <div role="status" aria-live="polite" className="waking-server" hidden={slow === 0}>
      {slow > 0 && (
        <>
          <span className="waking-server__spinner" aria-hidden="true" />
          <span>
            <strong>A acordar o servidor…</strong> O primeiro pedido depois de algum tempo parado pode demorar até um
            minuto.
          </span>
        </>
      )}
    </div>
  )
}
