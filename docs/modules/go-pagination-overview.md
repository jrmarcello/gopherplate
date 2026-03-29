# go-pagination — Overview

> Paginação padronizada para APIs REST — cursor-based e offset.

## Problema

Todo serviço com endpoints de listagem precisa de paginação. Hoje cada serviço implementa de um jeito: query params diferentes, formato de resposta diferente, sem suporte a cursor. Isso dificulta a vida dos consumidores da API e gera retrabalho.

## Escopo

| Feature | O que faz |
| ------- | --------- |
| **Offset pagination** | `?page=2&per_page=20` — simples, bom para admin panels e UIs com "página X de Y" |
| **Cursor pagination** | `?cursor=abc123&limit=20` — performante, ideal para feeds, mobile e datasets grandes |
| **Sorting** | `?sort=created_at&order=desc` — múltiplos campos suportados |
| **Filtering** | `?filter[status]=active&filter[created_after]=2026-01-01` — type-safe, extensível |
| **Response envelope** | Metadados padronizados: total, has_next, cursor, page info |

## Interface Proposta

```go
// Parsing de query params → struct tipada
params, err := pagination.ParseRequest(c.Request,
    pagination.WithDefaultLimit(20),
    pagination.WithMaxLimit(100),
    pagination.AllowSorts("created_at", "name", "email"),
    pagination.AllowFilters("status", "created_after"),
)

// Uso no repository
query, args := params.ApplyTo("SELECT * FROM users WHERE 1=1")

// Resposta padronizada
pagination.Respond(c, items, params, total)
// → {"data": [...], "pagination": {"total": 150, "page": 2, "per_page": 20, "has_next": true}}
```

### Cursor pagination

```go
params, err := pagination.ParseCursorRequest(c.Request,
    pagination.WithDefaultLimit(20),
    pagination.CursorField("created_at"), // campo do cursor
)

// Decodifica cursor opaco → WHERE created_at < $1
query, args := params.ApplyTo("SELECT * FROM users WHERE 1=1")

// Resposta com next_cursor
pagination.RespondCursor(c, items, params)
// → {"data": [...], "pagination": {"limit": 20, "has_next": true, "next_cursor": "eyJ..."}}
```

## Estrutura Prevista

```text
go-pagination/
├── pagination.go      # Tipos core (Params, CursorParams, Response)
├── offset.go          # Offset pagination (page/per_page)
├── cursor.go          # Cursor pagination (encode/decode, keyset)
├── sorting.go         # Sort parsing + validation
├── filtering.go       # Filter parsing + type coercion
├── sql.go             # SQL builder helpers (ApplyTo)
├── gin/               # Gin helpers (ParseRequest, Respond)
│   └── gin.go
└── examples/
```

## Dependências

- Core: zero (stdlib only)
- `gin/`: `github.com/gin-gonic/gin`

## Por que na Appmax

- Literalmente todo serviço com CRUD precisa — é o módulo com maior reuso potencial
- Padroniza o contrato de paginação para consumidores (front-end, mobile, outros serviços)
- Cursor pagination é essencial para listagens grandes (transações, logs, eventos)
- Baixa complexidade de implementação, alto impacto

## Referências

- [Slack API — Pagination](https://api.slack.com/docs/pagination) — referência de cursor pagination
- [Stripe API — Pagination](https://stripe.com/docs/api/pagination) — referência de design
- [Use The Index, Luke — Pagination](https://use-the-index-luke.com/no-offset) — por que offset é lento em datasets grandes
