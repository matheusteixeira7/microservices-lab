# syntax=docker/dockerfile:1
# Imagem do serviço estoque (Go).
# Uso: docker build -f docker/go.Dockerfile .

FROM golang:1.26-alpine AS build
WORKDIR /src
# Só go.mod/go.sum primeiro: esta camada só refaz quando as dependências mudam.
COPY apps/estoque/go.mod apps/estoque/go.sum ./
RUN go mod download
COPY apps/estoque/ ./
# GOWORK=off: o go.work da raiz não entra na imagem.
RUN CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags="-s -w" -o /out/estoque .

# alpine (e não scratch) para ter o wget do healthcheck.
FROM alpine:3.22 AS runtime
RUN adduser -D -H app
COPY --from=build /out/estoque /usr/local/bin/estoque
USER app
CMD ["estoque"]
