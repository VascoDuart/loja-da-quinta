import { type FormEvent, useState } from 'react'

import type { ManagedUser, Role, UserUpdate } from '../../api/generated/model'
import { errorMessage } from '../../api/http'
import { Alert, Button, Checkbox, RadioGroup, TextField } from '../../shared/ui/ui'

const roles = [
  { value: 'funcionario', label: 'Funcionário' },
  { value: 'gestor', label: 'Gestor' },
] as const satisfies readonly { value: Role; label: string }[]

export type NewUserValues = { email: string; name: string; role: Role; password: string }

type Props =
  | { mode: 'create'; onSubmit: (v: NewUserValues) => Promise<unknown>; onCancel: () => void }
  | {
      mode: 'edit'
      user: ManagedUser
      /** É a conta de quem está a editar: não pode desativar-se nem despromover-se. */
      isSelf: boolean
      onSubmit: (changes: UserUpdate) => Promise<unknown>
      onCancel: () => void
    }

/**
 * Formulário de criar ou editar um utilizador. Na edição só envia o que
 * mudou, para não terminar sessões sem necessidade (a API termina-as ao
 * mudar o papel, desativar ou definir palavra-passe nova).
 */
export function UserForm(props: Props) {
  const editing = props.mode === 'edit' ? props.user : null
  const isSelf = props.mode === 'edit' && props.isSelf
  const [role, setRole] = useState<Role>(editing?.role ?? 'funcionario')
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    const text = (k: string) => String(form.get(k) ?? '')
    setPending(true)
    setError(null)
    try {
      if (props.mode === 'create') {
        await props.onSubmit({ email: text('email'), name: text('name'), role, password: text('password') })
      } else {
        const changes: UserUpdate = {}
        if (text('name').trim() !== props.user.name) changes.name = text('name')
        if (role !== props.user.role) changes.role = role
        const active = form.get('active') === 'on'
        if (active !== props.user.active) changes.active = active
        if (text('password') !== '') changes.password = text('password')
        if (Object.keys(changes).length === 0) {
          props.onCancel()
          return
        }
        await props.onSubmit(changes)
      }
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setPending(false)
    }
  }

  const title = editing ? `Editar ${editing.name}` : 'Novo utilizador'
  return (
    <section className="user-form" aria-labelledby="user-form-title">
      <h2 id="user-form-title" className="user-form__title">
        {title}
      </h2>
      {error && <Alert>{error}</Alert>}
      <form className="user-form__fields" onSubmit={(e) => void onSubmit(e)}>
        {editing ? (
          <p className="user-form__email">{editing.email}</p>
        ) : (
          <TextField label="E-mail" name="email" type="email" autoComplete="off" required maxLength={254} />
        )}
        <TextField label="Nome" name="name" defaultValue={editing?.name} required maxLength={100} autoComplete="off" />
        <RadioGroup
          legend="Papel"
          name="role"
          value={role}
          options={roles}
          onChange={setRole}
          disabled={isSelf}
          hint={isSelf ? 'Não podes tirar o papel de gestor à tua própria conta.' : undefined}
        />
        {editing && (
          <Checkbox
            label="Conta ativa"
            name="active"
            defaultChecked={editing.active}
            disabled={isSelf}
            hint={
              isSelf
                ? 'Não podes desativar a tua própria conta.'
                : 'Uma conta desativada não entra, e as sessões abertas terminam logo.'
            }
          />
        )}
        <TextField
          label={editing ? 'Palavra-passe nova' : 'Palavra-passe'}
          name="password"
          type="password"
          autoComplete="new-password"
          required={!editing}
          minLength={12}
          maxLength={128}
          hint={
            editing
              ? 'Deixa em branco para não mudar. Se mudares, as sessões abertas terminam.'
              : 'Pelo menos 12 caracteres. Uma frase é mais fácil de lembrar.'
          }
        />
        <div className="user-form__actions">
          <Button type="submit" disabled={pending}>
            {pending ? 'A guardar…' : editing ? 'Guardar' : 'Criar utilizador'}
          </Button>
          <Button variant="secondary" onClick={props.onCancel} disabled={pending}>
            Cancelar
          </Button>
        </div>
      </form>
    </section>
  )
}
