# 0014. Imagens dos produtos no PostgreSQL, servidas pela CDN

- **Estado:** Aceite
- **Data:** 2026-11-19

## Contexto

O design aprovado ([ADR-0013](0013-design-system-horta-fresca.md)) mostra uma imagem em cada produto, no catálogo público e no ecrã de venda. As imagens passam por isso para o MVP. O orçamento é zero e uma loja de quinta tem dezenas de produtos, não milhares.

## Opções consideradas

1. **Armazenamento de objetos** (Cloudflare R2, 10 GB grátis): escala bem, mas exige registar um cartão, mais um serviço e mais segredos.
2. **Ficheiros estáticos no repositório:** o gestor não os consegue mudar sem um deploy.
3. **PostgreSQL (`bytea`)**, com as imagens processadas pela API e servidas com cache longa pela CDN da Vercel.

## Decisão

Opção 3.

### Upload (só gestor)

`PUT /api/v1/products/{id}/image`, com um corpo de no máximo 5 MB.

1. Ler os primeiros bytes e confirmar que é **JPEG, PNG ou WebP pelo conteúdo**, nunca pela extensão nem pelo `Content-Type` enviado. Outro formato dá `415`.
2. Ler só as dimensões antes de descodificar; mais de 40 megapíxeis dá `413` (proteção contra bombas de descompressão).
3. Descodificar e **voltar a codificar** em JPEG (qualidade 82) em duas versões: `large` com 800 px no lado maior e `thumb` com 160 px. O original nunca é guardado: assim desaparecem metadados (EXIF, localização GPS) e qualquer conteúdo escondido no ficheiro.
4. Gravar as duas versões e o evento de auditoria numa única transação.

### Leitura (pública)

`GET /api/v1/public/products/{id}/image?size=thumb|large&v=<sha256>`

- `Cache-Control: public, max-age=31536000, immutable`, `ETag` com o hash, `X-Content-Type-Options: nosniff`.
- O `v` muda sempre que a imagem muda: a cache invalida-se sozinha, sem purgas.
- A CDN da Vercel serve as repetições; a base de dados só é lida na primeira vez de cada versão.

### Modelo

Tabela `product_images` em [data-model.md](../architecture/data-model.md). O módulo `catalog` fala com uma porta `ImageStore`; o adaptador é o PostgreSQL.

## Consequências

- Sem serviço extra, sem cartão e sem novos segredos. Produto e imagem ficam consistentes na mesma transação; não há imagens órfãs.
- Backups e auditoria cobrem as imagens sem trabalho extra.
- Ocupação estimada: cerca de 135 KB por produto, ou seja, 100 produtos ≈ 14 MB dos 1 GB da Neon.
- Sem a cache da CDN, cada imagem passaria pela API e pela base de dados. É por isso que os URLs são imutáveis.
- Não serve para milhares de imagens grandes. Se a loja crescer, troca-se o adaptador de `ImageStore` por R2 ou S3 e migram-se os ficheiros; o domínio e a API não mudam.
- O capítulo V12 (ficheiros) do ASVS passa a aplicar-se.
- A codificação em WebP ou AVIF exigiria cgo no Go; fica JPEG, que a biblioteca padrão codifica.
