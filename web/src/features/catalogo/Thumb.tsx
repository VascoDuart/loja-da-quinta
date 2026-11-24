import type { CatalogItem } from '../../api/generated/model'
import { productImageUrl } from '../../api/productImage'

/** Miniatura do produto; sem imagem, um quadrado neutro (decorativo). */
export function Thumb({ item, size }: { item: CatalogItem; size: 'thumb' | 'large' }) {
  if (!item.image)
    return <span className={`catalog-thumb catalog-thumb--${size} catalog-thumb--empty`} aria-hidden="true" />
  return (
    <img
      className={`catalog-thumb catalog-thumb--${size}`}
      src={productImageUrl(item.id, item.image.version, size)}
      alt=""
      loading="lazy"
      width={size === 'thumb' ? 96 : 480}
      height={size === 'thumb' ? 96 : 480}
    />
  )
}
