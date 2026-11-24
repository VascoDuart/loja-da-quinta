import { type FormEvent, useState } from 'react'

import type { Category } from '../../api/generated/model'
import { errorMessage } from '../../api/http'
import { Alert, Button, TextField } from '../../shared/ui/ui'

type Props = {
  categories: readonly Category[]
  onCreate: (name: string) => Promise<unknown>
  onRename: (id: string, name: string) => Promise<unknown>
  onDelete: (id: string) => Promise<unknown>
}

/** Categorias: criar, mudar o nome e apagar (só as que não têm produtos). */
export function CategoriesPanel({ categories, onCreate, onRename, onDelete }: Props) {
  const [renaming, setRenaming] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  async function run(action: () => Promise<unknown>) {
    setPending(true)
    setError(null)
    try {
      await action()
      return true
    } catch (err) {
      setError(errorMessage(err))
      return false
    } finally {
      setPending(false)
    }
  }

  async function onAdd(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = e.currentTarget
    const name = String(new FormData(form).get('name') ?? '')
    if (await run(() => onCreate(name))) form.reset()
  }

  return (
    <section className="categories" aria-labelledby="categorias-titulo">
      <h2 id="categorias-titulo" className="section-title">
        Categorias
      </h2>
      {error && <Alert>{error}</Alert>}
      <ul className="categories__list">
        {categories.map((c) => (
          <li key={c.id} className="categories__item">
            {renaming === c.id ? (
              <form
                className="categories__rename"
                onSubmit={(e) => {
                  e.preventDefault()
                  const name = String(new FormData(e.currentTarget).get('name') ?? '')
                  void run(() => onRename(c.id, name)).then((ok) => ok && setRenaming(null))
                }}
              >
                <TextField label={`Novo nome de ${c.name}`} name="name" defaultValue={c.name} required maxLength={60} />
                <Button type="submit" disabled={pending}>
                  Guardar
                </Button>
                <Button variant="secondary" onClick={() => setRenaming(null)}>
                  Cancelar
                </Button>
              </form>
            ) : (
              <>
                <span className="categories__name">
                  {c.name}{' '}
                  <span className="categories__count">
                    ({c.productCount} {c.productCount === 1 ? 'produto' : 'produtos'})
                  </span>
                </span>
                <span className="categories__actions">
                  <Button variant="ghost" aria-label={`Mudar o nome de ${c.name}`} onClick={() => setRenaming(c.id)}>
                    Mudar o nome
                  </Button>
                  {c.productCount === 0 && (
                    <Button
                      variant="ghost"
                      aria-label={`Apagar ${c.name}`}
                      disabled={pending}
                      onClick={() => void run(() => onDelete(c.id))}
                    >
                      Apagar
                    </Button>
                  )}
                </span>
              </>
            )}
          </li>
        ))}
      </ul>
      <form className="categories__add" onSubmit={(e) => void onAdd(e)}>
        <TextField label="Nova categoria" name="name" required maxLength={60} placeholder="Ex.: Fruta" />
        <Button type="submit" variant="secondary" disabled={pending}>
          Adicionar
        </Button>
      </form>
    </section>
  )
}
