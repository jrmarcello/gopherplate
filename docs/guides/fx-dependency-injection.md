# Guia: Uber Fx para Injecao de Dependencia

Este projeto usa **DI manual** via `cmd/api/server.go:buildDependencies()`. Para servicos com muitas dependencias, o [Uber Fx](https://uber-go.github.io/fx/) oferece uma alternativa mais robusta com autowiring, lifecycle hooks e modularizacao.

> **Quando considerar Fx?** Quando `buildDependencies()` ultrapassar ~50 linhas ou quando houver muitos use cases com dependencias opcionais complexas. Para servicos simples, o DI manual e suficiente.

---

## Instalacao

```bash
go get go.uber.org/fx@latest
```

---

## Conceitos

| Conceito | Descricao |
| -------- | --------- |
| `fx.Provide` | Registra um construtor no container (lazy — so instancia quando necessario) |
| `fx.Invoke` | Executa uma funcao no startup (garante que dependencias sao instanciadas) |
| `fx.Module` | Agrupa providers/invokes em um modulo reutilizavel |
| `fx.Lifecycle` | Hooks de `OnStart`/`OnStop` para gerenciar ciclo de vida |
| `fx.Annotate` | Adiciona metadata (tags, interfaces) a providers |
| `fx.In` / `fx.Out` | Structs para multiplos parametros/resultados |

---

## Exemplo: Migrando buildDependencies() para Fx

### Antes (DI manual atual)

```go
// cmd/api/server.go
func buildDependencies(cluster *database.DBCluster, cfg *config.Config, ...) router.Dependencies {
    repo := repository.NewUserRepository(cluster)
    redisClient, _ := pkgcache.NewRedisClient(cfg.Redis)
    createUC := useruc.NewCreateUseCase(repo)
    getUC := useruc.NewGetUseCase(repo).WithCache(redisClient)
    // ... mais 10 linhas de wiring ...
    userHandler := handler.NewUserHandler(createUC, getUC, listUC, updateUC, deleteUC, metrics)
    return router.Dependencies{...}
}
```

### Depois (com Fx)

#### 1. Modulo de Database

```go
// internal/infrastructure/db/module.go
package db

import (
    "go.uber.org/fx"

    "github.com/jrmarcello/gopherplate/config"
    "github.com/jrmarcello/gopherplate/pkg/database"
)

var Module = fx.Module("database",
    fx.Provide(func(cfg *config.Config) (*database.DBCluster, error) {
        writerCfg := database.Config{
            DSN:             cfg.DB.GetWriterDSN(),
            MaxOpenConns:    cfg.DB.MaxOpenConns,
            MaxIdleConns:    cfg.DB.MaxIdleConns,
            ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
            ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
        }
        var readerCfg *database.Config
        if cfg.DB.ReplicaEnabled {
            readerCfg = &database.Config{
                DSN:             cfg.DB.GetReaderDSN(),
                MaxOpenConns:    cfg.DB.ReplicaMaxOpenConns,
                MaxIdleConns:    cfg.DB.ReplicaMaxIdleConns,
                ConnMaxLifetime: cfg.DB.ReplicaConnMaxLifetime,
                ConnMaxIdleTime: cfg.DB.ReplicaConnMaxIdleTime,
            }
        }
        return database.NewDBCluster(writerCfg, readerCfg)
    }),
)
```

#### 2. Modulo de Cache

```go
// pkg/cache/module.go (example — would be added alongside redis.go)
package cache

import (
    "go.uber.org/fx"

    "github.com/jrmarcello/gopherplate/config"
    pkgcache "github.com/jrmarcello/gopherplate/pkg/cache"
)

var Module = fx.Module("cache",
    fx.Provide(func(cfg *config.Config) (*pkgcache.RedisClient, error) {
        return pkgcache.NewRedisClient(pkgcache.RedisConfig{
            URL:          cfg.Redis.URL,
            TTL:          cfg.Redis.TTL,
            Enabled:      cfg.Redis.Enabled,
            PoolSize:     cfg.Redis.PoolSize,
            MinIdleConns: cfg.Redis.MinIdleConns,
            DialTimeout:  cfg.Redis.DialTimeout,
            ReadTimeout:  cfg.Redis.ReadTimeout,
            WriteTimeout: cfg.Redis.WriteTimeout,
        })
    }),
)
```

#### 3. Modulo de Use Cases

```go
// internal/usecases/module.go
package usecases

import (
    "go.uber.org/fx"

    useruc "github.com/jrmarcello/gopherplate/internal/usecases/user"
    "github.com/jrmarcello/gopherplate/internal/usecases/user/interfaces"
    pkgcache "github.com/jrmarcello/gopherplate/pkg/cache"
)

var Module = fx.Module("usecases",
    fx.Provide(
        func(repo interfaces.Repository, cache *pkgcache.RedisClient) *useruc.CreateUseCase {
            return useruc.NewCreateUseCase(repo)
        },
        func(repo interfaces.Repository, cache *pkgcache.RedisClient) *useruc.GetUseCase {
            fg := useruc.NewFlightGroup()
            return useruc.NewGetUseCase(repo).WithCache(cache).WithFlight(fg)
        },
        func(repo interfaces.Repository) *useruc.ListUseCase {
            return useruc.NewListUseCase(repo)
        },
        func(repo interfaces.Repository, cache *pkgcache.RedisClient) *useruc.UpdateUseCase {
            return useruc.NewUpdateUseCase(repo).WithCache(cache)
        },
        func(repo interfaces.Repository, cache *pkgcache.RedisClient) *useruc.DeleteUseCase {
            return useruc.NewDeleteUseCase(repo).WithCache(cache)
        },
    ),
)
```

#### 4. Modulo de HTTP Server com Lifecycle

```go
// internal/infrastructure/web/module.go
package web

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "time"

    "go.uber.org/fx"

    "github.com/jrmarcello/gopherplate/config"
    "github.com/jrmarcello/gopherplate/internal/infrastructure/web/router"
)

var Module = fx.Module("http",
    fx.Provide(func(cfg *config.Config, deps router.Dependencies) *http.Server {
        r := router.Setup(deps)
        return &http.Server{
            Addr:              ":" + cfg.Server.Port,
            Handler:           r,
            ReadHeaderTimeout: 10 * time.Second,
            ReadTimeout:       30 * time.Second,
            WriteTimeout:      30 * time.Second,
            IdleTimeout:       120 * time.Second,
        }
    }),
    fx.Invoke(func(lc fx.Lifecycle, srv *http.Server) {
        lc.Append(fx.Hook{
            OnStart: func(ctx context.Context) error {
                ln, listenErr := net.Listen("tcp", srv.Addr)
                if listenErr != nil {
                    return fmt.Errorf("listening on %s: %w", srv.Addr, listenErr)
                }
                go srv.Serve(ln)
                return nil
            },
            OnStop: func(ctx context.Context) error {
                return srv.Shutdown(ctx)
            },
        })
    }),
)
```

#### 5. main.go com Fx

```go
// cmd/api/main.go
package main

import (
    "go.uber.org/fx"

    "github.com/jrmarcello/gopherplate/config"
    dbmodule "github.com/jrmarcello/gopherplate/internal/infrastructure/db"
    cachemodule "github.com/jrmarcello/gopherplate/pkg/cache"
    webmodule "github.com/jrmarcello/gopherplate/internal/infrastructure/web"
    ucmodule "github.com/jrmarcello/gopherplate/internal/usecases"
)

func main() {
    fx.New(
        // Config
        fx.Provide(config.Load),

        // Infrastructure
        dbmodule.Module,
        cachemodule.Module,

        // Business logic
        ucmodule.Module,

        // HTTP
        webmodule.Module,
    ).Run()
}
```

---

## Lifecycle Hooks

Fx gerencia o ciclo de vida automaticamente. Util para:

```go
// Registrar cleanup de recursos
lc.Append(fx.Hook{
    OnStart: func(ctx context.Context) error {
        // Inicializar recurso (ex: conexao, worker)
        return nil
    },
    OnStop: func(ctx context.Context) error {
        // Cleanup (ex: fechar conexao, parar worker)
        return cluster.Close()
    },
})
```

O graceful shutdown que hoje esta em `runWithGracefulShutdown()` e substituido pelo `fx.Lifecycle` — Fx escuta SIGINT/SIGTERM e chama todos os `OnStop` hooks automaticamente.

---

## Boas Praticas

### Use fx.Module para organizar

```go
// Um modulo por bounded context
var Module = fx.Module("user",
    fx.Provide(
        repository.NewUserRepository,
        NewCreateUseCase,
        NewGetUseCase,
        // ...
    ),
)
```

### Mantenha construtores compatíveis com DI manual

```go
// Construtor funciona com Fx E com DI manual
func NewGetUseCase(repo interfaces.Repository) *GetUseCase {
    return &GetUseCase{repo: repo}
}

// Builder pattern continua funcionando
uc := NewGetUseCase(repo).WithCache(cache).WithFlight(fg)
```

### Use Parameter Objects para muitas deps

```go
type HandlerParams struct {
    fx.In

    CreateUC *useruc.CreateUseCase
    GetUC    *useruc.GetUseCase
    ListUC   *useruc.ListUseCase
    UpdateUC *useruc.UpdateUseCase
    DeleteUC *useruc.DeleteUseCase
    Metrics  *telemetry.Metrics `optional:"true"`
}

func NewUserHandler(p HandlerParams) *UserHandler {
    return &UserHandler{
        CreateUC: p.CreateUC,
        GetUC:    p.GetUC,
        // ...
    }
}
```

### Deps opcionais com `optional:"true"`

```go
type Params struct {
    fx.In

    Repo  interfaces.Repository
    Cache *pkgcache.RedisClient `optional:"true"` // nil se Redis desabilitado
}
```

---

## Tradeoffs

| Aspecto | DI Manual (atual) | Uber Fx |
| ------- | ------------------ | ------- |
| **Simplicidade** | Explicito, facil de seguir | Implicito, requer conhecer Fx |
| **Boilerplate** | Cresce com numero de deps | Constante (autowiring) |
| **Erros** | Compilacao | Runtime (startup) |
| **Lifecycle** | Manual (graceful shutdown) | Automatico (OnStart/OnStop) |
| **Testabilidade** | Instanciar direto | `fxtest.New()` para testes de integracao |
| **Curva de aprendizado** | Zero | Medio (concepts, annotations) |

---

## Referencias

- [Documentacao oficial](https://uber-go.github.io/fx/)
- [API Reference](https://pkg.go.dev/go.uber.org/fx)
- [GitHub](https://github.com/uber-go/fx)
- [Blog post: Dependency Injection in Go](https://blog.uber.com/go-dependency-injection/)
