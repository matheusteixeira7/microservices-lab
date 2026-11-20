# syntax=docker/dockerfile:1
# Uma imagem para cada serviço Node (pedidos, pagamento, notificacao).
# Uso: docker build -f docker/node.Dockerfile --build-arg APP=pedidos .
# Os 3 serviços dividem o cache das camadas de instalação.

FROM node:24-alpine AS base
RUN npm install -g pnpm@11.17.0
ENV NX_DAEMON=false NX_NO_CLOUD=true NX_TUI=false CI=true
# O store do pnpm fica num cache do BuildKit (--mount=type=cache), fora da imagem.
ENV PNPM_STORE_DIR=/pnpm-store

# --- build: instala tudo, compila e gera o dist "podado" do serviço ---
FROM base AS build
WORKDIR /repo
# Só o lockfile primeiro: esta camada só refaz quando as dependências mudam.
COPY pnpm-lock.yaml pnpm-workspace.yaml ./
RUN --mount=type=cache,id=pnpm-store,target=/pnpm-store \
    pnpm fetch --store-dir=/pnpm-store
COPY . .
RUN --mount=type=cache,id=pnpm-store,target=/pnpm-store \
    pnpm install --offline --frozen-lockfile --store-dir=/pnpm-store
ARG APP
# prune = build + package.json/lockfile só com as dependências de produção do serviço.
RUN pnpm nx run ${APP}:prune --skip-nx-cache

# --- runtime: só o código compilado + dependências de produção ---
FROM base AS runtime
ARG APP
WORKDIR /app
COPY --from=build /repo/apps/${APP}/dist ./
RUN --mount=type=cache,id=pnpm-store,target=/pnpm-store \
    pnpm install --prod --frozen-lockfile --store-dir=/pnpm-store --package-import-method=copy
ENV NODE_ENV=production
USER node
CMD ["node", "main.js"]
