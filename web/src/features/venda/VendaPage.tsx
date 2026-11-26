import { useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useMemo, useRef, useState } from 'react'

import { useListProducts } from '../../api/generated/endpoints/catalog/catalog'
import { getGetStockOverviewQueryKey, useGetStockOverview } from '../../api/generated/endpoints/inventory/inventory'
import { createSale } from '../../api/generated/endpoints/sales/sales'
import type { Payment, Product, Sale } from '../../api/generated/model'
import { errorMessage } from '../../api/http'
import { formatEuro, formatQuantity, formatUnitPrice } from '../../shared/lib/format'
import { Alert, Button, Pill, RadioGroup, TextField } from '../../shared/ui/ui'
import { buildSaleInput, type CartLine, cartTotalCents, isDecimal, lineMilli, lineTotalCents, searchKey } from './cart'
import './VendaPage.css'

const paymentOptions = [
  { value: 'numerario', label: 'Numerário' },
  { value: 'mbway', label: 'MB Way' },
] as const satisfies readonly { value: Payment; label: string }[]

const paymentLabel: Record<Payment, string> = { numerario: 'Numerário', mbway: 'MB Way' }

const unitShort = { kg: 'kg', l: 'L', un: 'unidades', molho: 'molhos', duzia: 'dúzias' } as const

/** At most this many search results, so the list stays quick to scan on a tablet. */
const MAX_RESULTS = 12

/**
 * Counter sale (UC-06), built for a tablet: search, tap to add, type the
 * quantity on the numeric keyboard, choose the payment and confirm. The total
 * shown here is only a preview: the server prices the sale again from the
 * catalog (RF-09). The Idempotency-Key stays the same while the cart does not
 * change, so a retry after a network failure never records the sale twice.
 */
export default function VendaPage() {
  const queryClient = useQueryClient()
  const productsQuery = useListProducts()
  const stockQuery = useGetStockOverview()

  const [search, setSearch] = useState('')
  const [lines, setLines] = useState<CartLine[]>([])
  const [payment, setPayment] = useState<Payment | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)
  const [sale, setSale] = useState<Sale | null>(null)
  // The key of the last attempt and what was sent with it: the same cart
  // reuses the key; a changed cart is another sale and gets a new one.
  const attempt = useRef<{ key: string; body: string } | null>(null)
  const sending = useRef(false)

  const products = useMemo(() => {
    const all = productsQuery.data?.status === 200 ? productsQuery.data.data : []
    return new Map(all.filter((p) => p.active).map((p) => [p.id, p]))
  }, [productsQuery.data])
  const sellable = useMemo(() => {
    const overview = stockQuery.data?.status === 200 ? stockQuery.data.data : undefined
    return new Map(overview?.products.map((p) => [p.productId, p.sellableMilli]))
  }, [stockQuery.data])

  const results = useMemo(() => {
    const q = searchKey(search)
    const inCart = new Set(lines.map((l) => l.productId))
    return [...products.values()]
      .filter((p) => !inCart.has(p.id) && searchKey(p.name).includes(q))
      .sort((a, b) => a.name.localeCompare(b.name, 'pt-PT'))
      .slice(0, MAX_RESULTS)
  }, [products, lines, search])

  const total = cartTotalCents(lines, products)

  function add(product: Product) {
    setError(null)
    setLines((cur) => [...cur, { productId: product.id, quantity: isDecimal(product.unit) ? '' : '1' }])
    setSearch('')
    // Focus the new line's quantity, so the numeric keyboard opens at once.
    requestAnimationFrame(() => document.querySelector<HTMLInputElement>(`[data-qty="${product.id}"]`)?.focus())
  }

  function setQuantity(productId: string, quantity: string) {
    setLines((cur) => cur.map((l) => (l.productId === productId ? { ...l, quantity } : l)))
  }

  function remove(productId: string) {
    setLines((cur) => cur.filter((l) => l.productId !== productId))
  }

  function reset() {
    setLines([])
    setPayment(null)
    setSearch('')
    setError(null)
    attempt.current = null
  }

  async function confirm(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (sending.current) return
    const input = buildSaleInput(lines, products, payment)
    if (typeof input === 'string') {
      setError(input)
      return
    }
    const body = JSON.stringify(input)
    if (attempt.current?.body !== body) attempt.current = { key: crypto.randomUUID(), body }

    sending.current = true
    setPending(true)
    setError(null)
    try {
      const res = await createSale(input, { headers: { 'Idempotency-Key': attempt.current.key } })
      if (res.status === 201 || res.status === 200) {
        void queryClient.invalidateQueries({ queryKey: getGetStockOverviewQueryKey() })
        setSale(res.data)
        reset()
      }
    } catch (err) {
      // The cart stays as it is (UC-06 7a, 7b): fix it, or simply confirm again.
      setError(errorMessage(err))
    } finally {
      sending.current = false
      setPending(false)
    }
  }

  if (sale) return <SaleSummary sale={sale} onNext={() => setSale(null)} />

  return (
    <form className="venda" noValidate onSubmit={(e) => void confirm(e)}>
      <section className="venda__card venda__search" aria-labelledby="venda-produtos">
        <h2 id="venda-produtos" className="venda__title">
          Produtos
        </h2>
        <TextField
          label="Pesquisar produto"
          type="search"
          name="search"
          autoComplete="off"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Nome do produto"
        />
        {productsQuery.isPending && <p role="status">A carregar os produtos…</p>}
        {productsQuery.isError && <Alert>{errorMessage(productsQuery.error)}</Alert>}
        {productsQuery.isSuccess && results.length === 0 && (
          <p className="venda__muted">{search ? 'Nenhum produto encontrado.' : 'Não há mais produtos ativos.'}</p>
        )}
        <ul className="venda__results" aria-label="Resultados da pesquisa">
          {results.map((p) => {
            const stock = sellable.get(p.id) ?? 0
            const soldOut = stockQuery.isSuccess && stock === 0
            return (
              <li key={p.id}>
                <button type="button" className="venda__product" onClick={() => add(p)} disabled={soldOut}>
                  <span className="venda__product-name">{p.name}</span>
                  <span className="venda__muted">{formatUnitPrice(p.priceCents, p.unit)}</span>
                  {soldOut ? (
                    <Pill tone="bad">Esgotado</Pill>
                  ) : (
                    stockQuery.isSuccess && <span className="venda__muted">{formatQuantity(stock / 1000, p.unit)}</span>
                  )}
                  {!soldOut && <span className="venda__add">Juntar</span>}
                </button>
              </li>
            )
          })}
        </ul>
      </section>

      <section className="venda__card venda__cart" aria-labelledby="venda-carrinho">
        <h2 id="venda-carrinho" className="venda__title">
          Registo de venda
        </h2>
        {lines.length === 0 && <p className="venda__muted">Toca num produto para o juntar à venda.</p>}
        <ul className="venda__lines" aria-label="Linhas da venda">
          {lines.map((line) => {
            const p = products.get(line.productId)
            if (!p) return null
            const milli = lineMilli(line, p)
            return (
              <li key={p.id} className="venda__line">
                <TextField
                  data-qty={p.id}
                  className="venda__qty"
                  label={`${p.name} (${unitShort[p.unit]})`}
                  inputMode={isDecimal(p.unit) ? 'decimal' : 'numeric'}
                  autoComplete="off"
                  value={line.quantity}
                  onChange={(e) => setQuantity(p.id, e.target.value)}
                  placeholder={isDecimal(p.unit) ? '1,5' : '1'}
                  hint={formatUnitPrice(p.priceCents, p.unit)}
                />
                <span className="venda__line-total" aria-label={`Preço da linha de ${p.name}`}>
                  {milli ? formatEuro(lineTotalCents(p.priceCents, milli)) : '—'}
                </span>
                <Button variant="ghost" onClick={() => remove(p.id)} aria-label={`Tirar ${p.name}`}>
                  Tirar
                </Button>
              </li>
            )
          })}
        </ul>

        <p className="venda__total">
          <span>Total</span>
          <strong data-testid="venda-total">{formatEuro(total)}</strong>
        </p>

        <RadioGroup
          legend="Pagamento"
          name="payment"
          value={payment ?? ('' as Payment)}
          options={paymentOptions}
          onChange={setPayment}
        />

        {error && <Alert>{error}</Alert>}

        <div className="venda__actions">
          <Button type="submit" size="lg" disabled={pending || lines.length === 0}>
            {pending ? 'A registar…' : 'Confirmar registo de venda'}
          </Button>
          <Button variant="secondary" size="lg" onClick={reset} disabled={pending || lines.length === 0}>
            Cancelar venda
          </Button>
        </div>
        <p className="venda__muted venda__note">Registo interno de vendas e stock: não é uma fatura.</p>
      </section>
    </form>
  )
}

/** What the clerk sees once the sale is recorded (UC-06, step 8). */
function SaleSummary({ sale, onNext }: { sale: Sale; onNext: () => void }) {
  return (
    <section className="venda__card venda__summary" aria-labelledby="venda-registada">
      <h2 id="venda-registada" className="venda__title">
        Venda {sale.number} registada
      </h2>
      <Alert tone="info">
        Registo de venda {sale.number} gravado: {formatEuro(sale.totalCents)}, {paymentLabel[sale.payment]}.
      </Alert>
      <table className="venda__table">
        <caption className="venda__muted">Resumo do registo (não é uma fatura)</caption>
        <thead>
          <tr>
            <th scope="col">Produto</th>
            <th scope="col">Quantidade</th>
            <th scope="col">Preço</th>
          </tr>
        </thead>
        <tbody>
          {sale.items.map((it) => (
            <tr key={it.productId}>
              <td>{it.productName}</td>
              <td>{formatQuantity(it.quantityMilli / 1000, it.unit)}</td>
              <td>{formatEuro(it.lineTotalCents)}</td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr>
            <th scope="row" colSpan={2}>
              Total (IVA incluído: {formatEuro(sale.vatCents)})
            </th>
            <td>{formatEuro(sale.totalCents)}</td>
          </tr>
        </tfoot>
      </table>
      <Button size="lg" onClick={onNext}>
        Nova venda
      </Button>
    </section>
  )
}
