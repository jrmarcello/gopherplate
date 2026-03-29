# Diretrizes para Agentes de IA

Este documento define regras e boas práticas para agentes de IA que trabalham neste projeto. **Leia este arquivo antes de fazer qualquer alteração no código.**

---

## Princípios Arquiteturais

Este projeto segue **Clean Architecture** e princípios **SOLID**. Consulte os ADRs para detalhes:

| Princípio | Descrição | Referência |
| --------- | --------- | ---------- |
| **Clean Architecture** | Separação em camadas com dependências apontando para dentro | `docs/adr/001-clean-architecture.md` |
| **Dependency Inversion** | Use Cases definem interfaces; Infrastructure implementa | `docs/adr/001-clean-architecture.md` |
| **Single Responsibility** | Cada arquivo/struct tem uma única responsabilidade | - |
| **Error Handling** | Erros de domínio são puros; tradução ocorre no handler | `docs/adr/004-error-handling.md` |

### Estrutura de Camadas

```text
internal/
├── domain/           # Entidades e VOs (SEM dependências externas)
├── usecases/         # Casos de uso + interfaces (depende só do domain)
└── infrastructure/   # Implementações concretas (DB, HTTP, Cache)

pkg/                  # Pacotes reutilizáveis entre serviços
```

**Regra de Ouro**: Código em camadas internas **NUNCA** importa de camadas externas.

---

## FAZER

### Código

- Usar **Value Objects** para validação (`vo.ID`, `vo.Email`)
- Retornar **erros de domínio** específicos (`user.ErrNotFound`)
- Definir **interfaces** na camada de Use Cases (`interfaces/`)
- Injetar dependências via **construtor** (DI manual)
- Nomear variáveis de erro de forma única (evitar shadowing)
- Usar `pkg/httputil` para respostas HTTP padronizadas
- Usar `pkg/apperror` para erros estruturados
- Rodar `make lint` antes de qualquer commit

### Testes

- Escrever testes unitários para domain e usecases
- Usar **mocks manuais** em `mock_test.go` (sem frameworks)
- Testes table-driven com nomes descritivos
- Rodar `make test` antes de finalizar

### Commits

- Usar formato: `type(scope): description`
- Tipos: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`
- Staging específico: `git add <arquivo>` (nunca `git add -A`)

---

## NÃO FAZER

### Código

- **Nunca usar `--no-verify`** em commits
- **Nunca** colocar lógica de negócio em handlers HTTP
- **Nunca** importar `infrastructure` de dentro de `domain` ou `usecases`
- **Nunca** usar `panic()` para erros de validação
- **Nunca** deixar código comentado (delete ou crie issue)
- **Nunca** ignorar erros de lint
- **Nunca** usar `c.JSON()` direto — usar `httputil.SendSuccess`/`httputil.SendError`

### Arquitetura

- **Nunca** acessar banco de dados diretamente dos use cases (use Repository interface)
- **Nunca** retornar HTTP status codes do domínio
- **Nunca** criar dependências cíclicas entre pacotes
- **Nunca** usar pacotes internos para código que deveria ser reutilizável (usar `pkg/`)

---

## Cenários de Dúvida

> **Regra fundamental**: Na dúvida, **PERGUNTE ao usuário** antes de prosseguir.

### Quando perguntar

- Mudanças de arquitetura: "Isso afeta a estrutura do projeto? Devo criar um ADR?"
- Múltiplas abordagens válidas: "Posso usar X ou Y. Qual você prefere?"
- Escopo indefinido: "Você quer que eu também faça Z ou só X?"
- Breaking changes: "Isso vai quebrar a API. Devo prosseguir?"
- Convenções não documentadas: "Não encontrei uma convenção para isso. Como devo proceder?"

### O que NÃO assumir

- **Nunca** assumir que o usuário quer uma solução complexa quando uma simples resolve
- **Nunca** adicionar dependências sem perguntar
- **Nunca** mudar padrões estabelecidos sem discutir primeiro
- **Nunca** ignorar inconsistências no código — pergunte como resolver

---

## Padrões de Código

### Erros

```go
// Correto — erro de domínio puro
var ErrNotFound = errors.New("user not found")

// Errado — acoplado a HTTP
var ErrNotFound = NewHTTPError(404, "not found")
```

### Variáveis de Erro (Evitar Shadowing)

```go
// Correto
if parseErr := Parse(input); parseErr != nil { return parseErr }
if saveErr := repo.Save(ctx, e); saveErr != nil { return saveErr }

// Errado — shadow
if err := Parse(input); err != nil { return err }
if err := repo.Save(ctx, e); err != nil { return err }
```

### Injeção de Dependência

```go
// Correto — recebe interface, dependências opcionais via builder
func NewGetUseCase(repo interfaces.Repository) *GetUseCase {
    return &GetUseCase{Repo: repo}
}
func (uc *GetUseCase) WithCache(cache interfaces.Cache) *GetUseCase {
    uc.Cache = cache
    return uc
}
// Uso: NewGetUseCase(repo).WithCache(cache)

// Errado — instancia dependência internamente
func NewCreateUseCase() *CreateUseCase {
    return &CreateUseCase{repo: postgres.NewRepository()}
}
```

### Respostas da API

Todas as respostas HTTP **devem** usar os helpers de `pkg/httputil`:

```go
// Correto — resposta padronizada
httputil.SendSuccess(c, http.StatusOK, data)
httputil.SendSuccessWithMeta(c, http.StatusOK, data, meta, links)
httputil.SendError(c, http.StatusBadRequest, "invalid")

// Errado — c.JSON direto
c.JSON(http.StatusOK, data)
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid"})
```

### Pacotes Reutilizáveis (pkg/)

O diretório `pkg/` contém pacotes **reutilizáveis entre serviços**:

| Pacote | Uso |
| ------ | --- |
| `pkg/apperror` | Erros estruturados com código, mensagem e HTTP status |
| `pkg/httputil` | Helpers de resposta HTTP padronizada |
| `pkg/ctxkeys` | Chaves tipadas para context.Value |
| `pkg/logutil` | Logging estruturado com propagação de contexto |
| `pkg/telemetry` | Setup OpenTelemetry + HTTP metrics + DB pool metrics |
| `pkg/cache` | Interface de cache + implementação Redis |
| `pkg/database` | Conexão PostgreSQL com Writer/Reader cluster |
| `pkg/idempotency` | Interface de Store para idempotência + implementação Redis |

```go
// Correto — usar pkg/ para código reutilizável
import "bitbucket.org/appmax-space/go-boilerplate/pkg/apperror"

// Errado — usar internal para código que deveria ser reutilizável
import "bitbucket.org/appmax-space/go-boilerplate/internal/something"
```

---

## Configuração

| Ambiente | Fonte | Arquivo |
| -------- | ----- | ------- |
| Local (Go) | godotenv + `os` | `.env` (opcional) |
| Local (Docker) | Docker Compose | `.env` |
| Kubernetes | ConfigMap | `deploy/overlays/*/configmap.yaml` |

Ver: `docs/adr/003-config-strategy.md`

---

## Comandos Úteis

```bash
make lint          # golangci-lint + gofmt
make vulncheck     # govulncheck
make test          # Rodar todos os testes
make test-unit     # Apenas testes unitários
make dev           # Hot reload local
make docker-up     # Subir infraestrutura
make kind-setup    # Setup completo Kind (cluster + db + migrate + deploy)
make help          # Ver todos os comandos
```

---

## Documentação de Referência

### ADRs (Decisões Arquiteturais)

| Arquivo | Sobre |
| ------- | ----- |
| `docs/adr/001-clean-architecture.md` | Estrutura de camadas e DI |
| `docs/adr/002-ids.md` | Estrategia de IDs (UUID v7) |
| `docs/adr/003-config-strategy.md` | godotenv + .env + K8s |
| `docs/adr/004-error-handling.md` | Tratamento de erros em camadas |
| `docs/adr/005-service-key-auth.md` | Autenticação via Service Key |
| `docs/adr/006-migration-strategy.md` | ArgoCD PreSync + binário separado |
| `docs/adr/007-pkg-reusable-packages.md` | Pacotes reutilizáveis em pkg/ |
| `docs/adr/008-api-response-format.md` | Formato padronizado de resposta HTTP |

### Guias

| Arquivo | Sobre |
| ------- | ----- |
| `docs/guides/architecture.md` | Diagramas e visão geral |
| `docs/guides/cache.md` | Cache com Redis, singleflight e pool config |
| `docs/guides/kubernetes.md` | Deploy, Kind e operação |
| `docs/guides/fx-dependency-injection.md` | Uber Fx como alternativa ao DI manual |
| `docs/guides/multi-database.md` | Estrategia para múltiplos bancos |

---

## Checklist Antes de Submeter

- `make lint` passa sem erros
- `make test` passa
- Código segue estrutura de camadas
- Não há imports proibidos (infra -> domain)
- Commit message segue convenção

---

## Claude Code — Skills e Agentes

### Skills disponíveis (`.claude/skills/`)

| Skill | Propósito | Quando usar |
| ----- | --------- | ----------- |
| `/validate` | Pipeline completa (build, lint, tests, Kind, smoke) | Antes de commitar |
| `/validate quick` | Validação estática + testes unitários | Feedback rápido |
| `/new-endpoint` | Scaffold de endpoint Clean Architecture | Novo endpoint |
| `/fix-issue` | Workflow completo de fix (entender -> corrigir -> testar) | Corrigir bugs |
| `/migrate` | Gerenciar migrações Goose (create/up/down/status) | Schema do banco |
| `/review` | Code review single-agent | Revisão rápida |
| `/full-review-team` | Review paralelo: arquitetura + segurança + DB | PRs, mudanças grandes |
| `/security-review-team` | Auditoria de segurança paralela | Releases, compliance |
| `/debug-logs` | Análise de logs Kind/Docker | Debug via logs |
| `/debug-team` | Investigação paralela com hipóteses concorrentes | Bugs complexos |
| `/load-test` | Testes de carga k6 | Validação de performance |

### Agentes especializados (`.claude/agents/`)

| Agente | Foco | Modelo |
| ------ | ---- | ------ |
| `code-reviewer` | Arquitetura, Go idioms, convenções | sonnet |
| `security-reviewer` | Vulnerabilidades OWASP, injection, auth | opus |
| `db-analyst` | Schema, queries, migrações, performance | sonnet |

### Hooks de qualidade (`.claude/hooks/`)

| Hook | Trigger | Função |
| ---- | ------- | ------ |
| `guard-bash.sh` | PreToolUse[Bash] | Bloqueia comandos perigosos |
| `lint-go-file.sh` | PostToolUse[Edit/Write] | goimports + gopls em cada edit |
| `validate-migration.sh` | PostToolUse[Edit/Write] | Valida Up + Down em migrações |
| `stop-validate.sh` | Stop | Gate de qualidade antes de finalizar |
| `worktree-create.sh` | WorktreeCreate | Setup automático de worktree |
| `worktree-remove.sh` | WorktreeRemove | Cleanup de worktree |

### Rules automáticas (`.claude/rules/`)

| Arquivo | Aplica-se a | Conteúdo |
| ------- | ----------- | -------- |
| `go-conventions.md` | `**/*.go` | Error handling, DI, testing, pkg/ |
| `migrations.md` | `**/migration/**` | Goose Up+Down, reversibilidade |
| `security.md` | `**/*` | Credenciais, PII, SQL injection |
