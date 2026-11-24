import { type ChangeEvent, useEffect, useState } from 'react'

import type { Product } from '../../api/generated/model'
import { errorMessage } from '../../api/http'
import { productImageUrl } from '../../api/productImage'
import { Alert, Button } from '../../shared/ui/ui'

const MAX_BYTES = 5 * 1024 * 1024
const ACCEPT = 'image/jpeg,image/png,image/webp'

type Props = {
  product: Product
  onUpload: (file: Blob) => Promise<unknown>
  onRemove: () => Promise<unknown>
}

/**
 * Imagem do produto: mostra a atual, pré-visualiza a escolhida e envia-a. A
 * API verifica o formato pelo conteúdo e reprocessa a imagem; aqui só se
 * evita enviar um ficheiro que de certeza vai ser recusado.
 */
export function ProductImageEditor({ product, onUpload, onRemove }: Props) {
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  // Liberta a memória da pré-visualização quando muda ou o editor fecha.
  useEffect(() => {
    if (!preview) return
    return () => URL.revokeObjectURL(preview)
  }, [preview])

  function choose(e: ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0] ?? null
    setError(null)
    if (f && f.size > MAX_BYTES) {
      setError('A imagem tem mais de 5 MB. Escolhe uma mais pequena.')
      e.target.value = ''
      return
    }
    setFile(f)
    setPreview(f ? URL.createObjectURL(f) : null)
  }

  async function run(action: () => Promise<unknown>) {
    setPending(true)
    setError(null)
    try {
      await action()
      setFile(null)
      setPreview(null)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setPending(false)
    }
  }

  const current = product.image ? productImageUrl(product.id, product.image.version, 'large') : null
  const shown = preview ?? current

  return (
    <section className="image-editor" aria-labelledby="image-editor-title">
      <h3 id="image-editor-title" className="image-editor__title">
        Imagem
      </h3>
      {error && <Alert>{error}</Alert>}
      <div className="image-editor__body">
        <div className="image-editor__frame">
          {shown ? (
            <img
              src={shown}
              alt={preview ? `Pré-visualização da nova imagem de ${product.name}` : product.name}
              className="image-editor__img"
            />
          ) : (
            <span className="image-editor__empty">Sem imagem</span>
          )}
        </div>
        <div className="image-editor__controls">
          <label className="field">
            <span className="field__label">Escolher imagem (JPEG, PNG ou WebP, até 5 MB)</span>
            <input type="file" accept={ACCEPT} onChange={choose} disabled={pending} />
          </label>
          <div className="product-form__actions">
            {file && (
              <Button disabled={pending} onClick={() => void run(() => onUpload(file))}>
                {pending ? 'A enviar…' : 'Enviar imagem'}
              </Button>
            )}
            {product.image && !file && (
              <Button variant="secondary" disabled={pending} onClick={() => void run(onRemove)}>
                Remover imagem
              </Button>
            )}
          </div>
          <span className="field__hint">
            A imagem é ajustada para 800 px e as informações da foto (como a localização) são apagadas.
          </span>
        </div>
      </div>
    </section>
  )
}
