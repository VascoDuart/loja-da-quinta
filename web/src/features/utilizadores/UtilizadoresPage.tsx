import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import {
  getListUsersQueryKey,
  useCreateUser,
  useListUsers,
  useUpdateUser,
} from '../../api/generated/endpoints/users/users'
import type { ManagedUser } from '../../api/generated/model'
import { errorMessage } from '../../api/http'
import { roleLabel } from '../../app/routes'
import { formatDate } from '../../shared/lib/format'
import { Alert, Button, Pill } from '../../shared/ui/ui'
import { useSession } from '../auth/session'
import { UserForm } from './UserForm'
import './UtilizadoresPage.css'

type Editing = { mode: 'create' } | { mode: 'edit'; user: ManagedUser } | null

/** Gestão de utilizadores pelo gestor (UC-02). */
export default function UtilizadoresPage() {
  const session = useSession()
  const me = session.status === 'authenticated' ? session.user.id : null
  const queryClient = useQueryClient()
  const users = useListUsers()
  // O apiFetch lança nas respostas de erro: aqui só chega o 200.
  const list = users.data?.status === 200 ? users.data.data : undefined
  const create = useCreateUser()
  const update = useUpdateUser()
  const [editing, setEditing] = useState<Editing>(null)
  const [notice, setNotice] = useState<string | null>(null)

  async function done(message: string) {
    await queryClient.invalidateQueries({ queryKey: getListUsersQueryKey() })
    setEditing(null)
    setNotice(message)
  }

  return (
    <div className="utilizadores">
      <div className="utilizadores__toolbar">
        <p className="utilizadores__lead">Quem pode entrar no backoffice e com que papel.</p>
        {!editing && (
          <Button
            onClick={() => {
              setNotice(null)
              setEditing({ mode: 'create' })
            }}
          >
            Novo utilizador
          </Button>
        )}
      </div>

      {notice && <Alert tone="info">{notice}</Alert>}

      {editing?.mode === 'create' && (
        <UserForm
          mode="create"
          onCancel={() => setEditing(null)}
          onSubmit={async (v) => {
            await create.mutateAsync({ data: v })
            await done(`Utilizador ${v.email.trim().toLowerCase()} criado.`)
          }}
        />
      )}
      {editing?.mode === 'edit' && (
        <UserForm
          key={editing.user.id}
          mode="edit"
          user={editing.user}
          isSelf={editing.user.id === me}
          onCancel={() => setEditing(null)}
          onSubmit={async (changes) => {
            await update.mutateAsync({ id: editing.user.id, data: changes })
            await done(`Alterações a ${editing.user.name} guardadas.`)
          }}
        />
      )}

      {users.isPending && <p role="status">A carregar os utilizadores…</p>}
      {users.isError && <Alert>{errorMessage(users.error)}</Alert>}
      {list && (
        <ul className="user-list" aria-label="Utilizadores">
          {list.map((u) => (
            <li key={u.id} className={`user-card${u.active ? '' : ' user-card--inactive'}`}>
              <div className="user-card__who">
                <span className="user-card__name">
                  {u.name}
                  {u.id === me && <span className="user-card__me"> (tu)</span>}
                </span>
                <span className="user-card__email">{u.email}</span>
                <span className="user-card__since">Desde {formatDate(u.createdAt)}</span>
              </div>
              <div className="user-card__tags">
                <Pill tone={u.role === 'gestor' ? 'ok' : 'neutral'}>{roleLabel[u.role]}</Pill>
                <Pill tone={u.active ? 'ok' : 'bad'}>{u.active ? 'Ativo' : 'Desativado'}</Pill>
              </div>
              <Button
                variant="secondary"
                aria-label={`Editar ${u.name}`}
                onClick={() => {
                  setNotice(null)
                  setEditing({ mode: 'edit', user: u })
                }}
              >
                Editar
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
