# ADRs (Architecture Decision Records)

Um ADR registra **uma decisão importante**: o contexto, o que foi decidido, as
alternativas e as consequências. Serve para o "eu do futuro" entender *por que*
as coisas são como são.

| # | Decisão | Status |
|---|---|---|
| [0001](0001-limites-entre-servicos.md) | Limites entre os serviços | Aceito |

## Como criar um novo

1. Copie o modelo abaixo para `NNNN-titulo-curto.md` (próximo número).
2. Preencha e adicione uma linha na tabela acima.
3. Um ADR aceito não é apagado. Se a decisão mudar, crie um novo ADR e marque o
   antigo como **Substituído por NNNN**.

```markdown
# ADR NNNN — Título

- **Status:** Proposto | Aceito | Substituído por NNNN
- **Data:** AAAA-MM-DD

## Contexto
Qual problema estamos resolvendo? Quais forças e restrições existem?

## Decisão
O que decidimos fazer.

## Alternativas consideradas
O que mais foi avaliado e por que não foi escolhido.

## Consequências
O que fica melhor, o que fica pior, o que precisamos fazer por causa disso.
```
