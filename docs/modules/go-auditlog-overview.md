# go-auditlog — Overview

> Registro de auditoria estruturado — quem fez o quê, quando, com qual resultado.

## Problema

Operações sensíveis (alteração de dados, ações administrativas, transações financeiras) precisam de registro de auditoria para compliance, debugging e investigação de incidentes. Sem padronização, cada serviço loga de um jeito diferente — ou não loga.

## Escopo

| Feature | O que faz |
| ------- | --------- |
| **Structured entries** | Actor, action, resource, result, metadata — formato padronizado |
| **Storage plugável** | PostgreSQL, Elasticsearch, S3 — interface única |
| **Context propagation** | Extrai actor/trace automaticamente do context (middleware) |
| **Async write** | Buffer + batch insert para não impactar latência da request |
| **Immutability** | Append-only, sem update/delete — registro é permanente |
| **Query API** | Busca por actor, resource, período, action |

## Interface Proposta

```go
// Registro direto
audit.Log(ctx, audit.Entry{
    Actor:    "user:usr_abc123",
    Action:   "user.updated",
    Resource: "user:usr_xyz789",
    Result:   audit.ResultSuccess,
    Metadata: audit.M{
        "changed_fields": []string{"email", "name"},
        "ip":             "10.0.1.50",
    },
})

// Via middleware (captura automaticamente de ctx)
router.Use(audit.Middleware(auditor,
    audit.ForMethods("POST", "PUT", "PATCH", "DELETE"),
))
```

### Entry structure

```go
type Entry struct {
    ID         string    // UUID v7
    Timestamp  time.Time // Quando aconteceu
    Actor      string    // Quem fez (user ID, service name, API key)
    Action     string    // O que fez ("user.created", "payment.refunded")
    Resource   string    // Sobre o quê ("user:123", "order:456")
    Result     Result    // Success, Failure, Denied
    Metadata   M         // Dados adicionais (campos alterados, IP, etc.)
    TraceID    string    // Correlação com distributed trace
    ServiceKey string    // Qual service key autenticou
}
```

## Estrutura Prevista

```text
go-auditlog/
├── audit.go           # Entry, Result, interfaces
├── logger.go          # Auditor (buffer + batch write)
├── middleware.go       # Gin/HTTP middleware
├── pgstore/           # PostgreSQL storage
├── esstore/           # Elasticsearch storage (futuro)
├── otel/              # Métricas (entries/sec, write latency)
└── examples/
```

## Dependências

- Core: zero (stdlib only)
- `pgstore/`: `database/sql`
- `esstore/`: elastic client (futuro)
- Middleware: `github.com/gin-gonic/gin`

## Por que na Appmax

- Operações financeiras (pagamentos, estornos, alteração de limites) exigem rastreabilidade completa
- Compliance (PCI DSS, LGPD) requer registro de quem acessou/alterou dados sensíveis
- Investigação de incidentes fica viável quando todo serviço registra ações no mesmo formato
- Audit trail padronizado simplifica auditorias externas

## Referências

- [OWASP — Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html)
- [AWS CloudTrail](https://aws.amazon.com/cloudtrail/) — referência de design de audit log
- [PCI DSS Requirement 10](https://listings.pcisecuritystandards.org/documents/PCI_DSS-QRG-v3_2_1.pdf) — track and monitor all access
