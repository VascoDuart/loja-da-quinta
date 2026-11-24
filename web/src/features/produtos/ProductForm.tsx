import { type FormEvent, useState } from 'react'

import type { Category, Product, ProductInput, Unit, VatRate } from '../../api/generated/model'
import { ApiError, errorMessage } from '../../api/http'
import { centsToInput, milliToInput, monthShort, parseEuroToCents, parseQuantityToMilli } from '../../shared/lib/format'
import { Alert, Button, Checkbox, Chip, RadioGroup, SelectField, TextField } from '../../shared/ui/ui'

const units = [
  { value: 'kg', label: 'Quilograma (kg)' },
  { value: 'l', label: 'Litro (L)' },
  { value: 'un', label: 'Unidade' },
  { value: 'molho', label: 'Molho' },
  { value: 'duzia', label: 'Dúzia' },
] as const satisfies readonly { value: Unit; label: string }[]

const vatRates = [
  { value: '6', label: '6 %' },
  { value: '13', label: '13 %' },
  { value: '23', label: '23 %' },
] as const

const unitShort: Record<Unit, string> = { kg: 'kg', l: 'L', un: 'unidades', molho: 'molhos', duzia: 'dúzias' }
const unitSingular: Record<Unit, string> = { kg: 'kg', l: 'litro', un: 'unidade', molho: 'molho', duzia: 'dúzia' }

type Props = {
  categories: readonly Category[]
  /** Sem produto: criar. Com produto: editar. */
  product?: Product | undefined
  /** version é a versão com que o formulário abriu (para o If-Match). */
  onSubmit: (input: ProductInput, version: string | undefined) => Promise<unknown>
  onCancel: () => void
  /** Volta a abrir o formulário com a versão atual (depois de um 412). */
  onReload?: (() => void) | undefined
}

/**
 * Formulário de produto. Os valores escritos (preço, limiar) são convertidos
 * em inteiros aqui, na fronteira (ADR-0009); a API recebe cêntimos e
 * milésimos.
 */
export function ProductForm({ categories, product, onSubmit, onCancel, onReload }: Props) {
  // A versão fica fixa enquanto o formulário estiver aberto: se outra pessoa
  // gravar entretanto, gravar daqui dá sempre 412, nunca apaga a alteração
  // dela sem aviso.
  const [baseVersion] = useState(product?.version)
  const [conflict, setConflict] = useState(false)
  const [unit, setUnit] = useState<Unit>(product?.unit ?? 'kg')
  const [vat, setVat] = useState<string>(String(product?.vatRate ?? 6))
  const [months, setMonths] = useState<number[]>(product?.seasonMonths ?? [])
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  function toggleMonth(m: number) {
    setMonths((cur) => (cur.includes(m) ? cur.filter((x) => x !== m) : [...cur, m].sort((a, b) => a - b)))
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    const text = (k: string) => String(form.get(k) ?? '')

    const priceCents = parseEuroToCents(text('price'))
    if (priceCents === null) {
      setError('O preço tem de ser um valor em euros, por exemplo 1,20.')
      return
    }
    const lowStockMilli = parseQuantityToMilli(text('lowStock') || '0', unit)
    if (lowStockMilli === null) {
      setError(
        unit === 'kg' || unit === 'l'
          ? 'O limiar de stock baixo tem de ser um número, com até 3 casas decimais.'
          : 'O limiar de stock baixo tem de ser um número inteiro.',
      )
      return
    }

    setPending(true)
    setError(null)
    setConflict(false)
    try {
      await onSubmit(
        {
          name: text('name'),
          categoryId: text('categoryId'),
          unit,
          priceCents,
          vatRate: Number(vat) as VatRate,
          seasonMonths: months,
          lowStockMilli,
          isPublic: form.get('isPublic') === 'on',
          active: form.get('active') === 'on',
        },
        baseVersion,
      )
    } catch (err) {
      setError(errorMessage(err))
      setConflict(err instanceof ApiError && err.status === 412)
    } finally {
      setPending(false)
    }
  }

  return (
    <section className="product-form" aria-labelledby="product-form-title">
      <h2 id="product-form-title" className="section-title">
        {product ? `Editar ${product.name}` : 'Novo produto'}
      </h2>
      {error && (
        <Alert>
          {error}
          {conflict && onReload && (
            <>
              {' '}
              <Button variant="ghost" onClick={onReload}>
                Carregar a versão atual
              </Button>
            </>
          )}
        </Alert>
      )}
      <form className="product-form__fields" onSubmit={(e) => void submit(e)}>
        <TextField label="Nome" name="name" defaultValue={product?.name} required maxLength={100} autoComplete="off" />
        <SelectField
          label="Categoria"
          name="categoryId"
          defaultValue={product?.categoryId ?? categories[0]?.id}
          options={categories.map((c) => ({ value: c.id, label: c.name }))}
          required
        />
        <div className="product-form__row">
          <SelectField
            label="Unidade de venda"
            name="unit"
            value={unit}
            onChange={(e) => setUnit(e.target.value as Unit)}
            options={units}
          />
          <TextField
            label={`Preço por ${unitSingular[unit]} (€, com IVA)`}
            name="price"
            inputMode="decimal"
            defaultValue={product ? centsToInput(product.priceCents) : ''}
            required
            placeholder="1,20"
          />
        </div>
        <RadioGroup legend="Taxa de IVA" name="vatRate" value={vat} options={vatRates} onChange={setVat} />
        <fieldset className="choice">
          <legend className="field__label">Época</legend>
          <div className="months">
            {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
              <Chip key={m} selected={months.includes(m)} onClick={() => toggleMonth(m)}>
                {monthShort(m)}
              </Chip>
            ))}
          </div>
          <span className="field__hint">Sem meses escolhidos, o produto é de todo o ano.</span>
        </fieldset>
        <TextField
          label={`Stock baixo abaixo de (${unitShort[unit]})`}
          name="lowStock"
          inputMode={unit === 'kg' || unit === 'l' ? 'decimal' : 'numeric'}
          defaultValue={product ? milliToInput(product.lowStockMilli) : '0'}
          hint="O painel avisa quando o stock fica abaixo deste valor."
        />
        <Checkbox
          label="Visível no catálogo público"
          name="isPublic"
          defaultChecked={product?.isPublic ?? true}
          hint="Os visitantes veem o preço e a disponibilidade, nunca as quantidades."
        />
        <Checkbox
          label="Ativo"
          name="active"
          defaultChecked={product?.active ?? true}
          hint="Um produto inativo não aparece no ecrã de venda nem no catálogo."
        />
        <div className="product-form__actions">
          <Button type="submit" disabled={pending}>
            {pending ? 'A guardar…' : product ? 'Guardar' : 'Criar produto'}
          </Button>
          <Button variant="secondary" onClick={onCancel} disabled={pending}>
            Cancelar
          </Button>
        </div>
      </form>
    </section>
  )
}
