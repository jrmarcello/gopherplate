# Arquitetura do User Service

Documentação técnica da arquitetura do microsserviço de gestão de usuários, seguindo **Clean Architecture** e **DDD**.

---

## Sumário

- [Diagrama de Casos de Uso](#diagrama-de-casos-de-uso)
- [Diagrama de Componentes](#diagrama-de-componentes-clean-architecture)
- [Diagramas de Sequência](#diagramas-de-sequência)
  - [Criar User](#1-criar-user)
  - [Buscar User por ID](#2-buscar-user-por-id)
  - [Listar Users](#3-listar-users)
  - [Atualizar User](#4-atualizar-user)
  - [Deletar User](#5-deletar-user-soft-delete)
- [Fluxo de Dados](#fluxo-de-dados-entre-camadas)

---

## Diagrama de Casos de Uso

```mermaid
flowchart LR
    subgraph Atores
        Client["🖥️ API Client"]
        Admin["👤 Admin"]
    end

    subgraph Sistema["User Service"]
        UC1["Criar User"]
        UC2["Buscar User"]
        UC3["Listar Users"]
        UC4["Atualizar User"]
        UC5["Deletar User"]
    end

    Client --> UC1
    Client --> UC2
    Client --> UC3
    Admin --> UC4
    Admin --> UC5

    UC1 -.->|valida| Email["Validar Email"]
    UC1 -.->|gera| ID["Gerar UUID v7"]
```

### Descrição dos Casos de Uso

| Caso de Uso | Ator | Descrição |
| --- | --- | --- |
| **Criar User** | API Client | Cadastra novo usuário com validação de email e geração de UUID v7 |
| **Buscar User** | API Client | Retorna dados de um usuário por ID (com cache) |
| **Listar Users** | API Client | Lista usuários com paginação e filtros (nome, email, active) |
| **Atualizar User** | Admin | Atualiza dados (nome, email) de um usuário existente |
| **Deletar User** | Admin | Realiza soft delete (active=false) |

---

## Diagrama de Componentes (Clean Architecture)

```mermaid
flowchart TB
    subgraph External["🌐 Camada Externa"]
        HTTP["HTTP Request"]
        DB[("PostgreSQL")]
        Redis[("Redis Cache")]
        OTEL["OpenTelemetry Collector"]
    end

    subgraph Infrastructure["⚙️ Infrastructure Layer"]
        direction TB
        Handler["UserHandler\n(handler/user.go)"]
        Middlewares["Middlewares\n(Logger, CORS, Idempotency)"]
        RepoImpl["UserRepository\n(repository/user.go)"]
        CacheImpl["RedisCache\n(pkg/cache/redis.go)"]
        Telemetry["Telemetry\n(otel.go)"]
    end

    subgraph Application["📦 Application Layer"]
        direction TB
        CreateUC["CreateUseCase"]
        GetUC["GetUseCase"]
        ListUC["ListUseCase"]
        UpdateUC["UpdateUseCase"]
        DeleteUC["DeleteUseCase"]
        DTOs["DTOs\n(Input/Output)"]
    end

    subgraph Domain["💎 Domain Layer"]
        direction TB
        Entity["User\nAggregate"]
        VOs["Value Objects\n(ID, Email)"]
        RepoInterface["Repository\nInterface"]
        Errors["Domain Errors"]
    end

    HTTP --> Middlewares
    Middlewares --> Handler
    Handler --> DTOs
    DTOs --> CreateUC & GetUC & ListUC & UpdateUC & DeleteUC
    
    CreateUC & GetUC & ListUC & UpdateUC & DeleteUC --> VOs
    CreateUC & GetUC & ListUC & UpdateUC & DeleteUC --> Entity
    CreateUC & GetUC & ListUC & UpdateUC & DeleteUC --> RepoInterface
    
    RepoInterface -.->|implementa| RepoImpl
    RepoImpl --> DB
    RepoImpl --> CacheImpl
    
    CacheImpl --> Redis
    
    Handler --> Telemetry
    Telemetry --> OTEL

    style Domain fill:#e1f5fe
    style Application fill:#fff3e0
    style Infrastructure fill:#f3e5f5
    style External fill:#fce4ec
```

### Regra de Dependência

> 💡 **As dependências sempre apontam para DENTRO** (em direção ao Domain).

```text
External → Infrastructure → Application → Domain
```

O **Domain** não conhece nenhuma outra camada. O **Application** conhece apenas o Domain. E assim por diante.

---

## Diagramas de Sequência

### 1. Criar User

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant MW as Middlewares
    participant H as Handler
    participant UC as CreateUseCase
    participant VO as Value Objects
    participant E as User Factory
    participant R as Repository
    participant DB as PostgreSQL

    Client->>+MW: POST /users<br/>X-Idempotency-Key: abc123<br/>{name, email}
    
    Note over MW: Logger: gera X-Request-ID
    Note over MW: OTEL: inicia span
    Note over MW: Idempotency: verifica cache
    
    MW->>+H: Request + Context

    H->>H: Bind JSON → InputDTO
    H->>+UC: Execute(ctx, InputDTO)

    UC->>+VO: NewEmail(email)
    alt Email inválido
        VO-->>UC: ErrInvalidEmail
        UC-->>H: error
        H-->>Client: 400 Bad Request
    end
    VO-->>-UC: Email (validado)

    UC->>+E: NewUser(name, email)
    Note over E: Gera UUID v7<br/>Define timestamps<br/>Active = true
    E-->>-UC: User

    UC->>+R: Create(ctx, user)
    R->>R: fromUser() → DB Model
    R->>+DB: INSERT INTO users...
    alt Email duplicado
        DB-->>R: unique_violation
        R-->>UC: error
        UC-->>H: error
        H-->>Client: 409 Conflict
    end
    DB-->>-R: OK
    R-->>-UC: nil

    UC-->>-H: OutputDTO{id, created_at}
    H-->>-MW: JSON Response

    Note over MW: Idempotency: cacheia resposta
    Note over MW: Logger: log completed

    MW-->>-Client: 201 Created<br/>{id, created_at}
```

---

### 2. Buscar User por ID

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant H as Handler
    participant UC as GetUseCase
    participant C as Redis Cache
    participant SF as SingleFlight
    participant R as Repository
    participant DB as PostgreSQL

    Client->>+H: GET /users/{id}

    H->>+UC: Execute(ctx, InputDTO{ID})

    UC->>+C: Get(ctx, "user:{id}")
    alt Cache Hit
        C-->>UC: User JSON
        UC-->>H: OutputDTO (from cache)
        H-->>Client: 200 OK
    else Cache Miss
        C-->>-UC: nil

        UC->>+SF: Do(id, fetchFn)
        Note over SF: Deduplicação de requests<br/>concorrentes para mesmo ID<br/>(previne cache stampede)
        SF->>+R: FindByID(ctx, id)
        R->>+DB: SELECT * FROM users WHERE id = $1
        alt Não encontrado
            DB-->>R: sql.ErrNoRows
            R-->>UC: ErrUserNotFound
            UC-->>H: error
            H-->>Client: 404 Not Found
        end
        DB-->>-R: userDB
        R-->>-SF: User
        SF-->>-UC: User

        UC->>C: Set(ctx, "user:{id}", User JSON)

        UC-->>-H: OutputDTO
        H-->>-Client: 200 OK
    end
```

---

### 3. Listar Users

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant H as Handler
    participant UC as ListUseCase
    participant R as Repository
    participant DB as PostgreSQL

    Client->>+H: GET /users?page=1&limit=10&name=Test
    
    H->>H: Bind Query → InputDTO
    H->>+UC: Execute(ctx, InputDTO)

    UC->>UC: filter.Normalize()
    Note over UC: Define defaults<br/>page=1, limit=20

    UC->>+R: List(ctx, filter)
    
    R->>R: Build WHERE clause<br/>com filtros dinâmicos
    
    R->>+DB: SELECT COUNT(*) FROM users WHERE...
    DB-->>-R: total = 42
    
    R->>+DB: SELECT * FROM users<br/>WHERE... ORDER BY created_at DESC<br/>LIMIT 10 OFFSET 0
    DB-->>-R: []userDB

    R->>R: toUser() para cada item
    R-->>-UC: ListResult{users, total, page, limit}

    UC->>UC: Users → OutputDTO
    UC-->>-H: OutputDTO{data, pagination}

    H-->>-Client: 200 OK
```

---

### 4. Atualizar User

```mermaid
sequenceDiagram
    autonumber
    actor Admin
    participant H as Handler
    participant UC as UpdateUseCase
    participant VO as Value Objects
    participant R as Repository
    participant C as Redis Cache
    participant DB as PostgreSQL

    Admin->>+H: PUT /users/{id}<br/>{name, email}

    H->>H: Bind JSON → InputDTO
    H->>+UC: Execute(ctx, InputDTO)

    UC->>+R: FindByID(ctx, id)
    alt Não encontrado
        R-->>UC: ErrUserNotFound
        UC-->>H: error
        H-->>Admin: 404 Not Found
    end
    R-->>-UC: User

    opt Email alterado
        UC->>+VO: NewEmail(newEmail)
        VO-->>-UC: Email (validado)
        UC->>UC: user.UpdateEmail(email)
    end

    opt Nome alterado
        UC->>UC: user.UpdateName(name)
    end

    Note over UC: UpdatedAt = time.Now()

    UC->>+R: Update(ctx, user)
    R->>+DB: UPDATE users SET... WHERE id = $1
    DB-->>-R: rowsAffected = 1
    R-->>-UC: nil
    
    UC->>C: Delete(ctx, "user:{id}")
    Note over C: Invalida cache

    UC-->>-H: OutputDTO

    H-->>-Admin: 200 OK<br/>{id, updated_at}
```

---

### 5. Deletar User (Soft Delete)

```mermaid
sequenceDiagram
    autonumber
    actor Admin
    participant H as Handler
    participant UC as DeleteUseCase
    participant R as Repository
    participant C as Redis Cache
    participant DB as PostgreSQL

    Admin->>+H: DELETE /users/{id}

    H->>+UC: Execute(ctx, InputDTO{ID})

    UC->>+R: Delete(ctx, id)
    
    R->>+DB: UPDATE users<br/>SET active = false, updated_at = now()<br/>WHERE id = $1 AND active = true
    
    alt Não encontrado ou já deletado
        DB-->>R: rowsAffected = 0
        R-->>UC: ErrUserNotFound
        UC-->>H: error
        H-->>Admin: 404 Not Found
    end
    
    DB-->>-R: rowsAffected = 1
    R-->>-UC: nil
    
    UC->>C: Delete(ctx, "user:{id}")
    Note over C: Invalida cache

    UC-->>-H: OutputDTO{success: true}

    H-->>-Admin: 200 OK
```

---

## Fluxo de Dados Entre Camadas

```mermaid
flowchart LR
    subgraph Input["📥 Entrada"]
        JSON["JSON Request"]
    end

    subgraph Handler["Handler"]
        InputDTO["InputDTO\n(tipos primitivos)"]
    end

    subgraph UseCase["UseCase"]
        VOs["Value Objects\n(validados)"]
        Entity["User\n(regras de negócio)"]
    end

    subgraph Repository["Repository"]
        DBModel["DB Model\n(sql.NullString)"]
    end

    subgraph Output["📤 Saída"]
        Response["JSON Response"]
    end

    JSON -->|"ShouldBindJSON()"| InputDTO
    InputDTO -->|"vo.NewEmail()"| VOs
    VOs -->|"NewUser()"| Entity
    Entity -->|"fromUser()"| DBModel
    DBModel -->|"toUser()"| Entity
    Entity -->|"→ OutputDTO"| Response

    style Input fill:#e8f5e9
    style Output fill:#e8f5e9
```

### Transformações de Dados

| Camada | Tipo de Dado | Exemplo |
| --- | --- | --- |
| **HTTP** | JSON string | `{"name": "Alice", "email": "alice@example.com"}` |
| **Handler** | InputDTO (primitivos) | `dto.CreateInput{Name: "Alice"}` |
| **UseCase** | Value Object (validado) | `vo.Email{value: "alice@example.com"}` |
| **Entity** | Agregado completo | `User{ID, Name, Email, Active...}` |
| **Repository** | DB Model (nullable) | `userDB{Name: "Alice", Email: "..."}` |
| **Database** | SQL | `name VARCHAR(255)` |

---

## Estrutura de Diretórios

```text
internal/
├── domain/                    # 💎 Camada de Domínio
│   └── user/
│       ├── user.go            # Aggregate User
│       ├── errors.go          # Erros de domínio
│       ├── filter.go          # Filtros de listagem
│       └── vo/                # Value Objects
│           ├── id.go          # UUID v7 (RFC 9562)
│           ├── email.go       # Email (RFC 5322)
│           └── errors.go      # Erros de VO
│
├── usecases/                  # 📦 Camada de Aplicação
│   └── user/
│       ├── create.go          # Use Case de Criação
│       ├── get.go             # Use Case de Leitura
│       ├── list.go            # Use Case de Listagem
│       ├── update.go          # Use Case de Atualização
│       ├── delete.go          # Use Case de Remoção
│       ├── dto/               # Input/Output DTOs
│       └── interfaces/        # Interfaces (Repository, Cache)
│
├── infrastructure/            # ⚙️ Camada de Infraestrutura
│   ├── cache/                 # Legacy (ver pkg/cache/ para novo código)
│   │   └── redis.go
│   ├── db/
│   │   ├── postgres/          # Implementação Postgres
│   │   │   ├── repository/
│   │   │   └── migration/
│   ├── web/
│   │   ├── handler/
│   │   │   └── user.go         # HTTP Handlers
│   │   ├── middleware/        # Middlewares (Logger, Auth, etc)
│   │   └── router/            # Rotas Gin
│   └── telemetry/
│       └── otel.go            # OpenTelemetry setup
│
├── cmd/api/                   # Entrypoint
└── config/                    # Configurações
```

---

## Referências

- [Clean Architecture - Uncle Bob](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Domain-Driven Design - Eric Evans](https://domainlanguage.com/ddd/)
- [RFC 9562 (UUID v7)](https://www.rfc-editor.org/rfc/rfc9562)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
