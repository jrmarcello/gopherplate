# Diretrizes para Agentes de IA

Este documento contém informações e regras para agentes de IA que trabalham neste projeto.

## Arquitetura

Este projeto segue **Clean Architecture** com as seguintes camadas:

```text
cmd/api/          → Entrypoint (NÃO colocar lógica de negócio aqui)
config/           → Configuração via env vars
internal/
  domain/         → Entidades e Value Objects (regras de negócio puras)
  usecases/       → Casos de uso (orquestração)
  infrastructure/ → Implementações (DB, cache, HTTP)
  pkg/            → Utilitários compartilhados
```

## Regras de Código

### ✅ FAZER

- Usar Value Objects para validação (`vo.CPF`, `vo.Email`, `vo.Phone`)
- Retornar erros do domínio, não erros genéricos
- Manter Use Cases simples (orquestração apenas)
- Usar interfaces para dependências (`interfaces.Repository`, `interfaces.Cache`)
- Nomear variáveis de erro de forma única para evitar shadowing
- Rodar `make lint` antes de commitar

### ❌ NÃO FAZER

- **Nunca usar `--no-verify` em commits** - os hooks existem por um motivo
- Não colocar lógica de negócio em handlers HTTP
- Não acessar banco de dados diretamente dos use cases
- Não usar `panic()` para erros de validação
- Não ignorar erros de lint
- Não deixar código comentado (remove ou cria issue)

## Padrões

### Nomes de Variáveis de Erro

Para evitar shadowing, use nomes específicos:

```go
// ❌ Ruim - causa shadow
if err := SomeFunc(); err != nil { }
if err := OtherFunc(); err != nil { } // shadow!

// ✅ Bom - nomes únicos
if parseErr := SomeFunc(); parseErr != nil { }
if saveErr := OtherFunc(); saveErr != nil { }
```

### Configuração

- **Docker local**: Usar variáveis `POSTGRES_*` no `.env`
- **Kubernetes**: Usar `configmap.yaml` no overlay correspondente
- `DB_DSN` é construída automaticamente se não definida

### Cache

- Cache é **opcional** (nil-safe)
- Sempre verificar `if cache != nil` antes de usar
- Invalidar cache em operações de escrita (Create, Update, Delete)

## Commits

Formato: `type(scope): description`

```text
feat: add new feature
fix: bug fix
refactor: code refactoring
docs: documentation
test: tests
chore: maintenance
```

## Comandos Úteis

```bash
make lint          # Verificar código
make test          # Rodar testes
make kind-deploy   # Deploy local
make help          # Ver todos os comandos
```

## Estrutura de Testes

- `*_test.go` junto ao código → testes unitários
- `tests/e2e/` → testes de integração com TestContainers
- `tests/load/` → testes de carga com k6

## Documentação

### ADRs (Architecture Decision Records)

Decisões arquiteturais documentadas em `docs/adr/`:

| Arquivo | Descrição |
| ------- | --------- |
| `clean-architecture.md` | Pilares da Clean Architecture e estrutura de camadas |
| `config-strategy.md` | Estratégia de configuração com Viper + .env |
| `error-handling.md` | Sistema de tratamento de erros em camadas |
| `ulid.md` | Por que usamos ULID ao invés de UUID |

### Guias

Documentação explicativa em `docs/guides/`:

| Arquivo | Descrição |
| ------- | --------- |
| `architecture.md` | Visão geral da arquitetura com diagramas |
| `kubernetes.md` | Guia de deploy e operação no Kubernetes |
