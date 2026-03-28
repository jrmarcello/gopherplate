# Go Boilerplate — Template para Microsserviços

> De zero a produção em minutos, não semanas.

---

## O problema

Toda vez que um time cria um novo microsserviço em Go, ele enfrenta as mesmas perguntas:

- Como organizar o código?
- Como configurar DB, cache, observabilidade?
- Como garantir qualidade (lint, testes, CI)?
- Como fazer deploy no Kubernetes?
- Quanto tempo até o primeiro endpoint funcionar?

A resposta geralmente é: **copiar de outro serviço e adaptar**. Isso gera inconsistência entre projetos, bugs copiados, e semanas perdidas em setup.

---

## A solução

O **Go Boilerplate** é um template completo e opinado que resolve tudo isso. Clone, renomeie a entidade, e comece a desenvolver features — a infraestrutura já está pronta.

```bash
git clone ... && make setup && make dev
# Servidor rodando com hot reload em ~2 minutos
```

---

## O que vem incluso

### Código pronto para produção

| Feature | O que faz | Por que importa |
| ------- | --------- | --------------- |
| **CRUD completo** | Create, Get, List, Update, Delete | Endpoint funcional de exemplo para copiar |
| **PostgreSQL** | Writer/Reader split, connection pool tunado | Escala com read replicas sem mudar código |
| **Redis Cache** | Cache-aside + singleflight + pool config | Performance com proteção contra cache stampede |
| **Idempotência** | Redis-backed, SHA-256 fingerprint, fail-open | Requests duplicados não causam side effects |
| **UUID v7** | IDs ordenados por tempo, tipo nativo no Postgres | Performance de índice + unicidade global |
| **OpenTelemetry** | Traces + metrics + logs integrados | Observabilidade completa desde o dia 1 |
| **Service Key Auth** | Middleware de autenticação service-to-service | Segurança entre microsserviços |
| **Rate Limiting** | Por IP com smart eviction e shutdown graceful | Proteção contra abuso |

### Qualidade automatizada

| Feature | O que faz | Quando roda |
| ------- | --------- | ----------- |
| **223 testes** | Unit + sqlmock + E2E com TestContainers | `make test` |
| **89% coverage** | Domain, usecases, middleware, pkg — tudo coberto | CI exige 60% mínimo |
| **golangci-lint** | 50+ linters incluindo gosec | Pre-commit + CI |
| **govulncheck** | Scan de vulnerabilidades em dependências | Pre-push + CI |
| **Lefthook** | 3 camadas: pre-commit (fmt), commit-msg (conventional), pre-push (lint+test+vuln) | Automático |

### DevOps pronto

| Feature | O que faz | Comando |
| ------- | --------- | ------- |
| **Docker Compose** | DB + Redis + API tudo em Docker | `make run` |
| **Hot Reload** | Air com rebuild automático | `make dev` |
| **Kubernetes** | Kustomize overlays (dev, hml, prd) | `make kind-setup` |
| **CI/CD** | 4 checks paralelos + Slack notifications | Bitbucket Pipelines |
| **Observabilidade** | ELK 8.13 + OTel + dashboard 20 painéis + 6 alertas | `make observability-up` |
| **Load Tests** | k6 com 4 cenários (smoke, load, stress, spike) | `make load-smoke` |
| **Migrations** | Goose SQL com ArgoCD PreSync | `make migrate-up` |

### DX (Developer Experience)

| Feature | O que faz |
| ------- | --------- |
| **40+ make targets** | Tudo via Makefile com help categorizado |
| **Prerequisite checks** | Falta Docker? k6? kind? O Makefile avisa como instalar |
| **Claude Code** | Skills, hooks, agents, rules — IA integrada ao workflow |
| **DevContainer** | Sandbox seguro com firewall default-deny |
| **Conventional Commits** | Enforced por Lefthook |

---

## Números

| Métrica | Valor |
| ------- | ----- |
| Testes passando | **223** |
| Coverage (código com lógica) | **89%** |
| Arquivos Go | **80** |
| Make targets | **40+** |
| ADRs documentados | **8** |
| Guides | **5** |
| Alerting rules | **6** |
| Dashboard panels | **20** |
| Vulnerabilidades conhecidas | **0** |

---

## Estrutura do projeto

O código é organizado em **camadas com responsabilidades claras**. Cada camada só depende da anterior — nunca o contrário.

```
Domain          Entidades, regras de negócio, value objects
    ^           (zero dependências externas)
    |
Use Cases       Operações de negócio (1 arquivo por operação)
    ^           (define interfaces, implementação fica na camada abaixo)
    |
Infrastructure  Banco, cache, HTTP, mensageria
                (implementa as interfaces definidas acima)
```

**Por que isso importa na prática?**

- **Testabilidade**: use cases testados com mocks simples, sem precisar de banco
- **Onboarding**: dev novo sabe exatamente onde colocar cada tipo de código
- **Manutenibilidade**: trocar Postgres por DynamoDB? Só muda a infra, use cases não mudam
- **Independência**: 5 devs podem trabalhar em paralelo sem pisar um no outro

Não é teoria — é como os serviços do ecossistema Appmax já funcionam em produção.

> **Nota**: se preferir uma abordagem mais simples para serviços pequenos, o template funciona perfeitamente com uma estrutura flat — basta mover os use cases para dentro dos handlers. A arquitetura em camadas é uma sugestão, não uma imposição.

---

## Comparativo: sem template vs com template

| Tarefa | Sem template | Com Go Boilerplate |
| ------ | ------------ | ------------------ |
| Setup do projeto | 2-3 dias | `make setup` (2 min) |
| Primeiro endpoint | 1-2 dias | Já vem pronto (CRUD completo) |
| CI/CD | 1 semana | Já configurado (Bitbucket Pipelines) |
| Kubernetes | 1-2 semanas | `make kind-setup` (5 min) |
| Observabilidade | "a gente vê depois" | `make observability-setup` (1 min) |
| Testes | "a gente escreve depois" | 223 testes de exemplo |
| Padronização | Cada serviço diferente | Mesmo padrão em todos |

---

## Como usar

### 1. Clone e renomeie

```bash
git clone https://bitbucket.org/appmax-space/go-boilerplate my-service
cd my-service

# Renomear entity_example para seu domínio
# (find+replace em todo o projeto)
```

### 2. Configure

```bash
cp .env.example .env
# Editar .env com suas configs
make setup
```

### 3. Desenvolva

```bash
make dev          # Hot reload
make test         # Testes
make lint         # Linters
make run          # Tudo em Docker
```

### 4. Deploy

```bash
make kind-setup   # Testar localmente no Kubernetes
# Push para develop → CI roda → deploy automático via ArgoCD
```

---

## Pacotes reutilizáveis (pkg/)

Estes pacotes podem ser importados por **qualquer serviço Go** — não só quem usa o template:

| Pacote | O que faz |
| ------ | --------- |
| `pkg/apperror` | Erros estruturados com código HTTP |
| `pkg/httputil` | Respostas JSON padronizadas |
| `pkg/cache` | Interface de cache + Redis + singleflight |
| `pkg/database` | PostgreSQL Writer/Reader cluster |
| `pkg/idempotency` | Idempotência distribuída via Redis |
| `pkg/logutil` | Logging estruturado com contexto + PII masking |
| `pkg/telemetry` | Setup OTel (traces + HTTP metrics + DB pool) |
| `pkg/health` | Health checker com timeouts |

---

## Decisões documentadas (ADRs)

Cada decisão técnica tem um documento explicando o **por quê**:

| ADR | Decisão |
| --- | ------- |
| 001 | Organização em camadas com DI manual |
| 002 | UUID v7 para IDs (não auto-increment, não UUID v4) |
| 003 | Configuração via env vars (não YAML, não Viper) |
| 004 | Erros de domínio puros (sem HTTP no domain) |
| 005 | Service Key para auth entre serviços |
| 006 | Migrations via ArgoCD PreSync Job |
| 007 | Pacotes reutilizáveis em pkg/ |
| 008 | Formato padrão de resposta da API |

---

## Roadmap

O template está em evolução contínua. Próximos passos planejados:

- [ ] Outbox pattern para eventos assíncronos (SQS/SNS)
- [ ] gRPC support como alternativa ao REST
- [ ] Feature flags com LaunchDarkly/Unleash
- [ ] Uber Fx como opção de DI (guide já documentado)
- [ ] Template CLI para scaffold automático (`boilerplate new my-service`)

---

## FAQ

**"Clean Architecture não é over-engineering pra Go?"**

O template usa camadas simples com DI manual — sem frameworks, sem reflection, sem magia. São 3 diretórios (domain, usecases, infrastructure) com regras claras de dependência. Se for muito pro seu caso, colapse as camadas. O valor real é a **padronização entre serviços**, não a arquitetura em si.

**"Por que não usar framework X?"**

O template usa Gin (HTTP), sqlx (DB), go-redis (cache) — bibliotecas maduras e amplamente adotadas. Não usa ORMs, DI frameworks, ou geradores de código. Quanto menos magia, mais fácil de debugar.

**"Posso usar só partes do template?"**

Sim. Os pacotes em `pkg/` são independentes. Pode importar `pkg/cache` ou `pkg/apperror` em qualquer projeto Go sem usar o template inteiro.

**"Como atualizo meu serviço quando o template evolui?"**

O template é um ponto de partida, não um fork contínuo. Acompanhe o CHANGELOG e adote as melhorias que fizerem sentido para seu serviço.

---

> **TL;DR**: Clone, renomeie, `make setup`, desenvolva features. A infraestrutura já está resolvida.
