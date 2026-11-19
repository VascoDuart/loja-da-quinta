/**
 * Conta os pedidos à API que estão a demorar mais do que o normal.
 * O aviso "a acordar o servidor" aparece enquanto houver pelo menos um.
 *
 * A API no plano gratuito da Render adormece após 15 min sem pedidos e o
 * primeiro pedido demora a responder (ADR-0007, RNF-04).
 */
type Listener = () => void

function createSlowRequests() {
  let slow = 0
  const listeners = new Set<Listener>()
  const emit = () => listeners.forEach((l) => l())

  return {
    /** Começa a contar um pedido. Devolve a função a chamar quando acaba. */
    track(afterMs: number): () => void {
      let isSlow = false
      const timer = setTimeout(() => {
        isSlow = true
        slow++
        emit()
      }, afterMs)
      let finished = false
      return () => {
        if (finished) return
        finished = true
        clearTimeout(timer)
        if (isSlow) {
          slow--
          emit()
        }
      }
    },
    count: () => slow,
    subscribe(listener: Listener): () => void {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
  }
}

export const slowRequests = createSlowRequests()
