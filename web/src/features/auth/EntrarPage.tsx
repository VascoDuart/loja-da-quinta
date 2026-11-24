import { type FormEvent, useState } from 'react'
import { Navigate, useLocation, useSearchParams } from 'react-router'

import { ApiError, errorMessage } from '../../api/http'
import { Alert, Button, TextField } from '../../shared/ui/ui'
import './EntrarPage.css'
import { safeNext } from './redirect'
import { signIn, useSession } from './session'

function loginErrorMessage(err: unknown): string {
  // O 401 e o 429 do login trazem uma mensagem escrita para pessoas.
  if (err instanceof ApiError && (err.status === 401 || err.status === 429) && err.problem?.detail) {
    return err.problem.detail
  }
  return errorMessage(err)
}

/** Início de sessão do backoffice (UC-01). */
export default function EntrarPage() {
  const session = useSession()
  const [params] = useSearchParams()
  const location = useLocation()
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)
  const next = safeNext(params.get('next'))
  const expired = (location.state as { expired?: boolean } | null)?.expired === true

  if (session.status === 'authenticated') return <Navigate to={next} replace />

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    setPending(true)
    setError(null)
    try {
      await signIn(String(form.get('email') ?? ''), String(form.get('password') ?? ''))
    } catch (err) {
      setError(loginErrorMessage(err))
    } finally {
      setPending(false)
    }
  }

  return (
    <section className="entrar" aria-labelledby="entrar-titulo">
      <h1 id="entrar-titulo" className="page-title">
        Entrar no backoffice
      </h1>
      <p className="entrar__lead">Para quem gere a loja ou vende ao balcão.</p>

      {expired && !error && <Alert tone="info">A sessão terminou. Inicia sessão de novo.</Alert>}
      {error && <Alert>{error}</Alert>}

      <form className="entrar__form" onSubmit={(e) => void onSubmit(e)}>
        <TextField label="E-mail" name="email" type="email" autoComplete="username" required maxLength={254} />
        <TextField
          label="Palavra-passe"
          name="password"
          type="password"
          autoComplete="current-password"
          required
          maxLength={128}
        />
        <Button type="submit" size="lg" disabled={pending}>
          {pending ? 'A entrar…' : 'Entrar'}
        </Button>
      </form>
    </section>
  )
}
