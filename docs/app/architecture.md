# Arquitetura do Person Service

Documentação técnica da arquitetura do microsserviço de gestão de clientes, seguindo **Clean Architecture** e **DDD**.

---

## Sumário

- [Diagrama de Casos de Uso](#diagrama-de-casos-de-uso)
- [Diagrama de Componentes](#diagrama-de-componentes-clean-architecture)
- [Diagramas de Sequência](#diagramas-de-sequência)
  - [Criar Cliente](#1-criar-cliente)
  - [Buscar Cliente por ID](#2-buscar-cliente-por-id)
  - [Listar Clientes](#3-listar-clientes)
  - [Atualizar Cliente](#4-atualizar-cliente)
  - [Deletar Cliente](#5-deletar-cliente-soft-delete)
- [Fluxo de Dados](#fluxo-de-dados-entre-camadas)

---

## Diagrama de Casos de Uso

```mermaid
flowchart LR
    subgraph Atores
        Client["🖥️ API Client"]
        Admin["👤 Admin"]
    end

    subgraph Sistema["Person Service"]
        UC1["Criar Cliente"]
        UC2["Buscar Cliente"]
        UC3["Listar Clientes"]
        UC4["Atualizar Cliente"]
        UC5["Deletar Cliente"]
    end

    Client --> UC1
    Client --> UC2
    Client --> UC3
    Admin --> UC4
    Admin --> UC5

    UC1 -.->|valida| CPF["Validar CPF"]
    UC1 -.->|valida| Email["Validar Email"]
    UC1 -.->|gera| ID["Gerar ULID"]
```

### Descrição dos Casos de Uso

| Caso de Uso | Ator | Descrição |
|---|---|---|
| **Criar Cliente** | API Client | Cadastra novo cliente com validação de CPF/Email |
| **Buscar Cliente** | API Client | Retorna dados de um cliente por ID |
| **Listar Clientes** | API Client | Lista clientes com paginação e filtros |
| **Atualizar Cliente** | Admin | Atualiza dados de um cliente existente |
| **Deletar Cliente** | Admin | Realiza soft delete (active=false) |

---

## Diagrama de Componentes (Clean Architecture)

```mermaid
flowchart TB
    subgraph External["🌐 Camada Externa"]
        HTTP["HTTP Request"]
        DB[("PostgreSQL")]
        OTEL["OpenTelemetry Collector"]
    end

    subgraph Infrastructure["⚙️ Infrastructure Layer"]
        direction TB
        Handler["Handler\n(person.go)"]
        Middlewares["Middlewares\n(Logger, CORS, Idempotency)"]
        RepoImpl["PersonRepository\n(person_repo.go)"]
        Telemetry["Telemetry\n(telemetry.go)"]
    end

    subgraph Application["📦 Application Layer"]
        direction TB
        CreateUC["CreatePerson\nUseCase"]
        GetUC["GetPerson\nUseCase"]
        ListUC["ListPersons\nUseCase"]
        UpdateUC["UpdatePerson\nUseCase"]
        DeleteUC["DeletePerson\nUseCase"]
        DTOs["DTOs\n(Input/Output)"]
    end

    subgraph Domain["💎 Domain Layer"]
        direction TB
        Entity["Person\nEntity"]
        VOs["Value Objects\n(ID, CPF, Email)"]
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
    
    Handler --> Telemetry
    Telemetry --> OTEL

    style Domain fill:#e1f5fe
    style Application fill:#fff3e0
    style Infrastructure fill:#f3e5f5
    style External fill:#fce4ec
```

### Regra de Dependência

> 💡 **As dependências sempre apontam para DENTRO** (em direção ao Domain).

```
External → Infrastructure → Application → Domain
```

O **Domain** não conhece nenhuma outra camada. O **Application** conhece apenas o Domain. E assim por diante.

---

## Diagramas de Sequência

### 1. Criar Cliente

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant MW as Middlewares
    participant H as Handler
    participant UC as CreateUseCase
    participant VO as Value Objects
    participant E as Entity Factory
    participant R as Repository
    participant DB as PostgreSQL

    Client->>+MW: POST /people<br/>X-Idempotency-Key: abc123<br/>{name, document, email}
    
    Note over MW: Logger: gera X-Request-ID
    Note over MW: OTEL: inicia span
    Note over MW: Idempotency: verifica cache
    
    MW->>+H: Request + Context

    H->>H: Bind JSON → InputDTO
    H->>+UC: Execute(ctx, InputDTO)

    UC->>+VO: NewCPF(document)
    alt CPF inválido
        VO-->>UC: ErrInvalidCPF
        UC-->>H: error
        H-->>Client: 400 Bad Request
    end
    VO-->>-UC: CPF (validado)

    UC->>+VO: NewEmail(email)
    alt Email inválido
        VO-->>UC: ErrInvalidEmail
        UC-->>H: error
        H-->>Client: 400 Bad Request
    end
    VO-->>-UC: Email (validado)

    UC->>+E: NewPerson(name, cpf, email)
    Note over E: Gera ULID<br/>Define timestamps<br/>Active = true
    E-->>-UC: Person Entity

    UC->>+R: Create(ctx, person)
    R->>R: fromEntity() → DB Model
    R->>+DB: INSERT INTO people...
    alt CPF duplicado
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

### 2. Buscar Cliente por ID

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant H as Handler
    participant UC as GetUseCase
    participant VO as Value Objects
    participant R as Repository
    participant DB as PostgreSQL

    Client->>+H: GET /people/{id}

    H->>+UC: Execute(ctx, InputDTO{ID})

    UC->>+VO: ParseID(id)
    alt ID inválido (não é ULID)
        VO-->>UC: error
        UC-->>H: error
        H-->>Client: 400 Bad Request
    end
    VO-->>-UC: ID (validado)

    UC->>+R: FindByID(ctx, id)
    R->>+DB: SELECT * FROM people WHERE id = $1
    alt Não encontrado
        DB-->>R: sql.ErrNoRows
        R-->>UC: ErrPersonNotFound
        UC-->>H: error
        H-->>Client: 404 Not Found
    end
    DB-->>-R: personDB
    R->>R: toEntity() → Person
    R-->>-UC: Person Entity

    UC->>UC: Entity → OutputDTO
    UC-->>-H: OutputDTO

    H-->>-Client: 200 OK<br/>{id, name, document, email, ...}
```

---

### 3. Listar Clientes

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant H as Handler
    participant UC as ListUseCase
    participant R as Repository
    participant DB as PostgreSQL

    Client->>+H: GET /people?page=1&limit=10&name=João

    H->>H: Bind Query → InputDTO
    H->>+UC: Execute(ctx, InputDTO)

    UC->>UC: filter.Normalize()
    Note over UC: Define defaults<br/>page=1, limit=20

    UC->>+R: List(ctx, filter)
    
    R->>R: Build WHERE clause<br/>com filtros dinâmicos
    
    R->>+DB: SELECT COUNT(*) FROM people WHERE...
    DB-->>-R: total = 42
    
    R->>+DB: SELECT * FROM people<br/>WHERE... ORDER BY created_at DESC<br/>LIMIT 10 OFFSET 0
    DB-->>-R: []personDB
    
    R->>R: toEntity() para cada item
    R-->>-UC: ListResult{people, total, page, limit}

    UC->>UC: Entities → OutputDTO
    UC-->>-H: OutputDTO{people, pagination}

    H-->>-Client: 200 OK<br/>{people: [...], pagination: {...}}
```

---

### 4. Atualizar Cliente

```mermaid
sequenceDiagram
    autonumber
    actor Admin
    participant H as Handler
    participant UC as UpdateUseCase
    participant VO as Value Objects
    participant R as Repository
    participant DB as PostgreSQL

    Admin->>+H: PUT /people/{id}<br/>{name, email, address}

    H->>H: Bind JSON → InputDTO
    H->>H: req.ID = c.Param("id")
    H->>+UC: Execute(ctx, InputDTO)

    UC->>+R: FindByID(ctx, id)
    alt Não encontrado
        R-->>UC: ErrPersonNotFound
        UC-->>H: error
        H-->>Admin: 404 Not Found
    end
    R-->>-UC: Person Entity

    opt Email alterado
        UC->>+VO: NewEmail(newEmail)
        VO-->>-UC: Email (validado)
        UC->>UC: person.UpdateEmail(email)
    end

    opt Nome alterado
        UC->>UC: person.UpdateName(name)
    end

    opt Endereço alterado
        UC->>UC: person.SetAddress(address)
    end

    Note over UC: UpdatedAt = time.Now()

    UC->>+R: Update(ctx, person)
    R->>+DB: UPDATE people SET... WHERE id = $1
    DB-->>-R: rowsAffected = 1
    R-->>-UC: nil

    UC-->>-H: OutputDTO

    H-->>-Admin: 200 OK<br/>{id, updated_at}
```

---

### 5. Deletar Cliente (Soft Delete)

```mermaid
sequenceDiagram
    autonumber
    actor Admin
    participant H as Handler
    participant UC as DeleteUseCase
    participant R as Repository
    participant DB as PostgreSQL

    Admin->>+H: DELETE /people/{id}

    H->>+UC: Execute(ctx, InputDTO{ID})

    UC->>+R: Delete(ctx, id)
    
    R->>+DB: UPDATE people<br/>SET active = false, updated_at = now()<br/>WHERE id = $1 AND active = true
    
    alt Não encontrado ou já deletado
        DB-->>R: rowsAffected = 0
        R-->>UC: ErrPersonNotFound
        UC-->>H: error
        H-->>Admin: 404 Not Found
    end
    
    DB-->>-R: rowsAffected = 1
    R-->>-UC: nil

    UC-->>-H: OutputDTO{success: true}

    H-->>-Admin: 200 OK<br/>{deleted: true}
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
        Entity["Entity\n(regras de negócio)"]
    end

    subgraph Repository["Repository"]
        DBModel["DB Model\n(sql.NullString)"]
    end

    subgraph Output["📤 Saída"]
        Response["JSON Response"]
    end

    JSON -->|"ShouldBindJSON()"| InputDTO
    InputDTO -->|"vo.NewCPF()"| VOs
    VOs -->|"NewPerson()"| Entity
    Entity -->|"fromEntity()"| DBModel
    DBModel -->|"toEntity()"| Entity
    Entity -->|"→ OutputDTO"| Response

    style Input fill:#e8f5e9
    style Output fill:#e8f5e9
```

### Transformações de Dados

| Camada | Tipo de Dado | Exemplo |
|---|---|---|
| **HTTP** | JSON string | `{"document": "529.982.247-25"}` |
| **Handler** | InputDTO (primitivos) | `Document string` |
| **UseCase** | Value Object (validado) | `vo.CPF{value: "52998224725"}` |
| **Entity** | Agregado completo | `Person{CPF, Email, ID, ...}` |
| **Repository** | DB Model (nullable) | `personDB{Document: "52998224725"}` |
| **Database** | SQL | `document VARCHAR(11)` |

---

## Estrutura de Diretórios

```
internal/
├── domain/                    # 💎 Camada de Domínio
│   └── person/
│       ├── entity.go          # Entidade Person
│       ├── repository.go      # Interface Repository
│       ├── errors.go          # Erros de domínio
│       ├── filter.go          # Filtros de listagem
│       └── vo/                # Value Objects
│           ├── id.go          # ULID
│           ├── cpf.go         # CPF (MOD 11)
│           ├── email.go       # Email (RFC 5322)
│           └── address.go     # Endereço
│
├── usecase/                   # 📦 Camada de Aplicação
│   ├── create_person/
│   │   ├── dto.go             # Input/Output DTOs
│   │   └── usecase.go         # Lógica de orquestração
│   ├── get_person/
│   ├── list_people/
│   ├── update_person/
│   └── delete_person/
│
├── infrastructure/            # ⚙️ Camada de Infraestrutura
│   ├── db/
│   │   ├── postgres.go        # Conexão com banco
│   │   └── repository/
│   │       └── person_repo.go  # Implementação do Repository
│   ├── web/
│   │   ├── handler/
│   │   │   └── person.go    # HTTP Handlers
│   │   └── middleware/
│   │       ├── logger.go      # Logging estruturado
│   │       ├── idempotency.go # Idempotência
│   └── telemetry/
│       └── otel.go            # OpenTelemetry setup
│
├── server/
│   └── server.go              # Bootstrap e DI
│
└── pkg/
    └── apperror/              # Erros de aplicação
```

---

## Referências

- [Clean Architecture - Uncle Bob](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Domain-Driven Design - Eric Evans](https://domainlanguage.com/ddd/)
- [ULID Spec](https://github.com/ulid/spec)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
