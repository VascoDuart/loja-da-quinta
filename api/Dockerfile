# Imagem da API. Dois binários: /api (servidor) e /migrate (migrações).
#
#   docker build -t loja-da-quinta-api api/

FROM golang:1.26.11-alpine AS build
WORKDIR /src

# Dependências primeiro: ficam em cache enquanto go.mod/go.sum não mudarem.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# distroless: sem shell nem gestor de pacotes, menos superfície de ataque.
# :nonroot corre com o utilizador 65532, nunca como root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
COPY --from=build /out/migrate /migrate
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
