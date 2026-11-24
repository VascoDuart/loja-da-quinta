import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import {
  createCategory,
  createProduct,
  deleteCategory,
  deleteProductImage,
  getListCategoriesQueryKey,
  getListProductsQueryKey,
  putProductImage,
  renameCategory,
  updateProduct,
  useListCategories,
  useListProducts,
} from '../../api/generated/endpoints/catalog/catalog'
import type { Product } from '../../api/generated/model'
import { ApiError, errorMessage } from '../../api/http'
import { productImageUrl } from '../../api/productImage'
import { formatSeason, formatUnitPrice } from '../../shared/lib/format'
import { Alert, Button, Pill } from '../../shared/ui/ui'
import { CategoriesPanel } from './CategoriesPanel'
import { ProductForm } from './ProductForm'
import { ProductImageEditor } from './ProductImageEditor'
import './ProdutosPage.css'

type Editing = { mode: 'create' } | { mode: 'edit'; id: string } | null

/** Catálogo do backoffice: categorias e produtos (UC-03). */
export default function ProdutosPage() {
  const queryClient = useQueryClient()
  const categories = useListCategories()
  const products = useListProducts()
  const [editing, setEditing] = useState<Editing>(null)
  const [notice, setNotice] = useState<string | null>(null)
  // Muda para voltar a abrir o formulário com os dados atuais (depois de um 412).
  const [formKey, setFormKey] = useState(0)

  // O apiFetch lança nas respostas de erro: aqui só chega o 200.
  const categoryList = categories.data?.status === 200 ? categories.data.data : undefined
  const productList = products.data?.status === 200 ? products.data.data : undefined
  // Em edição, usa-se sempre a versão mais recente da lista (por exemplo,
  // depois de trocar a imagem, que muda o produto).
  const current: Product | undefined =
    editing?.mode === 'edit' ? productList?.find((p) => p.id === editing.id) : undefined

  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: getListProductsQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getListCategoriesQueryKey() }),
    ])

  return (
    <div className="produtos">
      {notice && <Alert tone="info">{notice}</Alert>}
      {categories.isError && <Alert>{errorMessage(categories.error)}</Alert>}
      {products.isError && <Alert>{errorMessage(products.error)}</Alert>}

      {categoryList && (
        <CategoriesPanel
          categories={categoryList}
          onCreate={async (name) => {
            await createCategory({ name })
            await refresh()
          }}
          onRename={async (id, name) => {
            await renameCategory(id, { name })
            await refresh()
          }}
          onDelete={async (id) => {
            await deleteCategory(id)
            await refresh()
          }}
        />
      )}

      <section className="produtos__products" aria-labelledby="produtos-titulo">
        <div className="produtos__toolbar">
          <h2 id="produtos-titulo" className="section-title">
            Lista de produtos
          </h2>
          {!editing && categoryList && categoryList.length > 0 && (
            <Button
              onClick={() => {
                setNotice(null)
                setEditing({ mode: 'create' })
              }}
            >
              Novo produto
            </Button>
          )}
        </div>
        {categoryList?.length === 0 && (
          <p>Cria primeiro uma categoria (por exemplo, Fruta) para poderes criar produtos.</p>
        )}

        {editing?.mode === 'create' && categoryList && (
          <ProductForm
            categories={categoryList}
            onCancel={() => setEditing(null)}
            onSubmit={async (input) => {
              const res = await createProduct(input)
              await refresh()
              // Depois de criar, fica em edição para se poder pôr a imagem.
              if (res.status === 201) setEditing({ mode: 'edit', id: res.data.id })
              setNotice(`Produto ${input.name.trim()} criado. Podes agora escolher a imagem.`)
            }}
          />
        )}

        {editing?.mode === 'edit' && current && categoryList && (
          <div className="produtos__edit">
            <ProductForm
              key={`${current.id}-${formKey}`}
              categories={categoryList}
              product={current}
              onCancel={() => setEditing(null)}
              onReload={() => setFormKey((k) => k + 1)}
              onSubmit={async (input, version) => {
                try {
                  await updateProduct(current.id, input, { headers: { 'If-Match': `"${version ?? current.version}"` } })
                } catch (err) {
                  // Outra pessoa gravou entretanto: traz a versão atual para a
                  // lista, sem fechar o formulário (o aviso tem de ficar à vista).
                  if (err instanceof ApiError && err.status === 412) await refresh()
                  throw err
                }
                await refresh()
                setEditing(null)
                setNotice(`Alterações a ${input.name.trim()} guardadas.`)
              }}
            />
            <ProductImageEditor
              product={current}
              onUpload={async (file) => {
                await putProductImage(current.id, file)
                await refresh()
              }}
              onRemove={async () => {
                await deleteProductImage(current.id)
                await refresh()
              }}
            />
          </div>
        )}

        {products.isPending && <p role="status">A carregar os produtos…</p>}
        {productList && productList.length === 0 && <p>Ainda não há produtos.</p>}
        {productList && productList.length > 0 && (
          <ul className="product-list" aria-label="Produtos">
            {productList.map((p) => (
              <li key={p.id} className={`product-card${p.active ? '' : ' product-card--inactive'}`}>
                {p.image ? (
                  <img
                    className="product-card__img"
                    src={productImageUrl(p.id, p.image.version, 'thumb')}
                    alt=""
                    width={56}
                    height={56}
                    loading="lazy"
                  />
                ) : (
                  <span className="product-card__img product-card__img--empty" aria-hidden="true">
                    {p.name.charAt(0)}
                  </span>
                )}
                <div className="product-card__info">
                  <span className="product-card__name">{p.name}</span>
                  <span className="product-card__meta">
                    {p.categoryName} · {formatUnitPrice(p.priceCents, p.unit)} · IVA {p.vatRate} % ·{' '}
                    {formatSeason(p.seasonMonths)}
                  </span>
                </div>
                <div className="product-card__tags">
                  <Pill tone={p.active ? 'ok' : 'bad'}>{p.active ? 'Ativo' : 'Inativo'}</Pill>
                  <Pill tone={p.isPublic ? 'ok' : 'neutral'}>{p.isPublic ? 'Público' : 'Só interno'}</Pill>
                </div>
                <Button
                  variant="secondary"
                  aria-label={`Editar ${p.name}`}
                  onClick={() => {
                    setNotice(null)
                    setEditing({ mode: 'edit', id: p.id })
                  }}
                >
                  Editar
                </Button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
