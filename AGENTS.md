# Diretrizes para Agentes de IA

Este documento define regras e boas práticas para agentes de IA que trabalham neste projeto. **Leia este arquivo antes de fazer qualquer alteração no código.**

---

## Princípios Arquiteturais

Este projeto segue **Clean Architecture** e princípios **SOLID**. Consulte os ADRs para detalhes:

| Princípio | Descrição | Referência |
| --------- | --------- | ---------- |
| **Clean Architecture** | Separação em camadas com dependências apontando para dentro | `docs/adr/clean-architecture.md` |
| **Dependency Inversion** | Use Cases definem interfaces; Infrastructure implementa | `docs/adr/clean-architecture.md` |
| **Single Responsibility** | Cada arquivo/struct tem uma única responsabilidade | - |
| **Error Handling** | Erros de domínio são puros; tradução ocorre no handler | `docs/adr/error-handling.md` |

### Estrutura de Camadas

```text
internal/
├── domain/           # 🟢 Entidades e VOs (SEM dependências externas)
├── usecases/         # 🟡 Casos de uso + interfaces (depende só do domain)
└── infrastructure/   # 🔴 Implementações concretas (DB, HTTP, Cache)
```

**Regra de Ouro**: Código em camadas internas **NUNCA** importa de camadas externas.

---

## ✅ FAZER

### Código

- [ ] Usar **Value Objects** para validação (`vo.ID`, `vo.Email`)
- [ ] Retornar **erros de domínio** específicos (`entity.ErrNotFound`)
- [ ] Definir **interfaces** na camada de Use Cases
- [ ] Injetar dependências via **construtor** (DI)
- [ ] Nomear variáveis de erro de forma única (evitar shadowing)
- [ ] Rodar `make lint` antes de qualquer commit

### Testes

- [ ] Escrever testes unitários para domain e usecases
- [ ] Usar **mocks** para dependências em testes de use case
- [ ] Rodar `make test` antes de finalizar

### Commits

- [ ] Usar formato: `type(scope): description`
- [ ] Tipos: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`

---

## ❌ NÃO FAZER

### Código

- [ ] **Nunca usar `--no-verify`** em commits
- [ ] **Nunca** colocar lógica de negócio em handlers HTTP
- [ ] **Nunca** importar `infrastructure` de dentro de `domain` ou `usecases`
- [ ] **Nunca** usar `panic()` para erros de validação
- [ ] **Nunca** deixar código comentado (delete ou crie issue)
- [ ] **Nunca** ignorar erros de lint

### Arquitetura

- [ ] **Nunca** acessar banco de dados diretamente dos use cases (use Repository interface)
- [ ] **Nunca** retornar HTTP status codes do domínio
- [ ] **Nunca** criar dependências cíclicas entre pacotes

---

## Padrões de Código

### Erros

```go
// ✅ Correto - erro de domínio puro
var ErrNotFound = errors.New("entity not found")

// ❌ Errado - acoplado a HTTP
var ErrNotFound = NewHTTPError(404, "not found")
```

### Variáveis de Erro (Evitar Shadowing)

```go
// ✅ Correto
if parseErr := Parse(input); parseErr != nil { return parseErr }
if saveErr := repo.Save(ctx, e); saveErr != nil { return saveErr }

// ❌ Errado - shadow
if err := Parse(input); err != nil { return err }
if err := repo.Save(ctx, e); err != nil { return err }
```

### Injeção de Dependência

```go
// ✅ Correto - recebe interface
func NewCreateUseCase(repo interfaces.Repository) *CreateUseCase {
    return &CreateUseCase{repo: repo}
}

// ❌ Errado - instancia dependência internamente
func NewCreateUseCase() *CreateUseCase {
    return &CreateUseCase{repo: postgres.NewRepository()}
}
```

---

## Configuração

| Ambiente | Fonte | Arquivo |
| -------- | ----- | ------- |
| Local (Go) | godotenv + `os` | `.env` (opcional) |
| Local (Docker) | Docker Compose | `.env` |
| Kubernetes | ConfigMap | `deploy/overlays/*/configmap.yaml` |

Ver: `docs/adr/config-strategy.md`

---

## Comandos Úteis

```bash
make lint          # Verificar código
make test          # Rodar todos os testes
make dev           # Hot reload local
make docker-up     # Subir infraestrutura
make help          # Ver todos os comandos
```

---

## Documentação de Referência

### ADRs (Decisões Arquiteturais)

| Arquivo | Sobre |
| ------- | ----- |
| `docs/adr/clean-architecture.md` | Estrutura de camadas e DI |
| `docs/adr/config-strategy.md` | godotenv + .env + K8s |
| `docs/adr/error-handling.md` | Tratamento de erros em camadas |
| `docs/adr/ulid.md` | Por que ULID ao invés de UUID |

### Guias

| Arquivo | Sobre |
| ------- | ----- |
| `docs/guides/architecture.md` | Diagramas e visão geral |
| `docs/guides/kubernetes.md` | Deploy e operação |

---

## Checklist Antes de Submeter

- [ ] `make lint` passa sem erros
- [ ] `make test` passa
- [ ] Código segue estrutura de camadas
- [ ] Não há imports proibidos (infra → domain)
- [ ] Commit message segue convenção
