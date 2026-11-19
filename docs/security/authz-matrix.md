# Matriz de autorização

Tudo é negado por omissão. Cada linha transforma-se num teste de integração: cada papel chama o endpoint e confirma a resposta esperada (2xx ou 401/403).

| Endpoint | Visitante | Funcionário | Gestor |
|---|:---:|:---:|:---:|
| `GET /public/catalog` | ✓ | ✓ | ✓ |
| `GET /public/products/{id}/image` | ✓ | ✓ | ✓ |
| `POST /auth/login` · `/auth/refresh` · `/auth/logout` | ✓ | ✓ | ✓ |
| `GET /stock` · `GET /products` | 401 | ✓ | ✓ |
| `POST /stock/lots` | 401 | ✓ | ✓ |
| `POST /sales` | 401 | ✓ | ✓ |
| `POST /stock/adjustments` | 401 | 403 | ✓ |
| `POST /sales/{id}/void` | 401 | 403 | ✓ |
| `POST` · `PUT` · `DELETE /products` · `/categories` | 401 | 403 | ✓ |
| `PUT` · `DELETE /products/{id}/image` | 401 | 403 | ✓ |
| `GET /reports/*` | 401 | 403 | ✓ |
| `* /users` | 401 | 403 | ✓ |
| `GET /audit` | 401 | 403 | ✓ |

**401** quando não há sessão; **403** quando há sessão mas o papel não chega.

Todos os caminhos têm o prefixo `/api/v1`.
