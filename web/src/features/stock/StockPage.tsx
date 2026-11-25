import { type FormEvent, useRef, useState } from 'react'
import { Link } from 'react-router'

import { useListProducts } from '../../api/generated/endpoints/catalog/catalog'
import { createStockLot } from '../../api/generated/endpoints/inventory/inventory'
import type { Product, StockLot, Unit } from '../../api/generated/model'
import { errorMessage } from '../../api/http'
import { formatDate, formatQuantity, lisbonToday, parseQuantityToMilli } from '../../shared/lib/format'
import { Alert, Button, SelectField, TextField } from '../../shared/ui/ui'
import { useSession } from '../auth/session'
import './StockPage.css'

const unitShort: Record<Unit, string> = { kg: 'kg', l: 'L', un: 'unidades', molho: 'molhos', duzia: 'dúzias' }

const isDecimal = (unit: Unit) => unit === 'kg' || unit === 'l'

type Registered = { lot: StockLot; product: Product }

/** "12,500 kg de Laranja, colheita 24/11/2026, validade 02/12/2026" */
function describe({ lot, product }: Registered): string {
  return `${formatQuantity(lot.quantityReceivedMilli / 1000, product.unit)} de ${product.name}, colheita ${formatDate(
    lot.harvestedOn,
  )}, validade ${formatDate(lot.expiresOn)}`
}

/**
 * Entrada de stock por lotes (UC-04). A quantidade escrita é convertida em
 * milésimos aqui, na fronteira (ADR-0009). A consulta do stock e os alertas
 * chegam com o #14. Os ajustes e as quebras (só o gestor) estão em
 * /app/stock/ajustes.
 */
export default function StockPage() {
  const session = useSession()
  const isGestor = session.status === 'authenticated' && session.user.role === 'gestor'
  const products = useListProducts()
  const active = products.data?.status === 200 ? products.data.data.filter((p) => p.active) : undefined

  // "Hoje" é o dia de Lisboa, como na API; fica fixo enquanto a página está aberta.
  const [today] = useState(lisbonToday)
  const [productId, setProductId] = useState('')
  const [quantity, setQuantity] = useState('')
  const [harvestedOn, setHarvestedOn] = useState(today)
  const [expiresOn, setExpiresOn] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)
  const [registered, setRegistered] = useState<Registered[]>([])
  // O botão desativa-se no render seguinte; esta guarda trava logo um segundo
  // envio (duplo toque no tablet, Enter repetido) para não registar o lote duas vezes.
  const sending = useRef(false)

  const product = active?.find((p) => p.id === productId) ?? active?.[0]
  const unit = product?.unit ?? 'kg'

  function validate(): { milli: number } | string {
    if (!product) return 'Escolhe o produto.'
    const milli = parseQuantityToMilli(quantity, unit)
    if (milli === null) {
      return isDecimal(unit)
        ? 'A quantidade tem de ser um número, com até 3 casas decimais (por exemplo 12,5).'
        : 'A quantidade tem de ser um número inteiro.'
    }
    if (milli <= 0) return 'A quantidade tem de ser maior do que 0.'
    if (!harvestedOn) return 'Indica a data de colheita.'
    if (harvestedOn > today) return 'A data de colheita não pode ser no futuro.'
    if (!expiresOn) return 'Indica a validade.'
    if (expiresOn < harvestedOn) return 'A validade não pode ser anterior à data de colheita.'
    if (expiresOn < today) return 'A validade já passou: um lote fora de validade não se pode vender.'
    return { milli }
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (sending.current || !product) return
    const checked = validate()
    if (typeof checked === 'string') {
      setError(checked)
      return
    }

    sending.current = true
    setPending(true)
    setError(null)
    try {
      const res = await createStockLot({ productId: product.id, quantityMilli: checked.milli, harvestedOn, expiresOn })
      if (res.status === 201) {
        setRegistered((cur) => [{ lot: res.data, product }, ...cur])
        // Limpa a quantidade e a validade: repetir o envio obriga a escrevê-las de novo.
        setQuantity('')
        setExpiresOn('')
      }
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      sending.current = false
      setPending(false)
    }
  }

  const last = registered[0]

  return (
    <div className="stock">
      {isGestor && (
        <p className="stock__lead">
          Algo se estragou ou a contagem não bate certo?{' '}
          <Link to="/app/stock/ajustes">Registar um ajuste ou uma quebra</Link>
        </p>
      )}
      <section className="stock__card" aria-labelledby="entrada-titulo">
        <h2 id="entrada-titulo" className="stock__title">
          Registar entrada
        </h2>
        <p className="stock__lead">Cada entrada fica como um lote, com a data de colheita e a validade.</p>

        {products.isPending && <p role="status">A carregar os produtos…</p>}
        {products.isError && <Alert>{errorMessage(products.error)}</Alert>}
        {active?.length === 0 && <p>Ainda não há produtos ativos. O gestor tem de os criar primeiro.</p>}

        {last && !error && <Alert tone="info">Entrada registada: {describe(last)}.</Alert>}
        {error && <Alert>{error}</Alert>}

        {product && (
          <form className="stock__form" noValidate onSubmit={(e) => void submit(e)}>
            <SelectField
              label="Produto"
              name="productId"
              value={product.id}
              onChange={(e) => setProductId(e.target.value)}
              options={active?.map((p) => ({ value: p.id, label: p.name })) ?? []}
            />
            <TextField
              label={`Quantidade (${unitShort[unit]})`}
              name="quantity"
              inputMode={isDecimal(unit) ? 'decimal' : 'numeric'}
              autoComplete="off"
              required
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
              placeholder={isDecimal(unit) ? '12,5' : '10'}
              hint={isDecimal(unit) ? 'Até 3 casas decimais.' : 'Só números inteiros.'}
            />
            <div className="stock__row">
              <TextField
                label="Data de colheita"
                name="harvestedOn"
                type="date"
                required
                max={today}
                value={harvestedOn}
                onChange={(e) => setHarvestedOn(e.target.value)}
              />
              <TextField
                label="Validade"
                name="expiresOn"
                type="date"
                required
                min={harvestedOn > today ? harvestedOn : today}
                value={expiresOn}
                onChange={(e) => setExpiresOn(e.target.value)}
                hint="Último dia em que se pode vender."
              />
            </div>
            <Button type="submit" size="lg" disabled={pending}>
              {pending ? 'A registar…' : 'Registar entrada'}
            </Button>
          </form>
        )}
      </section>

      {registered.length > 0 && (
        <section className="stock__card" aria-labelledby="registadas-titulo">
          <h2 id="registadas-titulo" className="stock__title">
            Entradas registadas agora
          </h2>
          <ul className="stock__registered" aria-label="Entradas registadas">
            {registered.map((r) => (
              <li key={r.lot.id}>{describe(r)}</li>
            ))}
          </ul>
        </section>
      )}
    </div>
  )
}
