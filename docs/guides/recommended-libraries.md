# Bibliotecas Recomendadas

Antes de construir algo do zero, vale verificar se já existe uma solução madura na comunidade Go. Este guia lista bibliotecas pesquisadas e recomendadas para necessidades comuns em microsserviços.

> Última atualização: março 2026.

---

## Resiliência (Circuit Breaker, Retry, Timeout, Bulkhead)

### Recomendação: failsafe-go

| Atributo | Valor |
| -------- | ----- |
| Repositório | [failsafe-go/failsafe-go](https://github.com/failsafe-go/failsafe-go) |
| Stars | ~2.200 |
| Licença | MIT |
| Go version | 1.21+ (generics) |
| Status | Pre-1.0 (v0.9.6), mas muito ativo e próximo da estabilidade |

Cobre **todos os padrões de resiliência** em uma API composable:

- **Circuit Breaker** — 3 estados (closed, open, half-open) com thresholds configuráveis
- **Retry** — exponential backoff, jitter, max attempts, condições customizáveis
- **Timeout** — por operação, context-based
- **Bulkhead** — limita concorrência por serviço/endpoint
- **Rate Limiter** — smooth e bursty
- **Fallback** — valor ou função alternativa em caso de falha
- **Hedge** — speculative execution para reduzir tail latency

Port do Failsafe Java (battle-tested). Políticas são composáveis — encadeie retry + circuit breaker + timeout em uma única chamada.

### Exemplo de uso

```go
import "github.com/failsafe-go/failsafe-go"

// Políticas composáveis
retryPolicy := retrypolicy.Builder[*http.Response]().
    WithMaxAttempts(3).
    WithBackoff(100*time.Millisecond, 10*time.Second).
    WithJitter(0.25).
    Build()

cb := circuitbreaker.Builder[*http.Response]().
    WithFailureThreshold(5).
    WithDelay(30 * time.Second).
    Build()

timeout := timeout.With[*http.Response](2 * time.Second)

// Execução com todas as políticas
response, err := failsafe.Get(
    func() (*http.Response, error) {
        return httpClient.Do(req)
    },
    retryPolicy, cb, timeout,
)
```

### Alternativa: best-of-breed

Se preferir libs estáveis (post-1.0) combinadas manualmente:

| Padrão | Biblioteca | Stars | Versão |
| ------ | ---------- | ----- | ------ |
| Circuit Breaker | [sony/gobreaker](https://github.com/sony/gobreaker) | ~3.600 | v2.4 (estável) |
| Retry | [avast/retry-go](https://github.com/avast/retry-go) | ~2.900 | v5.0 (estável) |
| Backoff | [cenkalti/backoff](https://github.com/cenkalti/backoff) | ~4.000 | v5.0 (estável) |
| Timeout | stdlib `context.WithTimeout` | — | — |

Nesse caso você compõe manualmente. Cada peça é independente e madura.

### Por que não construir

Resiliência é um problema resolvido. As libs acima são mantidas há anos, com milhares de stars e uso em produção. Construir do zero seria reimplementar padrões bem conhecidos sem benefício real.

---

## Criptografia (Encrypt/Decrypt, Hashing, KMS)

### Recomendação: tink-go + golang.org/x/crypto

| Necessidade | Biblioteca | Stars | Notas |
| ----------- | ---------- | ----- | ----- |
| Encrypt/decrypt | [tink-crypto/tink-go](https://github.com/tink-crypto/tink-go) v2 | ~13.500 (monorepo original) | Google. Misuse-resistant, key rotation built-in |
| AWS KMS | [tink-crypto/tink-go-awskms](https://github.com/tink-crypto/tink-go-awskms) | — | Envelope encryption com KMS. Wraps aws-sdk-go-v2 |
| Hashing (senhas) | [golang.org/x/crypto/argon2](https://pkg.go.dev/golang.org/x/crypto/argon2) | ~3.300 | Stdlib-quality. Argon2id (PHC winner) > bcrypt para novos projetos |
| Hashing (geral) | `crypto/sha256` (stdlib) | — | SHA-256 para fingerprints, checksums |
| Token generation | `crypto/rand` (stdlib) | — | Suficiente para tokens seguros |

### Tink — por que usar

Tink foi criado pelo Google para prevenir erros comuns em criptografia:

- **Não expõe algoritmos inseguros** — só primitivas seguras (AES-GCM, ChaCha20-Poly1305)
- **Key rotation automática** — keyset handles gerenciam múltiplas versões de chave
- **Envelope encryption** — `tink-go-awskms` faz GenerateDataKey + encrypt local + store encrypted key
- **Deterministic encryption** — para campos que precisam de busca por igualdade (ex: email lookup)
- **Auditado** — usado em centenas de produtos Google

```go
import (
    "github.com/tink-crypto/tink-go/v2/aead"
    "github.com/tink-crypto/tink-go/v2/keyset"
)

// Gerar keyset
handle, _ := keyset.NewHandle(aead.AES256GCMKeyTemplate())

// Encrypt
primitive, _ := aead.New(handle)
ciphertext, _ := primitive.Encrypt(plaintext, associatedData)

// Decrypt
decrypted, _ := primitive.Decrypt(ciphertext, associatedData)
```

### Hashing de senhas com Argon2id

```go
import "golang.org/x/crypto/argon2"

// Hash
salt := make([]byte, 16)
crypto_rand.Read(salt)
hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

// Argon2id é recomendado sobre bcrypt para novos projetos:
// - Resistente a GPU/ASIC attacks (memory-hard)
// - Vencedor da Password Hashing Competition (PHC)
// - Parametrizável (tempo, memória, paralelismo)
```

### Por que não construir

Criptografia é o pior lugar para reinventar a roda. Tink existe especificamente para que devs não implementem crypto do zero. `x/crypto` é mantido pelo time de Go com auditoria de segurança (Trail of Bits, 2025). Stdlib `crypto/rand` cobre geração de tokens.

---

## Event Bus In-Process (Publish/Subscribe)

### Recomendação: Watermill (GoChannel)

| Atributo | Valor |
| -------- | ----- |
| Repositório | [ThreeDotsLabs/watermill](https://github.com/ThreeDotsLabs/watermill) |
| Stars | ~9.600 |
| Licença | MIT |
| Go version | 1.21+ |
| Status | v1.5.1 (estável, production-ready) |

Watermill é um framework de eventos com um componente **GoChannel** que funciona 100% in-process (sem broker externo):

- **315k publish/s** e **138k subscribe/s** para GoChannel (benchmark oficial)
- Interface uniforme `Publisher`/`Subscriber` — troque GoChannel por Kafka, SQS, NATS, RabbitMQ sem mudar handlers
- Middleware built-in: retry, throttle, correlation ID, deduplication, poison queue, metrics
- Módulo CQRS incluso
- Stress-tested com race detector

### Exemplo de uso

```go
import (
    "github.com/ThreeDotsLabs/watermill"
    "github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
)

// In-process (dev/serviços simples)
pubSub := gochannel.NewGoChannel(gochannel.Config{}, watermill.NewStdLogger(false, false))

// Publish
msg := message.NewMessage(watermill.NewUUID(), []byte(`{"user_id": "123"}`))
pubSub.Publish("user.created", msg)

// Subscribe
messages, _ := pubSub.Subscribe(context.Background(), "user.created")
go func() {
    for msg := range messages {
        // handle event
        msg.Ack()
    }
}()

// Quando precisar escalar → troque GoChannel por Kafka/SQS (mesma interface)
```

### Como integra com Clean Architecture

```text
usecases/user/interfaces/
    publisher.go → define Publisher interface (porta)

infrastructure/messaging/
    watermill.go → implementa com GoChannel ou Kafka (adapter)

cmd/api/server.go
    → wira Publisher no buildDependencies()
```

### Por que não construir

Um event bus in-process parece simples, mas middleware (retry, dedup, dead letter), race safety, e migration path para brokers externos adicionam complexidade real. Watermill resolve tudo isso com uma API limpa e 9.6k stars de validação.

---

## Notificações Multi-Canal (Slack, Email, Webhook)

### Recomendação: nikoksr/notify

| Atributo | Valor |
| -------- | ----- |
| Repositório | [nikoksr/notify](https://github.com/nikoksr/notify) |
| Stars | ~3.700 |
| Licença | MIT |
| Status | v1.5.0 (estável, production-ready) |

32 integrações de notificação, cada uma como sub-package independente (importa só o que usa):

- **Slack** (webhook), **Amazon SES**, **Amazon SNS**, **Email (SMTP)**, **Discord**, **MS Teams**, **Telegram**, **PagerDuty**, **Webhook genérico**, Google Chat, Mattermost, Twilio, WhatsApp, e mais 19.

### Exemplo de uso

```go
import (
    "github.com/nikoksr/notify"
    "github.com/nikoksr/notify/service/slack"
    "github.com/nikoksr/notify/service/mail"
)

// Setup
slackSvc := slack.New("webhook-url")
slackSvc.AddReceivers("#alerts")

emailSvc := mail.New("noreply@appmax.com.br", "smtp.host:587")
emailSvc.AddReceivers("oncall@appmax.com.br")

notifier := notify.New()
notifier.UseServices(slackSvc, emailSvc)

// Enviar (fanout para todos os canais)
notifier.Send(ctx, "Deploy falhou", "payment-service deploy failed on production")
```

### Alternativa: slack-go/slack

Se o único canal for Slack e precisar de features avançadas (Block Kit, interactive messages, threads):

| Biblioteca | Stars | Escopo |
| ---------- | ----- | ------ |
| [slack-go/slack](https://github.com/slack-go/slack) | ~4.900 | SDK completo do Slack (v0.20, muito ativo) |

nikoksr/notify usa slack-go/slack internamente para o adapter de Slack.

### Por que não construir

nikoksr/notify faz exatamente o que o módulo `go-notifier` proposto faria, com 32 canais prontos e v1.5 estável. Construir seria reimplementar adapters para cada serviço sem benefício.

---

## Sagas / Transações Distribuídas

### Recomendação: Temporal ou DTM (avaliar por contexto)

Diferente das categorias acima, sagas não têm uma lib Go leve e production-ready. As opções são **plataformas** com trade-offs distintos:

### Opção A: Temporal (padrão da indústria)

| Atributo | Valor |
| -------- | ----- |
| Repositório | [temporalio/sdk-go](https://github.com/temporalio/sdk-go) |
| Stars | ~19.200 (server) / ~855 (SDK Go) |
| Licença | MIT |
| Usado por | Netflix, Uber, Stripe, Snap, Datadog, HashiCorp |

- Workflow engine completo — sagas são um dos muitos padrões suportados
- Estado durável (sobrevive a crashes e restarts)
- Retry policies, timeouts, heartbeats built-in
- Visibilidade sobre workflows em execução (qual step, o que falhou)
- OpenTelemetry integrado

**Trade-off**: requer **Temporal Server** (Postgres + Cassandra/MySQL + Elasticsearch opcional) ou **Temporal Cloud** (~$200+/mês). É uma plataforma, não uma lib.

**Quando usar**: sagas long-running (minutos a dias), fluxos complexos com branches condicionais, necessidade de visibilidade e durabilidade. Se a equipe já planeja workflow orchestration além de sagas.

### Opção B: DTM (focado em transações distribuídas)

| Atributo | Valor |
| -------- | ----- |
| Repositório | [dtm-labs/dtm](https://github.com/dtm-labs/dtm) |
| Stars | ~10.800 |
| Licença | BSD-3-Clause |
| Usado por | Tencent, ByteDance |

- **Focado** em transações distribuídas: Saga, TCC, XA, 2-phase message, outbox
- Server leve (binário único, usa MySQL/Postgres/Redis como backend)
- Multi-linguagem (Go, Java, Python, PHP, Node.js, C#)
- HTTP e gRPC suportados

**Trade-off**: server dedicado (mais leve que Temporal), documentação majoritariamente em chinês, menos adoção no ocidente.

**Quando usar**: sagas simples/médias onde Temporal é pesado demais. Serviços que já usam diferentes linguagens e precisam de coordenação cross-language.

### Opção C: Construir saga simples + River (DIY)

Para sagas curtas e simples (3-5 steps, tudo síncrono):

```go
// ~50 linhas de código: slice de steps, executa forward, compensa em reverse on error
type Step struct {
    Name       string
    Action     func(ctx context.Context) error
    Compensate func(ctx context.Context) error
}
```

Combinado com [River](https://github.com/riverqueue/river) (~4.900 stars, Postgres-based job queue) para persistência de jobs, se necessário.

**Quando usar**: sagas curtas que não precisam de durabilidade, persistência ou visibilidade avançada. Equipe quer evitar infra adicional.

### Qual escolher?

| Cenário | Recomendação |
| ------- | ------------ |
| Fluxos complexos, long-running, precisam de visibilidade | Temporal |
| Sagas simples, quer evitar Temporal mas precisa de robustez | DTM |
| Sagas curtas (3-5 steps), sem necessidade de persistência | DIY + River |

---

## Resumo

| Necessidade | Construir? | Usar |
| ----------- | ---------- | ---- |
| Resiliência (CB, retry, timeout) | Não | failsafe-go ou sony/gobreaker + avast/retry-go |
| Criptografia (encrypt, hash, KMS) | Não | tink-go + x/crypto |
| Event bus in-process | Não | Watermill GoChannel |
| Notificações multi-canal | Não | nikoksr/notify |
| Sagas / transações distribuídas | Depende | Temporal, DTM, ou DIY (conforme complexidade) |
