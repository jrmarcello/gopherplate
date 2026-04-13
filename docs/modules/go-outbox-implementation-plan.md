# go-outbox — Plano de Implementacao

> Modulo Go standalone para o Transactional Outbox Pattern.
> Repositorio separado, importavel via `go get`.

---

## Indice

1. [Contexto e Problema](#1-contexto-e-problema)
2. [Visao Geral do Modulo](#2-visao-geral-do-modulo)
3. [Arquitetura](#3-arquitetura)
4. [Interfaces Centrais](#4-interfaces-centrais)
5. [Estrutura do Repositorio](#5-estrutura-do-repositorio)
6. [Store (Persistencia)](#6-store-persistencia)
7. [Relay (Processamento)](#7-relay-processamento)
8. [Dispatcher (Entrega)](#8-dispatcher-entrega)
9. [Retry Strategy](#9-retry-strategy)
10. [Observabilidade](#10-observabilidade)
11. [Gestao Transacional](#11-gestao-transacional)
12. [Escalabilidade e Ordering](#12-escalabilidade-e-ordering)
13. [Cleanup e Retencao](#13-cleanup-e-retencao)
14. [Migration de Referencia](#14-migration-de-referencia)
15. [Exemplo de Integracao](#15-exemplo-de-integracao)
16. [Fases de Implementacao](#16-fases-de-implementacao)
17. [Decisoes Arquiteturais](#17-decisoes-arquiteturais)
18. [Referencias](#18-referencias)

---

## 1. Contexto e Problema

Quando um servico precisa **persistir dados no banco E publicar um evento** para outro servico
(ex: "UserCreated" via SQS), existe um risco de inconsistencia:

```text
Cenario A: Salva no banco OK → Publica no SQS FALHA → evento perdido
Cenario B: Publica no SQS OK → Salva no banco FALHA → evento fantasma
```

O **dual write problem** nao tem solucao com two-phase commit em sistemas distribuidos modernos.
O Outbox Pattern resolve isso com **atomicidade transacional**: o evento e gravado na mesma
transacao do banco, e um processo separado (relay) faz o dispatch para o broker.

```text
┌─────────────┐     TX atomica      ┌─────────────────┐
│  Use Case   │ ──────────────────→  │ DB: entity +     │
│  (Create)   │                      │     outbox event  │
└─────────────┘                      └────────┬─────────┘
                                              │
                                     ┌────────▼─────────┐
                                     │  Relay (poller)   │
                                     │  le pending →     │
                                     │  dispatch → mark  │
                                     └────────┬─────────┘
                                              │
                                     ┌────────▼─────────┐
                                     │  Broker (SQS,    │
                                     │  SNS, Kafka...)   │
                                     └──────────────────┘
```

**Garantia**: at-least-once delivery. Consumers devem ser idempotentes
(o event ID / UUID v7 serve como chave de deduplicacao).

---

## 2. Visao Geral do Modulo

| Aspecto              | Decisao                                                  |
| -------------------- | -------------------------------------------------------- |
| **Repositorio**      | `github.com/jrmarcello/go-outbox` (modulo separado)      |
| **Go version**       | 1.23+                                                    |
| **Deps externas**    | Minimas — `database/sql`, `go.opentelemetry.io/otel`     |
| **DB suportados**    | PostgreSQL (primario), MySQL (futuro)                    |
| **Brokers**          | Interface generica + adapters (SQS, SNS, Kafka, NATS)   |
| **Delivery**         | At-least-once (consumer e responsavel por idempotencia)  |
| **Licenca**          | Interna Appmax                                           |

### Principios de Design

1. **Minimo de dependencias** — o core depende apenas de `database/sql` e stdlib
2. **Interfaces pequenas** — cada contrato faz uma coisa
3. **Plugavel** — store, relay, dispatcher, retry e observabilidade sao substituiveis
4. **Seguro para concorrencia** — multiplas instancias do relay podem rodar em paralelo
5. **Observable by default** — OpenTelemetry integrado, mas opcional (graceful degradation)

---

## 3. Arquitetura

```text
go-outbox/
│
│   Camada Core (zero deps externas alem de stdlib)
│   ┌──────────────────────────────────────────┐
│   │  outbox.go       — tipos + interfaces     │
│   │  event.go        — Event, Status, etc.    │
│   │  options.go      — functional options      │
│   └──────────────────────────────────────────┘
│
│   Camada Store (database/sql)
│   ┌──────────────────────────────────────────┐
│   │  pgstore/        — PostgreSQL (SKIP LOCKED,│
│   │                    LISTEN/NOTIFY, cleanup) │
│   │  mysqlstore/     — MySQL (futuro)          │
│   └──────────────────────────────────────────┘
│
│   Camada Relay
│   ┌──────────────────────────────────────────┐
│   │  relay/          — Polling relay (default) │
│   │                    worker pool, circuit    │
│   │                    breaker, backoff        │
│   └──────────────────────────────────────────┘
│
│   Camada Dispatcher (adapters)
│   ┌──────────────────────────────────────────┐
│   │  sqsdispatcher/  — AWS SQS adapter        │
│   │  snsdispatcher/  — AWS SNS adapter        │
│   │  kafkadispatcher/— Kafka adapter (futuro) │
│   └──────────────────────────────────────────┘
│
│   Observabilidade
│   ┌──────────────────────────────────────────┐
│   │  otel/           — Metricas + traces      │
│   └──────────────────────────────────────────┘
```

### Diagrama de Dependencias

```text
outbox (core)          ← zero deps
  ├── pgstore          ← database/sql
  ├── relay            ← outbox (core)
  │     └── otel       ← go.opentelemetry.io/otel (opcional)
  ├── sqsdispatcher    ← aws-sdk-go-v2 (dep isolada)
  ├── snsdispatcher    ← aws-sdk-go-v2 (dep isolada)
  └── kafkadispatcher  ← sarama ou confluent (dep isolada)
```

Cada dispatcher e um sub-package separado — quem usa SQS nao importa Kafka.
O `go.sum` do servico consumidor so tera as deps que realmente precisa.

---

## 4. Interfaces Centrais

### 4.1 Event

```go
// outbox.go

package outbox

import (
    "encoding/json"
    "time"
)

// Status representa o estado de um evento no outbox.
type Status string

const (
    StatusPending    Status = "pending"
    StatusProcessing Status = "processing"
    StatusDispatched Status = "dispatched"
    StatusFailed     Status = "failed"
    StatusDead       Status = "dead" // esgotou retentativas → dead letter
)

// Event representa um evento armazenado na outbox table.
type Event struct {
    ID            string          `json:"id"`             // UUID v7
    AggregateType string          `json:"aggregate_type"` // "user", "order"
    AggregateID   string          `json:"aggregate_id"`   // ID da entidade
    EventType     string          `json:"event_type"`     // "user.created"
    Payload       json.RawMessage `json:"payload"`        // corpo do evento
    Metadata      Metadata        `json:"metadata"`       // trace IDs, correlation, headers
    Status        Status          `json:"status"`
    RetryCount    int             `json:"retry_count"`
    MaxRetries    int             `json:"max_retries"`
    NextRetryAt   time.Time       `json:"next_retry_at"`
    LastError     string          `json:"last_error,omitempty"`
    CreatedAt     time.Time       `json:"created_at"`
    DispatchedAt  *time.Time      `json:"dispatched_at,omitempty"`
}

// Metadata carrega informacoes de contexto propagadas entre servicos.
type Metadata map[string]string
```

### 4.2 Store

```go
// DBTX e satisfeito por *sql.DB e *sql.Tx — permite operar
// dentro ou fora de uma transacao sem alterar a interface.
type DBTX interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store define as operacoes de persistencia do outbox.
type Store interface {
    // Save grava um evento usando qualquer executor (DB ou TX).
    // Para atomicidade com a entidade, passe o *sql.Tx da mesma transacao.
    Save(ctx context.Context, db DBTX, event *Event) error

    // FetchPending retorna e trava (SKIP LOCKED) eventos pendentes.
    // Seguro para multiplos consumers concorrentes.
    FetchPending(ctx context.Context, limit int) ([]*Event, error)

    // MarkDispatched atualiza status para dispatched.
    MarkDispatched(ctx context.Context, id string) error

    // MarkFailed incrementa retry_count, calcula next_retry_at.
    // Se retry_count >= max_retries, move para StatusDead.
    MarkFailed(ctx context.Context, id string, reason string) error

    // Cleanup remove eventos dispatched mais antigos que retention.
    Cleanup(ctx context.Context, retention time.Duration) (int64, error)
}
```

### 4.3 Dispatcher

```go
// Message e a representacao broker-agnostica de um evento pronto para envio.
type Message struct {
    ID            string
    AggregateType string
    AggregateID   string
    EventType     string
    Payload       []byte
    Metadata      Metadata
    CreatedAt     time.Time
}

// DispatchResult representa o resultado do envio de uma mensagem.
type DispatchResult struct {
    EventID   string // ID do evento no outbox
    MessageID string // ID atribuido pelo broker (para tracing)
    Err       error
}

// Dispatcher envia mensagens para um broker externo.
// Implementacoes: sqsdispatcher, snsdispatcher, kafkadispatcher.
type Dispatcher interface {
    // Dispatch envia um batch de mensagens.
    // Retorna um resultado por mensagem (permite falha parcial).
    Dispatch(ctx context.Context, msgs []Message) []DispatchResult
}

// Router resolve o destino de uma mensagem (topic, queue URL, subject).
// Permite roteamento dinamico baseado no tipo do evento.
type Router func(msg Message) string
```

### 4.4 Relay

```go
// Relay processa eventos pendentes e os despacha via Dispatcher.
type Relay interface {
    // Start inicia o processamento em background.
    // Bloqueia ate ctx ser cancelado.
    Start(ctx context.Context) error

    // Stop sinaliza parada graceful (drena batch atual).
    Stop(ctx context.Context) error
}
```

---

## 5. Estrutura do Repositorio

```text
go-outbox/
├── outbox.go                  # Event, Status, Metadata (tipos core)
├── store.go                   # Store interface + DBTX
├── dispatcher.go              # Dispatcher, Message, Router interfaces
├── relay.go                   # Relay interface
├── options.go                 # Functional options (RetryConfig, etc.)
├── errors.go                  # Erros sentinela do modulo
├── doc.go                     # Package-level documentation
│
├── pgstore/                   # PostgreSQL implementation
│   ├── store.go               # Store usando database/sql + SKIP LOCKED
│   ├── store_test.go          # Testes com go-sqlmock
│   ├── listen.go              # LISTEN/NOTIFY helper (low-latency polling)
│   └── migration/
│       └── 001_create_outbox.sql  # Migration de referencia (Goose)
│
├── relay/                     # Polling relay implementation
│   ├── relay.go               # Worker pool + ticker + circuit breaker
│   ├── relay_test.go
│   ├── worker.go              # Worker individual (fetch → dispatch → mark)
│   ├── worker_test.go
│   ├── circuitbreaker.go      # Circuit breaker para o dispatcher
│   ├── circuitbreaker_test.go
│   ├── backoff.go             # Exponential backoff + jitter
│   └── backoff_test.go
│
├── sqsdispatcher/             # AWS SQS adapter
│   ├── dispatcher.go
│   ├── dispatcher_test.go
│   └── options.go             # SQS-specific config (FIFO, dedup, etc.)
│
├── snsdispatcher/             # AWS SNS adapter
│   ├── dispatcher.go
│   ├── dispatcher_test.go
│   └── options.go
│
├── otel/                      # OpenTelemetry instrumentation
│   ├── metrics.go             # Counters, histograms, gauges
│   ├── metrics_test.go
│   ├── trace.go               # Trace propagation (event → broker)
│   └── trace_test.go
│
├── examples/                  # Exemplos de uso
│   └── basic/
│       └── main.go
│
├── go.mod
├── go.sum
├── README.md
├── LICENSE
└── Makefile
```

### Separacao de Dependencias

O `go.mod` principal contem apenas:

```text
require (
    // Nenhuma dep obrigatoria alem de stdlib
)
```

Adapters que puxam deps pesadas ficam isolados para que o `go.sum` do consumidor
nao infle desnecessariamente:

| Sub-package       | Dep externa                          |
| ----------------- | ------------------------------------ |
| `pgstore`         | nenhuma (`database/sql`)             |
| `relay`           | nenhuma                              |
| `otel`            | `go.opentelemetry.io/otel`           |
| `sqsdispatcher`   | `github.com/aws/aws-sdk-go-v2/...`  |
| `snsdispatcher`   | `github.com/aws/aws-sdk-go-v2/...`  |
| `kafkadispatcher` | `github.com/IBM/sarama` (futuro)     |

> **Nota**: Se o isolamento de deps por sub-package nao for suficiente,
> os dispatchers podem virar modulos separados (`go-outbox-sqs`, etc.)
> com seus proprios `go.mod`. Decisao a ser tomada na Fase 2.

---

## 6. Store (Persistencia)

### 6.1 PostgreSQL Store (`pgstore`)

**Query principal — FetchPending com SKIP LOCKED:**

```sql
WITH batch AS (
    SELECT id
    FROM outbox
    WHERE status = 'pending'
      AND next_retry_at <= NOW()
    ORDER BY created_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE outbox
SET status = 'processing'
FROM batch
WHERE outbox.id = batch.id
RETURNING outbox.*;
```

Porque `SKIP LOCKED`:

- `FOR UPDATE` sozinho bloqueia consumers concorrentes
- `SKIP LOCKED` silenciosamente pula rows travadas por outras transacoes
- Combinado com o CTE, cada consumer pega um batch diferente — **zero coordenacao**
- Isso e o que habilita escala horizontal do relay

**Save (dentro de transacao):**

```go
func (s *Store) Save(ctx context.Context, db outbox.DBTX, event *outbox.Event) error {
    _, execErr := db.ExecContext(ctx, `
        INSERT INTO outbox (
            id, aggregate_type, aggregate_id, event_type,
            payload, metadata, status, max_retries,
            next_retry_at, created_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
        event.ID, event.AggregateType, event.AggregateID, event.EventType,
        event.Payload, event.Metadata, outbox.StatusPending, event.MaxRetries,
        event.NextRetryAt, event.CreatedAt,
    )
    return execErr
}
```

O parametro `db outbox.DBTX` aceita tanto `*sql.DB` quanto `*sql.Tx`.
Para atomicidade com a entidade, o caller passa o mesmo `*sql.Tx`:

```go
tx, _ := db.BeginTx(ctx, nil)
repo.CreateWithTx(ctx, tx, user)   // persiste entidade
store.Save(ctx, tx, event)          // persiste evento (mesma TX)
tx.Commit()                         // atomico
```

### 6.2 LISTEN/NOTIFY (Low-Latency)

Para cenarios onde a latencia do polling (ex: 1s) nao e aceitavel:

```sql
-- Trigger criado pela migration
CREATE OR REPLACE FUNCTION outbox_notify() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('outbox_events', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER outbox_after_insert
    AFTER INSERT ON outbox
    FOR EACH ROW EXECUTE FUNCTION outbox_notify();
```

O relay escuta no canal e dispara polling imediato ao receber notificacao,
com fallback para polling periodico (notificacoes podem ser perdidas se a
conexao cair — o polling garante consistencia eventual).

```go
// listen.go
type Listener struct {
    connStr  string
    channel  string
    onNotify func()  // callback que acorda o relay
}
```

**Isso e opcional.** O relay funciona perfeitamente so com polling.
LISTEN/NOTIFY e uma otimizacao para quem precisa de latencia sub-segundo.

### 6.3 Cleanup

```go
func (s *Store) Cleanup(ctx context.Context, retention time.Duration) (int64, error) {
    result, execErr := s.db.ExecContext(ctx, `
        DELETE FROM outbox
        WHERE status = 'dispatched'
          AND dispatched_at < $1`,
        time.Now().Add(-retention),
    )
    if execErr != nil {
        return 0, execErr
    }
    return result.RowsAffected()
}
```

Para alto volume (>1M eventos/dia), a strategy muda para **table partitioning**
com `DROP PARTITION` ao inves de `DELETE` — documentado como guide avancado.

---

## 7. Relay (Processamento)

### 7.1 Estrategia: Polling com Worker Pool

O relay e o coracao do modulo. A estrategia padrao e **polling com SKIP LOCKED**
porque:

- Funciona com qualquer PostgreSQL (sem extensoes)
- Escala horizontalmente sem coordenacao (SKIP LOCKED)
- Simples de operar (zero infra adicional)
- Latencia configuravel (polling interval)

```text
┌──────────────────────────────────────────┐
│                 Relay                     │
│                                          │
│  ┌─────────┐    ┌─────────────────────┐  │
│  │ Ticker  │───→│  FetchPending(N)    │  │
│  │(interval)│   │  (SKIP LOCKED)      │  │
│  └─────────┘    └──────────┬──────────┘  │
│                            │              │
│       ┌────────────────────┼─────────┐   │
│       ▼          ▼         ▼         ▼   │
│   ┌────────┐ ┌────────┐ ┌────────┐      │
│   │Worker 1│ │Worker 2│ │Worker N│      │
│   └───┬────┘ └───┬────┘ └───┬────┘      │
│       │          │          │            │
│       ▼          ▼          ▼            │
│   ┌──────────────────────────────────┐   │
│   │    Circuit Breaker               │   │
│   │    (protege o dispatcher)        │   │
│   └─────────────┬────────────────────┘   │
│                 │                        │
│                 ▼                        │
│   ┌──────────────────────────────────┐   │
│   │    Dispatcher.Dispatch(batch)    │   │
│   └──────────────────────────────────┘   │
└──────────────────────────────────────────┘
```

### 7.2 Configuracao

```go
// options.go

type RelayConfig struct {
    // Polling
    PollInterval  time.Duration // Intervalo entre polls (default: 1s)
    BatchSize     int           // Eventos por poll (default: 100)

    // Workers
    WorkerCount   int           // Goroutines de dispatch (default: 5)

    // Circuit Breaker
    CBFailureThreshold int           // Falhas consecutivas para abrir (default: 5)
    CBResetTimeout     time.Duration // Tempo para tentar half-open (default: 30s)

    // Cleanup
    CleanupInterval time.Duration // Intervalo de limpeza (default: 1h)
    RetentionPeriod time.Duration // Retencao de eventos dispatched (default: 72h)

    // LISTEN/NOTIFY (opcional)
    EnableNotify   bool   // Habilita LISTEN/NOTIFY (default: false)
    NotifyChannel  string // Canal PostgreSQL (default: "outbox_events")
}
```

### 7.3 Lifecycle

```go
func (r *PollingRelay) Start(ctx context.Context) error {
    g, gCtx := errgroup.WithContext(ctx)

    // Worker pool
    eventCh := make(chan []*outbox.Event, r.cfg.WorkerCount)
    for i := 0; i < r.cfg.WorkerCount; i++ {
        g.Go(func() error {
            return r.worker(gCtx, eventCh)
        })
    }

    // Poller
    g.Go(func() error {
        return r.poll(gCtx, eventCh)
    })

    // Cleanup (background)
    g.Go(func() error {
        return r.cleanup(gCtx)
    })

    // LISTEN/NOTIFY (opcional)
    if r.cfg.EnableNotify {
        g.Go(func() error {
            return r.listen(gCtx)
        })
    }

    return g.Wait()
}
```

### 7.4 Relay como Goroutine vs Processo Separado

O modulo suporta **ambos** — a decisao e do servico consumidor:

| Modo | Como usar | Quando |
|------|-----------|--------|
| **Goroutine** | `go relay.Start(ctx)` no `main.go` | Servicos simples, baixo volume |
| **Processo separado** | `cmd/relay/main.go` dedicado | Alto volume, escala independente, isolamento de falha |

O modulo nao opina — ele expoe `Start(ctx)` que bloqueia ate `ctx` ser cancelado.
O caller decide se roda no mesmo binario ou em outro.

**Exemplo — goroutine (simples):**

```go
// cmd/api/main.go
go relay.Start(ctx)  // roda em background junto com o HTTP server
```

**Exemplo — processo separado (escalavel):**

```go
// cmd/relay/main.go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
    defer cancel()

    store := pgstore.New(db)
    dispatcher := sqsdispatcher.New(sqsClient, queueURL)
    r := relay.New(store, dispatcher, relay.WithPollInterval(500*time.Millisecond))

    if startErr := r.Start(ctx); startErr != nil {
        log.Fatal(startErr)
    }
}
```

---

## 8. Dispatcher (Entrega)

### 8.1 Interface Batch-Oriented

A interface `Dispatcher` e batch-oriented porque:

- SQS suporta `SendMessageBatch` (ate 10 mensagens)
- SNS suporta `PublishBatch` (ate 10 mensagens)
- Kafka produz nativamente em batch
- Reduz round-trips ao broker

Cada dispatcher adapter mapeia `Message` para os conceitos nativos do broker:

| Conceito      | SQS                  | SNS                  | Kafka            |
| ------------- | -------------------- | -------------------- | ---------------- |
| Destino       | Queue URL            | Topic ARN            | Topic            |
| Ordenacao     | MessageGroupId (FIFO)| N/A                  | Partition Key    |
| Deduplicacao  | MessageDeduplicationId (FIFO) | N/A         | Idempotent producer |
| Batch maximo  | 10                   | 10                   | Configuravel     |
| Metadata      | MessageAttributes    | MessageAttributes    | Headers          |

### 8.2 Router

O `Router` resolve o destino de cada mensagem:

```go
// Default: aggregate_type.event_type → "user.created"
func DefaultRouter(msg outbox.Message) string {
    return msg.AggregateType + "." + msg.EventType
}

// Custom: mapeamento explicito
router := outbox.RouterFunc(func(msg outbox.Message) string {
    switch msg.EventType {
    case "user.created", "user.updated":
        return "https://sqs.us-east-1.amazonaws.com/123/user-events"
    case "order.completed":
        return "arn:aws:sns:us-east-1:123:order-events"
    default:
        return "arn:aws:sns:us-east-1:123:default"
    }
})
```

### 8.3 SQS Dispatcher (Exemplo)

```go
// sqsdispatcher/dispatcher.go

type SQSDispatcher struct {
    client  *sqs.Client
    router  outbox.Router
    opts    Options
}

type Options struct {
    FIFO           bool   // usar MessageGroupId/DeduplicationId
    GroupIDFunc     func(msg outbox.Message) string // default: AggregateID
}

func (d *SQSDispatcher) Dispatch(ctx context.Context, msgs []outbox.Message) []outbox.DispatchResult {
    // Agrupa por queue URL (router pode retornar destinos diferentes)
    // Envia em batches de 10 (limite SQS)
    // Retorna resultado por mensagem
}
```

---

## 9. Retry Strategy

### 9.1 Exponential Backoff com Full Jitter

```go
// backoff.go

type BackoffConfig struct {
    BaseDelay   time.Duration // delay base (default: 1s)
    MaxDelay    time.Duration // delay maximo (default: 5min)
    MaxRetries  int           // tentativas maximas (default: 5)
    Multiplier  float64       // fator de multiplicacao (default: 2.0)
}

func DefaultBackoffConfig() BackoffConfig {
    return BackoffConfig{
        BaseDelay:  1 * time.Second,
        MaxDelay:   5 * time.Minute,
        MaxRetries: 5,
        Multiplier: 2.0,
    }
}

// NextDelay calcula o proximo delay com full jitter.
// Full jitter (random entre 0 e delay calculado) performa melhor
// que equal jitter sob contencao (referencia: AWS Architecture Blog).
func (c BackoffConfig) NextDelay(retryCount int) time.Duration {
    delay := float64(c.BaseDelay) * math.Pow(c.Multiplier, float64(retryCount))
    if delay > float64(c.MaxDelay) {
        delay = float64(c.MaxDelay)
    }
    // Full jitter: uniform random entre 0 e delay
    jittered := time.Duration(rand.Int63n(int64(delay)))
    return jittered
}
```

**Progressao com MaxRetries=5, BaseDelay=1s, Multiplier=2.0:**

| Tentativa | Delay calculado | Range com jitter |
| --------- | --------------- | ---------------- |
| 1         | 2s              | 0–2s             |
| 2         | 4s              | 0–4s             |
| 3         | 8s              | 0–8s             |
| 4         | 16s             | 0–16s            |
| 5         | 32s             | 0–32s            |
| 6+        | → StatusDead    | movido para DLQ  |

### 9.2 Dead Letter

Quando `retry_count >= max_retries`:

1. Status muda para `StatusDead`
2. Evento permanece na tabela outbox (nao e deletado pelo cleanup)
3. Metrica `outbox_events_dead_total` e incrementada
4. **Alerta deve ser configurado** — dead letter requer atencao humana

Replay de dead letters:

```sql
-- Apos corrigir a causa raiz, reprocessar:
UPDATE outbox
SET status = 'pending', retry_count = 0, next_retry_at = NOW(), last_error = NULL
WHERE status = 'dead' AND event_type = 'user.created';
```

O modulo pode expor um helper para isso:

```go
func (s *Store) ReplayDead(ctx context.Context, filter ReplayFilter) (int64, error)
```

### 9.3 Circuit Breaker

Protege o dispatcher quando o broker esta fora do ar:

```go
// circuitbreaker.go

type State int

const (
    StateClosed   State = iota // normal — requests passam
    StateOpen                  // broker fora — requests bloqueados
    StateHalfOpen              // testando — 1 request passa
)

type CircuitBreaker struct {
    mu                sync.Mutex
    state             State
    failures          int
    failureThreshold  int           // default: 5
    resetTimeout      time.Duration // default: 30s
    lastFailure       time.Time
}

func (cb *CircuitBreaker) Allow() bool {
    // Closed → permite
    // Open → bloqueia (a menos que resetTimeout tenha passado → HalfOpen)
    // HalfOpen → permite 1 request
}

func (cb *CircuitBreaker) RecordSuccess() { /* Closed */ }
func (cb *CircuitBreaker) RecordFailure() { /* incrementa ou abre */ }
```

**Comportamento no relay:**

- Circuit **closed**: dispatch normal
- Circuit **open**: pula o ciclo de dispatch, eventos ficam pending
- Circuit **half-open**: tenta 1 batch, se ok fecha, se falha reabre

Isso evita enfileirar requests contra um broker que sabidamente esta fora.

---

## 10. Observabilidade

### 10.1 Metricas (OpenTelemetry)

| Metrica                              | Tipo          | Labels                            | Descricao                          |
| ------------------------------------ | ------------- | --------------------------------- | ---------------------------------- |
| `outbox.events.saved`                | Counter       | aggregate_type, event_type        | Eventos gravados no outbox         |
| `outbox.events.dispatched`           | Counter       | aggregate_type, event_type        | Eventos enviados com sucesso       |
| `outbox.events.failed`               | Counter       | aggregate_type, event_type, error | Tentativas de envio que falharam   |
| `outbox.events.dead`                 | Counter       | aggregate_type, event_type        | Eventos movidos para dead letter   |
| `outbox.dispatch.duration`           | Histogram     | aggregate_type                    | Tempo de envio ao broker (ms)      |
| `outbox.event.lag`                   | Histogram     | aggregate_type                    | Tempo entre criacao e dispatch (ms)|
| `outbox.pending.count`               | Gauge         | —                                 | Eventos pendentes no momento       |
| `outbox.poll.duration`               | Histogram     | —                                 | Tempo da query de fetch (ms)       |
| `outbox.batch.size`                  | Histogram     | —                                 | Eventos por ciclo de poll          |
| `outbox.circuit_breaker.state`       | Gauge         | —                                 | 0=closed, 1=open, 2=half-open     |
| `outbox.cleanup.deleted`             | Counter       | —                                 | Eventos removidos pelo cleanup     |

**SLI principal**: `outbox.event.lag` — tempo entre criacao do evento e dispatch bem-sucedido.
Se esse numero cresce, o relay nao esta acompanhando a producao.

### 10.2 Traces

O modulo propaga trace context do request original ate o broker:

```text
[HTTP Request] → [Use Case] → [Outbox Save] ···(async)··· [Relay Dispatch] → [SQS]
     span 1         span 2        span 3                       span 4
                                    │                             │
                                    └── trace_id no Metadata ────┘
```

- `Save()` captura `trace_id` e `span_id` do contexto e grava em `event.Metadata`
- `Dispatch()` cria um novo span linkado ao trace original
- O consumer pode extrair o `trace_id` e continuar a trace

### 10.3 Alertas Recomendados

| Alerta                          | Condicao                             | Severidade |
| ------------------------------- | ------------------------------------ | ---------- |
| Event lag alto                  | p95 de `outbox.event.lag` > 30s      | Warning    |
| Event lag critico               | p95 de `outbox.event.lag` > 5min     | Critical   |
| Dead letter                     | `outbox.events.dead` > 0             | Critical   |
| Circuit breaker aberto          | `outbox.circuit_breaker.state` = 1   | Warning    |
| Pending crescendo               | `outbox.pending.count` crescendo 5min| Warning    |

---

## 11. Gestao Transacional

### Abordagem: DBTX Interface

A interface `DBTX` e satisfeita por `*sql.DB`, `*sql.Tx`, `*sqlx.DB` e `*sqlx.Tx`.
O caller decide se esta dentro de uma transacao ou nao:

```go
// Sem transacao (evento independente — raro, mas valido)
store.Save(ctx, db, event)

// Com transacao (atomico com a entidade — caso padrao)
tx, _ := db.BeginTx(ctx, nil)
repo.CreateWithTx(ctx, tx, user)
store.Save(ctx, tx, event)
tx.Commit()
```

### Porque nao Unit of Work

O UoW (Unit of Work) pattern e mais elegante, mas adiciona uma camada de abstracao
que **nao existe hoje no boilerplate**. Os use cases atuais trabalham com interfaces
simples (Repository) e builder pattern para deps opcionais.

Introduzir UoW so para o outbox criaria inconsistencia arquitetural. Se no futuro
o boilerplate migrar para UoW de forma geral, o outbox module pode adicionar um
adapter `uow.go` sem breaking change.

### Quem gerencia a transacao?

**O use case.** Ele ja conhece o Repository e agora tambem conhece o Outbox Store
(via interface). Ele abre a TX, usa ambos, e comita:

```go
func (uc *CreateUseCase) Execute(ctx context.Context, input dto.CreateInput) (*dto.CreateOutput, error) {
    // ... validacao, criacao da entidade ...

    tx, beginErr := uc.db.BeginTx(ctx, nil)
    if beginErr != nil {
        return nil, fmt.Errorf("beginning transaction: %w", beginErr)
    }
    defer tx.Rollback()

    if saveErr := uc.Repo.CreateWithTx(ctx, tx, entity); saveErr != nil {
        return nil, saveErr
    }

    if outboxErr := uc.Outbox.Save(ctx, tx, &outbox.Event{
        AggregateType: "user",
        AggregateID:   entity.ID.String(),
        EventType:     "user.created",
        Payload:       mustMarshal(entity),
    }); outboxErr != nil {
        return nil, outboxErr
    }

    if commitErr := tx.Commit(); commitErr != nil {
        return nil, fmt.Errorf("committing transaction: %w", commitErr)
    }

    return output, nil
}
```

**Nota**: Isso requer que o Repository exponha um metodo `CreateWithTx(ctx, tx, entity)`.
O modulo outbox nao forca isso — e responsabilidade do servico consumidor adaptar
seus repositories para aceitar `DBTX` quando precisar de atomicidade com o outbox.

---

## 12. Escalabilidade e Ordering

### 12.1 Escala Horizontal

Multiplas instancias do relay podem rodar em paralelo graças ao `SKIP LOCKED`:

```text
Relay Instance A                 Relay Instance B
     │                                │
     ▼                                ▼
SELECT ... SKIP LOCKED           SELECT ... SKIP LOCKED
(pega batch 1-100)               (pega batch 101-200)
     │                                │
     ▼                                ▼
Dispatch → SQS                   Dispatch → SQS
```

Nao precisa de coordenacao, leader election, ou locks distribuidos.

### 12.2 Ordering Guarantees

| Garantia            | Como                                         | Trade-off               |
| ------------------- | -------------------------------------------- | ----------------------- |
| **Nenhuma** (default) | Workers processam eventos em qualquer ordem | Maximo throughput       |
| **Per-aggregate**   | Particionar por `aggregate_id`               | Throughput reduzido     |
| **Global**          | Single consumer, sem paralelismo             | Throughput minimo       |

**Per-aggregate ordering** (quando necessario):

```sql
-- Cada relay instance processa uma particao
SELECT ... FROM outbox
WHERE status = 'pending'
  AND hashtext(aggregate_id) % $1 = $2  -- $1=total partitions, $2=my partition
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT $3
```

O modulo expoe isso via configuracao:

```go
relay.New(store, dispatcher,
    relay.WithPartitioning(totalPartitions, myPartition),
)
```

### 12.3 Throughput Estimado

| Configuracao                          | Throughput esperado   |
| ------------------------------------- | --------------------- |
| 1 relay, 5 workers, poll 1s, batch 100 | ~500 events/s        |
| 3 relays, 5 workers, poll 500ms, batch 200 | ~6.000 events/s |
| 5 relays, 10 workers, poll 100ms, batch 500 | ~25.000 events/s |

Para >25k events/s, considerar CDC (Debezium/WAL) — fora do escopo inicial
do modulo, mas a interface `Relay` permite plugar uma implementacao CDC.

---

## 13. Cleanup e Retencao

### Estrategia Padrao: DELETE Periodico

```go
// Roda como goroutine no relay (configuravel)
func (r *PollingRelay) cleanup(ctx context.Context) error {
    ticker := time.NewTicker(r.cfg.CleanupInterval) // default: 1h
    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            deleted, cleanupErr := r.store.Cleanup(ctx, r.cfg.RetentionPeriod)
            if cleanupErr != nil {
                slog.Warn("outbox cleanup failed", "error", cleanupErr)
                continue
            }
            slog.Info("outbox cleanup completed", "deleted", deleted)
        }
    }
}
```

### Estrategia Avancada: Table Partitioning

Para alto volume, documentado como guide:

```sql
-- Particionamento por range temporal (semanal)
CREATE TABLE outbox (...) PARTITION BY RANGE (created_at);

-- Criar particao da semana
CREATE TABLE outbox_2026_w13 PARTITION OF outbox
    FOR VALUES FROM ('2026-03-23') TO ('2026-03-30');

-- Limpar: DROP ao inves de DELETE (O(1) vs O(n))
DROP TABLE outbox_2026_w11;
```

### Politicas de Retencao

| Status       | Retencao recomendada | Motivo                         |
| ------------ | -------------------- | ------------------------------ |
| `dispatched` | 24h–72h              | Debugging, audit trail         |
| `dead`       | Indefinido           | Requer intervencao manual      |
| `failed`     | Ate retry esgotar    | Retry automatico em andamento  |
| `processing` | Timeout (5min)       | Se nao resolver, volta pending |

**Stale processing detection**: eventos em `processing` ha mais de 5min
provavelmente vieram de um relay que crashou. O cleanup os revert para `pending`:

```sql
UPDATE outbox SET status = 'pending'
WHERE status = 'processing' AND updated_at < NOW() - INTERVAL '5 minutes';
```

---

## 14. Migration de Referencia

Incluida no modulo como referencia (`pgstore/migration/001_create_outbox.sql`):

```sql
-- +goose Up

CREATE TABLE IF NOT EXISTS outbox (
    id              UUID        PRIMARY KEY,
    aggregate_type  TEXT        NOT NULL,
    aggregate_id    TEXT        NOT NULL,
    event_type      TEXT        NOT NULL,
    payload         JSONB       NOT NULL,
    metadata        JSONB       NOT NULL DEFAULT '{}',
    status          TEXT        NOT NULL DEFAULT 'pending',
    retry_count     INTEGER     NOT NULL DEFAULT 0,
    max_retries     INTEGER     NOT NULL DEFAULT 5,
    next_retry_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dispatched_at   TIMESTAMPTZ,

    CONSTRAINT chk_outbox_status CHECK (
        status IN ('pending', 'processing', 'dispatched', 'failed', 'dead')
    )
);

-- Indice principal: polling de eventos pendentes (partial index)
CREATE INDEX idx_outbox_pending
    ON outbox (next_retry_at, created_at)
    WHERE status = 'pending';

-- Indice para ordering per-aggregate
CREATE INDEX idx_outbox_aggregate
    ON outbox (aggregate_type, aggregate_id, created_at);

-- Indice para cleanup de eventos dispatched
CREATE INDEX idx_outbox_cleanup
    ON outbox (dispatched_at)
    WHERE status = 'dispatched';

-- Indice para monitoramento de dead letters
CREATE INDEX idx_outbox_dead
    ON outbox (created_at)
    WHERE status = 'dead';

-- Funcao + trigger para LISTEN/NOTIFY (opcional)
CREATE OR REPLACE FUNCTION outbox_notify() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('outbox_events', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER outbox_after_insert
    AFTER INSERT ON outbox
    FOR EACH ROW EXECUTE FUNCTION outbox_notify();

-- +goose Down

DROP TRIGGER IF EXISTS outbox_after_insert ON outbox;
DROP FUNCTION IF EXISTS outbox_notify();
DROP TABLE IF EXISTS outbox;
```

---

## 15. Exemplo de Integracao

### 15.1 go.mod do servico consumidor

```text
require (
    github.com/jrmarcello/go-outbox v0.1.0
)
```

### 15.2 Wiring no server.go (boilerplate)

```go
import (
    "github.com/jrmarcello/go-outbox"
    "github.com/jrmarcello/go-outbox/pgstore"
    "github.com/jrmarcello/go-outbox/relay"
    "github.com/jrmarcello/go-outbox/sqsdispatcher"
)

func buildDependencies(...) router.Dependencies {
    // ... repos, cache, etc ...

    // Outbox (opcional)
    var outboxStore outbox.Store
    var outboxRelay outbox.Relay

    if cfg.Outbox.Enabled {
        // Store
        outboxStore = pgstore.New(cluster.Writer())

        // Dispatcher (SQS)
        sqsClient := sqs.NewFromConfig(awsCfg)
        dispatcher := sqsdispatcher.New(sqsClient,
            sqsdispatcher.WithRouter(outbox.DefaultRouter),
        )

        // Relay
        outboxRelay = relay.New(outboxStore, dispatcher,
            relay.WithPollInterval(cfg.Outbox.PollInterval),
            relay.WithBatchSize(cfg.Outbox.BatchSize),
            relay.WithWorkerCount(cfg.Outbox.WorkerCount),
        )
    }

    // Use cases com outbox opcional (builder pattern)
    createUC := useruc.NewCreateUseCase(repo).WithOutbox(outboxStore)

    // Start relay em background
    if outboxRelay != nil {
        go func() {
            if relayErr := outboxRelay.Start(ctx); relayErr != nil {
                slog.Error("outbox relay failed", "error", relayErr)
            }
        }()
    }

    return router.Dependencies{...}
}
```

### 15.3 Use case com outbox

```go
// internal/usecases/user/interfaces/outbox.go
package interfaces

import (
    "context"
    "database/sql"
    "encoding/json"
    "time"
)

// EventStore grava eventos na outbox (interface definida pelo use case).
type EventStore interface {
    Save(ctx context.Context, db DBTX, event *OutboxEvent) error
}

// DBTX e satisfeito por *sql.DB e *sql.Tx.
type DBTX interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
```

```go
// internal/usecases/user/create.go
type CreateUseCase struct {
    Repo   interfaces.Repository
    Outbox interfaces.EventStore // nil = sem outbox
    DB     interfaces.DBTX       // para gerenciar TX quando outbox esta ativo
}

func (uc *CreateUseCase) WithOutbox(store interfaces.EventStore, db interfaces.DBTX) *CreateUseCase {
    uc.Outbox = store
    uc.DB = db
    return uc
}
```

---

## 16. Fases de Implementacao

### Fase 1 — Core + PostgreSQL Store (MVP)

**Objetivo**: modulo funcional com polling relay e interface de dispatcher.

| Tarefa                                    | Descricao                                |
| ----------------------------------------- | ---------------------------------------- |
| Repositorio + go.mod                      | Setup inicial com CI                     |
| `outbox.go`, `store.go`, `dispatcher.go`  | Tipos e interfaces centrais              |
| `options.go`, `errors.go`                 | Functional options, erros sentinela      |
| `pgstore/store.go`                        | SKIP LOCKED, Save, FetchPending, Mark*   |
| `pgstore/migration/`                      | Migration de referencia                  |
| `relay/relay.go`                          | Polling relay com worker pool            |
| `relay/backoff.go`                        | Exponential backoff + jitter             |
| Testes unitarios                          | pgstore (go-sqlmock), relay (mocks)      |
| Testes de integracao                      | TestContainers (Postgres)                |
| README.md                                 | Documentacao + exemplos                  |

**Entregavel**: `go get github.com/jrmarcello/go-outbox` funcional com pgstore + relay.
Dispatcher e um `interface` — o consumidor implementa.

### Fase 2 — Dispatchers + Observabilidade

| Tarefa                           | Descricao                                  |
| -------------------------------- | ------------------------------------------ |
| `sqsdispatcher/`                 | Adapter SQS com batch + FIFO support       |
| `snsdispatcher/`                 | Adapter SNS com batch                      |
| `relay/circuitbreaker.go`        | Circuit breaker para o dispatcher          |
| `otel/metrics.go`                | Metricas OTel (counters, histograms)       |
| `otel/trace.go`                  | Trace propagation                          |
| `pgstore/listen.go`             | LISTEN/NOTIFY para low-latency             |
| Cleanup automatico               | Goroutine de retencao no relay             |
| Stale processing recovery        | Revert processing → pending apos timeout   |

**Entregavel**: dispatchers prontos para AWS, observabilidade completa.

### Fase 3 — Advanced Features

| Tarefa                           | Descricao                                  |
| -------------------------------- | ------------------------------------------ |
| Per-aggregate ordering           | Partitioning por aggregate_id              |
| `ReplayDead()`                   | Helper para reprocessar dead letters       |
| Partitioning guide               | Documentacao para table partitioning       |
| `kafkadispatcher/` (se demanda)  | Adapter Kafka                              |
| Benchmarks                       | Throughput tests com diferentes configs    |

---

## 17. Decisoes Arquiteturais

| #  | Decisao                        | Opcoes consideradas                  | Escolha             | Motivo                                                                 |
| -- | ------------------------------ | ------------------------------------ | ------------------- | ---------------------------------------------------------------------- |
| 1  | Relay strategy                 | Polling, CDC (Debezium), WAL reader  | **Polling**         | Zero infra adicional, funciona com qualquer Postgres, SKIP LOCKED escala horizontalmente. CDC pode ser plugado via interface Relay. |
| 2  | Low-latency boost              | Polling rapido (100ms), LISTEN/NOTIFY, CDC | **LISTEN/NOTIFY (opcional)** | Latencia sub-segundo sem CDC. Fallback para polling garante confiabilidade. |
| 3  | Retry strategy                 | Fixed delay, linear, exponential, exponential+jitter | **Exponential + full jitter** | Evita thundering herd. Full jitter performa melhor que equal jitter sob contencao (ref: AWS Architecture Blog). |
| 4  | Dead letter handling           | Retry infinito, DLQ table separada, status na mesma tabela | **Status `dead` na mesma tabela** | Simples, sem tabela extra. Query de replay e trivial. |
| 5  | Gestao transacional            | Pass `*sql.Tx`, DBTX interface, Unit of Work, Context-based | **DBTX interface** | Flexivel (`*sql.DB` ou `*sql.Tx`), sem abstracao extra, compativel com sqlx. |
| 6  | Dispatcher interface           | Single message, batch                | **Batch**           | SQS/SNS suportam batch nativamente. Reduz round-trips. Single message e batch de 1. |
| 7  | Circuit breaker                | No CB, por-evento, por-broker        | **Por-broker**      | Protege contra outage sistemico do broker. Retry per-evento ja e coberto pelo backoff. |
| 8  | Onde fica cada broker adapter  | Mesmo pacote, sub-packages, modulos separados | **Sub-packages** (fase 1), modulos separados se necessario | Isolamento de deps sem overhead de multi-repo. Migra para modulos se o `go.sum` inflar. |
| 9  | Ordering                       | Nenhuma, per-aggregate, global       | **Nenhuma (default), per-aggregate (opt-in)** | Maximo throughput por default. Per-aggregate via partitioning quando necessario. Global nao escala. |
| 10 | Cleanup                        | DELETE periodico, partition drop, nenhum | **DELETE periodico (default)**, partition drop documentado como guide | Simples para a maioria dos casos. Partitioning e overkill para <1M events/dia. |
| 11 | Relay como goroutine ou processo | Goroutine, processo separado        | **Ambos (caller decide)** | O modulo expoe `Start(ctx)`. Goroutine para servicos simples, processo separado para escala. |

---

## 18. Referencias

- [Microservices Patterns — Chris Richardson (Outbox Pattern)](https://microservices.io/patterns/data/transactional-outbox.html)
- [AWS Architecture Blog — Exponential Backoff and Jitter](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/)
- [PostgreSQL SKIP LOCKED](https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE)
- [PostgreSQL LISTEN/NOTIFY](https://www.postgresql.org/docs/current/sql-listen.html)
- [ThreeDotsLabs/watermill](https://github.com/ThreeDotsLabs/watermill) — referencia de design (publisher/subscriber)
- [nikolayk812/pgx-outbox](https://github.com/nikolayk812/pgx-outbox) — referencia de outbox pattern Go nativo
