# Matriz de autorização

Tudo é negado por omissão. Cada linha transforma-se num teste de integração (`TestAuthorizationMatrix`): cada papel chama o endpoint e confirma a resposta esperada (2xx ou 401/403).

**Onde está a regra:** na própria OpenAPI. Cada operação declara os papéis com acesso na lista do `bearerAuth`:

```yaml
security:
  - bearerAuth: [funcionario, gestor]
```

O valor por omissão da spec é `bearerAuth: []`, que não deixa ninguém entrar: um endpoint novo esquecido dá 403 a todos, em vez de ficar aberto. As operações públicas declaram `security: []` e têm de estar na lista `publicOperations` de um teste (`TestEveryOperationDeclaresItsAccess`), por isso abrir um endpoint fica sempre visível no PR.

| Endpoint | Visitante | Funcionário | Gestor |
|---|:---:|:---:|:---:|
| `GET /public/catalog` | ✓ | ✓ | ✓ |
| `GET /public/products/{id}/image` | ✓ | ✓ | ✓ |
| `POST /auth/login` · `/auth/refresh` · `/auth/logout` | ✓ | ✓ | ✓ |
| `GET /me` | 401 | ✓ | ✓ |
| `GET /stock` · `GET /products` · `GET /products/{id}` · `GET /categories` | 401 | ✓ | ✓ |
| `POST /stock/lots` | 401 | ✓ | ✓ |
| `POST /sales` | 401 | ✓ | ✓ |
| `POST /stock/adjustments` | 401 | 403 | ✓ |
| `POST /sales/{id}/void` | 401 | 403 | ✓ |
| `POST` · `PUT` · `DELETE /products` · `POST` · `PATCH` · `DELETE /categories` | 401 | 403 | ✓ |
| `PUT` · `DELETE /products/{id}/image` | 401 | 403 | ✓ |
| `GET /reports/*` | 401 | 403 | ✓ |
| `GET` · `POST /users` · `PATCH /users/{id}` | 401 | 403 | ✓ |
| `GET /audit` | 401 | 403 | ✓ |

**401** quando não há sessão; **403** quando há sessão mas o papel não chega.

Regras de `/users` além do papel: não há `DELETE` (um utilizador fica no histórico e só se desativa), e o gestor não pode desativar nem despromover a própria conta (`409`), para haver sempre um gestor ativo. O serviço `users` volta a verificar o papel, mesmo depois do RBAC da API.

`POST /stock/lots` (entrada de stock) é para o funcionário e o gestor. O serviço `inventory` volta a verificar o papel, e o autor do movimento vem sempre da sessão, nunca do pedido (um campo a mais no corpo dá `400`).

`GET /public/catalog` é público e igual para toda a gente: só devolve produtos ativos e visíveis no catálogo (`isPublic`), com os campos do DTO `CatalogItem` (nome, categoria, unidade, preço por unidade, época, imagem e disponibilidade). Nunca devolve quantidades, IVA, limiar de stock, estado "ativo" nem datas, mesmo a um gestor autenticado: o backoffice usa `GET /products`. A disponibilidade vem do stock real (só os lotes dentro da validade): sem stock, o produto aparece esgotado. A resposta tem `Cache-Control: public, max-age=60`.

Todos os caminhos têm o prefixo `/api/v1`.
