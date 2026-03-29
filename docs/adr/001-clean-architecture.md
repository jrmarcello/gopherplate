# ADR-001: Clean Architecture

**Status**: Aceito  
**Data**: 2026-01-16  
**Autor**: Marcelo Jr

---

## Contexto

Aplicações complexas tendem a se tornar difíceis de manter, testar e evoluir quando as regras de negócio estão acopladas a detalhes de implementação (frameworks, banco de dados, UI). Buscamos uma arquitetura que garanta longevidade ao projeto e facilite a manutenção.

---

## Decisão

Adotamos a **Clean Architecture** proposta por Robert C. Martin, focando em seus **pilares fundamentais**:

1. **Regra da Dependência**: Dependências de código fonte apontam **apenas para dentro** (camadas internas nunca conhecem camadas externas).
2. **Entidades**: Objetos de domínio que encapsulam regras de negócio corporativas.
3. **Casos de Uso**: Orquestram o fluxo de dados e aplicam regras de negócio específicas da aplicação.
4. **Inversão de Dependência**: Camadas internas definem **interfaces**; camadas externas as implementam.

---

## Alternativas Consideradas

| Estratégia | Veredicto | Motivo |
| ---------- | --------- | ------ |
| MVC (Layered) | ❌ Rejeitado | Tende a acoplar lógica de negócio em Controllers ou Models "gordos" |
| Hexagonal (Ports & Adapters) | ⚠️ Alternativa | Muito similar à Clean, princípios compatíveis |
| **Clean Architecture** | ✅ **Escolhido** | Definição mais rigorosa de fronteiras e regra de dependência |

---

## Justificativa

1. **Independência de Frameworks**: Frameworks são ferramentas, não o centro da aplicação.
2. **Testabilidade**: Regras de negócio testáveis sem UI, DB ou Web Server.
3. **Independência de Banco de Dados**: O DB é um detalhe. Podemos trocar Postgres por Mongo ou In-Memory sem tocar nas regras de negócio.
4. **Independência de Interface**: A UI (Web, CLI, Mobile) pode mudar sem afetar o core.

---

## Consequências

### Positivas

- Padronização do projeto.
- Testes unitários triviais (mocks fáceis).
- Evolução flexível (ex: começar com repositório em memória).

### Negativas

- Setup inicial mais verboso (mais arquivos e camadas).
- Curva de aprendizado inicial para quem vem de MVC tradicional.

### Mitigações

- Uso de boilerplate/template para reduzir setup inicial.
- Documentação rica (este ADR e guias).

---

## Implementação

### Estrutura de Camadas

| Camada | Responsabilidade | Exemplo |
| ------ | ---------------- | ------- |
| **Domain** | Entidades e Value Objects puros | `User`, `ID`, `Email` |
| **Usecases** | Lógica de aplicação, DTOs, interfaces de repositório | `CreateUseCase`, `Repository` (interface) |
| **Infrastructure** | Implementações concretas (DB, Web, Cache) | `PostgresRepository`, `GinHandler` |

### Estrutura de Pastas

```text
internal/
├── domain/              # 🟢 Camada mais interna (sem dependências externas)
│   ├── user/
│   │   ├── user.go           # Entidade de domínio
│   │   ├── errors.go         # Erros de domínio
│   │   └── filter.go         # Filtros de busca
│   └── role/
│       ├── role.go
│       └── errors.go
│
├── usecases/            # 🟡 Orquestração (depende apenas do Domain)
│   ├── user/
│   │   ├── interfaces/
│   │   │   └── repository.go  # Interface do repositório (definida aqui!)
│   │   ├── create.go          # Caso de uso de criação
│   │   ├── get.go
│   │   └── dto/               # Data Transfer Objects
│   └── role/
│       ├── interfaces/
│       │   └── repository.go
│       └── create.go
│
└── infrastructure/      # 🔴 Camada externa (implementa interfaces)
    ├── db/postgres/
    │   └── repository/
    │       ├── user.go        # Implementação concreta do Repository
    │       └── role.go
    └── web/
        ├── handler/
        │   ├── user.go        # Handler HTTP (Gin)
        │   └── role.go
        └── router/
```

### Inversão de Dependência (DI)

O **Use Case** define a interface do repositório. A **Infrastructure** a implementa.

```go
// usecases/user/interfaces/repository.go (Camada Interna)
type Repository interface {
    Save(ctx context.Context, u *user.User) error
    FindByID(ctx context.Context, id vo.ID) (*user.User, error)
}
```

```go
// infrastructure/db/postgres/repository/user.go (Camada Externa)
type UserRepository struct {
    db *sql.DB
}

func (r *UserRepository) Save(ctx context.Context, u *user.User) error {
    // Implementação concreta usando database/sql
}
```

### Composição (Bootstrap)

No `main.go` ou `server.go`, injetamos as dependências concretas:

```go
// cmd/api/server.go
func Run(cfg *config.Config) {
    writerCfg := database.DefaultConfig("postgres", cfg.DB.GetWriterDSN())
    cluster, _ := database.NewDBCluster(writerCfg, nil)

    // Injeção de Dependência
    repo := repository.NewUserRepository(cluster.Writer())   // Implementação concreta
    createUC := useruc.NewCreateUseCase(repo)                // Use Case recebe a interface
    handler := handler.NewUserHandler(createUC)

    router.Setup(handler)
}
```

O Use Case (`createUC`) **não sabe** que está falando com PostgreSQL. Ele só conhece a interface `Repository`. Isso permite trocar a implementação (ex: para MongoDB ou um mock em testes) sem alterar o caso de uso.
