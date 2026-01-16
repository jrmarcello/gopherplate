# Cache Strategy

Este guia explica o padrão de cache implementado neste projeto e como utilizá-lo.

---

## Cache-Aside Pattern

O **Cache-Aside** (ou Lazy Loading) é o padrão de cache mais comum para aplicações web. A aplicação é responsável por gerenciar o cache explicitamente.

### Fluxo de Leitura (GET)

```mermaid
sequenceDiagram
    participant App as Use Case
    participant Cache as Redis
    participant DB as PostgreSQL
    
    App->>Cache: 1. Get(key)
    alt Cache Hit
        Cache-->>App: Dados encontrados
    else Cache Miss
        Cache-->>App: Não encontrado
        App->>DB: 2. Query
        DB-->>App: Dados
        App->>Cache: 3. Set(key, dados)
    end
```

### Fluxo de Escrita (UPDATE/DELETE)

```mermaid
sequenceDiagram
    participant App as Use Case
    participant Cache as Redis
    participant DB as PostgreSQL
    
    App->>DB: 1. Update/Delete
    DB-->>App: Confirmação
    App->>Cache: 2. Delete(key)
    Note right of Cache: Cache invalidado
```

---

## Implementação

### Interface

A interface de cache está definida em [`internal/domain/shared/interfaces/cache.go`](file:///internal/domain/shared/interfaces/cache.go):

```go
type Cache interface {
    Get(ctx context.Context, key string, dest interface{}) error
    Set(ctx context.Context, key string, value interface{}) error
    Delete(ctx context.Context, key string) error
    Ping(ctx context.Context) error
}
```

### Cliente Redis

A implementação está em [`internal/infrastructure/cache/redis.go`](file:///internal/infrastructure/cache/redis.go):

| Método | Descrição |
| -------- | ----------- |
| `Get` | Busca e deserializa do cache |
| `Set` | Serializa e armazena com TTL |
| `Delete` | Invalida uma chave |
| `Ping` | Health check da conexão |

### Uso nos Use Cases

```go
// GET - Cache-first
func (uc *GetUseCase) Execute(ctx context.Context, input dto.GetInput) (*dto.GetOutput, error) {
    cacheKey := "entity:" + input.ID

    // 1. Tentar cache primeiro
    if uc.Cache != nil {
        var cached dto.GetOutput
        if err := uc.Cache.Get(ctx, cacheKey, &cached); err == nil {
            return &cached, nil // Cache hit
        }
    }

    // 2. Cache miss - buscar no DB
    entity, err := uc.Repo.FindByID(ctx, id)
    if err != nil {
        return nil, err
    }

    // 3. Armazenar no cache
    if uc.Cache != nil {
        uc.Cache.Set(ctx, cacheKey, output)
    }

    return output, nil
}

// UPDATE/DELETE - Invalidar cache
func (uc *UpdateUseCase) Execute(ctx context.Context, input dto.UpdateInput) (*dto.UpdateOutput, error) {
    // ... update no DB ...

    // Invalidar cache
    if uc.Cache != nil {
        uc.Cache.Delete(ctx, "entity:" + input.ID)
    }

    return output, nil
}
```

---

## Configuração

### Variáveis de Ambiente

| Variável | Descrição | Default |
| -------- | ----------- | ------- |
| `REDIS_ENABLED` | Habilita/desabilita cache | `false` |
| `REDIS_URL` | URL de conexão | `redis://localhost:6379` |
| `REDIS_TTL` | Tempo de expiração | `5m` |

### Exemplo `.env`

```bash
REDIS_ENABLED=true
REDIS_URL=redis://localhost:6379
REDIS_TTL=5m
```

---

## Boas Práticas

### ✅ Fazer

- **TTL curto**: Prefira TTL de 1-5 minutos para dados que mudam
- **Graceful degradation**: Se Redis falhar, continue operando (só mais lento)
- **Keys descritivas**: Use padrão `entity:id` para facilitar debug
- **Invalidar nas mutações**: Sempre invalide após Update/Delete

### ❌ Evitar

- **Cache de listas**: Difícil invalidar corretamente
- **TTL muito longo**: Dados podem ficar desatualizados
- **Dependência crítica**: Cache deve ser otimização, não requisito

---

## Testes

### Unitários

Os testes de use case usam mocks para simular o cache:

```go
mockCache.On("Get", mock.Anything, "entity:123", mock.Anything).
    Return(errors.New("cache miss"))
mockCache.On("Set", mock.Anything, "entity:123", mock.Anything).
    Return(nil)
```

### E2E

O teste `TestE2E_CacheBehavior` valida o fluxo completo:

1. Cache miss na primeira leitura
2. Cache hit na segunda leitura  
3. Invalidação após update

---

## Referências

- [Redis Best Practices](https://redis.io/docs/manual/patterns/)
- [Cache-Aside Pattern (Microsoft)](https://learn.microsoft.com/en-us/azure/architecture/patterns/cache-aside)
