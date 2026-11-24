/**
 * URL da imagem de um produto. A versão (hash) faz parte do URL: quando a
 * imagem muda, o URL muda, e a cache (que é para sempre) nunca serve uma
 * imagem antiga (ADR-0014).
 */
export function productImageUrl(productId: string, version: string, size: 'thumb' | 'large'): string {
  return `/api/v1/public/products/${encodeURIComponent(productId)}/image?size=${size}&v=${encodeURIComponent(version)}`
}
