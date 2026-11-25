import { useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'

import { useListProducts } from '../../api/generated/endpoints/catalog/catalog'
import {
  createStockAdjustment,
  getGetStockOverviewQueryKey,
  useListStockLots,
} from '../../api/generated/endpoints/inventory/inventory'
import type { Product, StockAdjustment, StockLot } from '../../api/generated/model'
import { ApiError, errorMessage } from '../../api/http'
import { formatDate, formatQuantity, lisbonToday, parseQuantityToMilli } from '../../shared/lib/format'
import { Alert, Button, RadioGroup, SelectField, TextField } from '../../shared/ui/ui'
import './StockPage.css'

/** O que a pessoa escolhe: a quebra e o ajuste para menos tiram stock do lote. */
type Kind = 'quebra' | 'menos' | 'mais'

const kinds: readonly { value: Kind; label: string }[] = [
  { value: 'quebra', label: 'Quebra' },
  { value: 'menos', label: 'Ajuste para menos' },
  { value: 'mais', label: 'Ajuste para mais' },
]

const kindHint: Record<Kind, string> = {
  quebra: 'Stock que se estragou, partiu ou perdeu: sai do lote.',
  menos: 'A contagem deu menos do que o registado.',
  mais: 'A contagem deu mais do que o registado.',
}

const unitShort = { kg: 'kg', l: 'L', un: 'unidades', molho: 'molhos', duzia: 'dúzias' } as const

const isDecimal = (p: Product) => p.unit === 'kg' || p.unit === 'l'
const qty = (milli: number, p: Product) => formatQuantity(milli / 1000, p.unit)

/** "Validade 02/12/2026 · restam 8,500 kg (entraram 10,000 kg a 24/11/2026)" */
function lotLabel(lot: StockLot, p: Product, today: string): string {
  const expired = lot.expiresOn < today ? ' · fora de validade' : ''
  return `Validade ${formatDate(lot.expiresOn)}${expired} · restam ${qty(lot.quantityLeftMilli, p)} (entraram ${qty(
    lot.quantityReceivedMilli,
    p,
  )} a ${formatDate(lot.harvestedOn)})`
}

type Done = { result: StockAdjustment; product: Product }

/** "Quebra registada: 1,500 kg de Laranja. O lote fica com 8,500 kg." */
function describe({ result, product }: Done): string {
  const { movement, lot } = result
  const amount = qty(Math.abs(movement.quantityMilli), product)
  const what =
    movement.type === 'quebra'
      ? `Quebra registada: ${amount}`
      : `Ajuste registado: ${movement.quantityMilli > 0 ? 'mais' : 'menos'} ${amount}`
  return `${what} de ${product.name}. O lote fica com ${qty(lot.quantityLeftMilli, product)}.`
}

/**
 * Ajustes e quebras por lote (RF-06), só para o gestor. A API bloqueia o
 * lote e recusa (409) uma saída maior do que o que resta, mesmo que outra
 * pessoa tenha mexido no lote entretanto; aqui só se evitam pedidos inúteis.
 */
export default function AjustesPage() {
  const products = useListProducts()
  const all = products.data?.status === 200 ? products.data.data : undefined

  const [today] = useState(lisbonToday)
  const queryClient = useQueryClient()
  // A consulta do stock liga para aqui com o produto e o lote já escolhidos
  // (?produto=…&lote=…). Um valor que não exista fica no primeiro da lista.
  const [params] = useSearchParams()
  const [productId, setProductId] = useState(() => params.get('produto') ?? '')
  const product = all?.find((p) => p.id === productId) ?? all?.[0]

  const lots = useListStockLots({ productId: product?.id ?? '' }, { query: { enabled: !!product } })
  const lotList = lots.data?.status === 200 ? lots.data.data : undefined
  const [lotId, setLotId] = useState(() => params.get('lote') ?? '')
  const lot = lotList?.find((l) => l.id === lotId) ?? lotList?.[0]

  const [kind, setKind] = useState<Kind>('quebra')
  const [quantity, setQuantity] = useState('')
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState<Done | null>(null)
  const [pending, setPending] = useState(false)
  // Trava logo um segundo envio (duplo toque, Enter repetido): cada envio
  // é um movimento novo, não se repete por engano.
  const sending = useRef(false)

  function validate(): { milli: number } | string {
    if (!product || !lot) return 'Escolhe o lote.'
    const milli = parseQuantityToMilli(quantity, product.unit)
    if (milli === null) {
      return isDecimal(product)
        ? 'A quantidade tem de ser um número, com até 3 casas decimais (por exemplo 1,5).'
        : 'A quantidade tem de ser um número inteiro.'
    }
    if (milli <= 0) return 'A quantidade tem de ser maior do que 0.'
    if (kind !== 'mais' && milli > lot.quantityLeftMilli) {
      return `Este lote só tem ${qty(lot.quantityLeftMilli, product)}: não se pode tirar mais do que isso.`
    }
    if (!reason.trim()) return 'Indica o motivo.'
    return { milli: kind === 'mais' ? milli : -milli }
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (sending.current || !product || !lot) return
    const checked = validate()
    if (typeof checked === 'string') {
      setError(checked)
      return
    }

    sending.current = true
    setPending(true)
    setError(null)
    try {
      const res = await createStockAdjustment({
        lotId: lot.id,
        type: kind === 'quebra' ? 'quebra' : 'ajuste',
        quantityMilli: checked.milli,
        reason: reason.trim(),
      })
      if (res.status === 201) {
        setDone({ result: res.data, product })
        setQuantity('')
        setReason('')
        void lots.refetch()
        void queryClient.invalidateQueries({ queryKey: getGetStockOverviewQueryKey() })
      }
    } catch (err) {
      setError(errorMessage(err))
      // 409: outra pessoa mexeu no lote; mostra-se o valor atual.
      if (err instanceof ApiError && err.status === 409) void lots.refetch()
    } finally {
      sending.current = false
      setPending(false)
    }
  }

  const unit = product ? unitShort[product.unit] : ''

  return (
    <div className="stock">
      <p className="stock__lead">
        <Link to="/app/stock">Voltar às entradas de stock</Link>
      </p>
      <section className="stock__card" aria-labelledby="ajuste-titulo">
        <h2 id="ajuste-titulo" className="stock__title">
          Registar um ajuste ou uma quebra
        </h2>
        <p className="stock__lead">
          Cada ajuste ou quebra fica no histórico do lote, com o motivo e quem o registou. O stock de um lote nunca fica
          negativo.
        </p>

        {products.isPending && <p role="status">A carregar os produtos…</p>}
        {products.isError && <Alert>{errorMessage(products.error)}</Alert>}
        {all?.length === 0 && <p>Ainda não há produtos.</p>}

        {done && !error && <Alert tone="info">{describe(done)}</Alert>}
        {error && <Alert>{error}</Alert>}

        {product && (
          <form className="stock__form" noValidate onSubmit={(e) => void submit(e)}>
            <SelectField
              label="Produto"
              name="productId"
              value={product.id}
              onChange={(e) => {
                setProductId(e.target.value)
                setLotId('')
                setError(null)
              }}
              options={all?.map((p) => ({ value: p.id, label: p.active ? p.name : `${p.name} (desativado)` })) ?? []}
            />

            {lots.isPending && <p role="status">A carregar os lotes…</p>}
            {lots.isError && <Alert>{errorMessage(lots.error)}</Alert>}
            {lotList?.length === 0 && (
              <p>Este produto não tem lotes com stock nem dentro da validade: não há nada para ajustar.</p>
            )}

            {lot && lotList && (
              <>
                <RadioGroup
                  legend="Lote"
                  name="lotId"
                  value={lot.id}
                  onChange={(id) => {
                    setLotId(id)
                    setError(null)
                  }}
                  options={lotList.map((l) => ({ value: l.id, label: lotLabel(l, product, today) }))}
                  hint="Pela ordem em que o stock sai: primeiro o que expira primeiro."
                />
                <RadioGroup
                  legend="O que aconteceu"
                  name="kind"
                  value={kind}
                  onChange={setKind}
                  options={kinds}
                  hint={kindHint[kind]}
                />
                <TextField
                  label={`Quantidade (${unit})`}
                  name="quantity"
                  inputMode={isDecimal(product) ? 'decimal' : 'numeric'}
                  autoComplete="off"
                  required
                  value={quantity}
                  onChange={(e) => setQuantity(e.target.value)}
                  placeholder={isDecimal(product) ? '1,5' : '2'}
                  hint={kind === 'mais' ? 'Quanto entra no lote.' : 'Quanto sai do lote.'}
                />
                <TextField
                  label="Motivo"
                  name="reason"
                  autoComplete="off"
                  required
                  maxLength={500}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder={kind === 'quebra' ? 'Caixa caiu ao descarregar' : 'Contagem de inventário'}
                  hint="Obrigatório, até 500 caracteres."
                />
                <Button type="submit" size="lg" disabled={pending}>
                  {pending ? 'A registar…' : kind === 'quebra' ? 'Registar quebra' : 'Registar ajuste'}
                </Button>
              </>
            )}
          </form>
        )}
      </section>
    </div>
  )
}
