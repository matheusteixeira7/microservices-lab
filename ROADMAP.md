# Microservices Lab — Roteiro de estudos

Objetivo: me manter afiado como dev sênior construindo uma rede de microsserviços em Node.js,
aplicando sistemas distribuídos na prática + desenvolvimento com IA.

> Lembrete: sênior não é quem sabe usar microsserviços, é quem sabe **quando usar e quando não usar**.
> Este projeto existe para aprender as partes difíceis, não para provar que microsserviço é sempre melhor.

## O projeto

Mini e-commerce com 4 serviços:

| Serviço      | Stack   | Responsabilidade                         |
|--------------|---------|------------------------------------------|
| pedidos      | NestJS  | Cria pedidos e coordena a saga           |
| pagamento    | NestJS  | Cobra o pedido (vai falhar de propósito) |
| estoque      | Go      | Reserva e libera itens (servidor gRPC)   |
| notificação  | Fastify | Avisa o cliente por e-mail/push (fake)   |

Mais tarde: um serviço de **suporte com IA** e um **dashboard em Next.js**.

## Regras de estudo

- Para cada conceito: **implementar → quebrar de propósito → observar**.
- Escrever um **ADR** (Architecture Decision Record) a cada decisão importante, em `docs/adr/`.
- Um serviço em **Fastify puro** (sem Nest), para ver o que o framework esconde.
- Um serviço em **Go**, para aprender um sistema poliglota (várias linguagens conversando).
- Fazer profiling do Node sempre que algo ficar lento.

---

## Fase 0 — Base

- [x] Monorepo com Nx (pnpm para JS/TS, Go modules + plugin `@nx-go/nx-go` para Go)
- [x] Criar os 4 serviços (2 em NestJS, 1 em Go, 1 em Fastify puro)
- [x] Testes e2e dentro de cada serviço (`apps/<servico>/e2e/`), todos com Vitest
- [x] Cada serviço com sua própria porta (pedidos 3001, pagamento 3002, estoque 3003, notificação 3004)
- [x] Um Postgres por serviço (cada serviço é dono dos seus dados)
- [x] Tudo subindo com Docker Compose (`docker compose up -d --build`)
- [x] Primeiro ADR: por que esses limites entre serviços (bounded contexts / DDD) — [`docs/adr/0001`](docs/adr/0001-limites-entre-servicos.md)

## Fase 1 — Comunicação

- [x] gRPC entre pedidos → estoque (chamada síncrona) — `libs/contracts/proto/inventory/v1`
- [ ] Eventos com Kafka (Redpanda localmente)
- [ ] Contratos versionados: protobuf ✔ + schemas de eventos (falta)
- [x] Usar `buf` para gerar código dos `.proto` em TypeScript e Go (`pnpm nx codegen contracts`)
- [ ] Estudar: quando usar síncrono vs assíncrono

## Fase 2 — Consistência

- [ ] Saga do pedido: reservar estoque → cobrar → confirmar (com compensação se falhar)
- [ ] Comparar saga orquestrada vs coreografada
- [ ] Outbox pattern (evento salvo na mesma transação do banco)
- [ ] Consumidores idempotentes (processar 2x não duplica efeito)

## Fase 3 — Resiliência

- [ ] Circuit breaker com `opossum`
- [ ] Retry com backoff + jitter
- [ ] Timeouts em todas as chamadas externas
- [ ] Bulkhead (isolar recursos) e rate limit
- [ ] Chaos com Toxiproxy: latência, queda, pacotes perdidos

## Fase 4 — Observabilidade

- [ ] OpenTelemetry em todos os serviços
- [ ] Traces no Jaeger (ver a requisição passando por todos os serviços)
- [ ] Métricas com Prometheus + Grafana
- [ ] Logs com Loki, correlacionados com o trace
- [ ] Definir SLOs e alertas

## Fase 5 — Kubernetes + Service Mesh

- [ ] Cluster local com `kind` + Helm
- [ ] Linkerd: mTLS, métricas e retries automáticos
- [ ] Comparar retry/timeout no código vs no mesh — escrever ADR com a conclusão
- [ ] Teste de carga com k6
- [ ] (Opcional) Testar Istio para comparar

## Fase 6 — IA

- [ ] Serviço de suporte com LLM usando **tool use** (consulta pedidos via API)
- [ ] RAG sobre um FAQ
- [ ] Evals: testes automáticos de qualidade das respostas
- [ ] Observabilidade do LLM (custo, latência, tokens)
- [ ] Dashboard em Next.js: chat + visão dos pedidos

## Contínuo (durante todas as fases)

- [ ] Node por dentro: event loop, streams, worker threads
- [ ] Profiling e caça a memory leak
- [ ] System design: cache, réplicas, sharding
- [ ] CI/CD (GitHub Actions) e Terraform

---

## Glossário rápido

- **Circuit breaker**: "disjuntor". Se um serviço falha muito, para de chamar ele por um tempo.
- **Backoff**: esperar cada vez mais entre tentativas.
- **Bulkhead**: separar recursos para uma falha não derrubar tudo.
- **Idempotência**: repetir a mesma operação não muda o resultado.
- **Saga**: transação dividida em passos, com "desfazer" para cada passo.
- **Outbox**: salvar o evento junto com o dado no banco e publicar depois.
- **Trace**: o caminho de uma requisição por todos os serviços.
- **SLO**: meta de qualidade (ex: 99,9% das requisições abaixo de 300ms).
- **Service mesh**: camada que cuida da rede entre serviços sem mexer no código.
- **RAG**: buscar textos relevantes e entregar ao LLM junto com a pergunta.
- **Evals**: testes automáticos para respostas de IA.
