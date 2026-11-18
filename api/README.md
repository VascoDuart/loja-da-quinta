# api

Backend em Go: monólito modular com arquitetura hexagonal. Ver [docs/architecture/backend.md](../docs/architecture/backend.md).

> Ainda sem código. O projeto Go é criado no primeiro PR de funcionalidade.

## Estrutura prevista

```
api/
├── cmd/api/main.go     arranque e composição
├── internal/
│   ├── platform/       config, httpserver, middleware, db, logging
│   ├── auth/           domain.go · service.go · ports.go · postgres.go · http.go
│   ├── users/
│   ├── catalog/
│   ├── inventory/
│   ├── sales/
│   ├── reports/
│   └── audit/
├── migrations/         goose (SQL puro)
├── queries/            SQL para o sqlc
├── sqlc.yaml
└── Dockerfile          multi-stage, distroless, sem root
```
