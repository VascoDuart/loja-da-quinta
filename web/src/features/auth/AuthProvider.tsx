import { useQueryClient } from '@tanstack/react-query'
import { type ReactNode, useEffect } from 'react'

import { restore, subscribe, getSession } from './session'

/**
 * Recupera a sessão ao abrir a app e limpa a cache das queries quando a
 * sessão termina, para os dados de um utilizador nunca aparecerem a outro.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()

  useEffect(() => {
    void restore()
    return subscribe(() => {
      if (getSession().status === 'anonymous') queryClient.clear()
    })
  }, [queryClient])

  return children
}
