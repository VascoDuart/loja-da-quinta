import { useGetPublicCatalog } from '../../api/generated/endpoints/catalog/catalog'
import type { CatalogItem } from '../../api/generated/model'

/** O catálogo é pequeno e muda pouco: vem todo de uma vez (cache de 60 s) e filtra-se no cliente. */
export function usePublicCatalog() {
  const query = useGetPublicCatalog()
  const items: CatalogItem[] | undefined = query.data?.status === 200 ? query.data.data : undefined
  return { query, items }
}
