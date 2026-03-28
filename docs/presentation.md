# Go Boilerplate — Template para Microservicos

> De zero a producao em minutos, nao semanas.

---

## O problema

Toda vez que um time cria um novo microservico em Go, ele enfrenta as mesmas perguntas:

- Como organizar o codigo?
- Como configurar DB, cache, observabilidade?
- Como garantir qualidade (lint, testes, CI)?
- Como fazer deploy no Kubernetes?
- Quanto tempo ate o primeiro endpoint funcionar?

A resposta geralmente e: **copiar de outro servico e adaptar**. Isso gera inconsistencia entre projetos, bugs copiados, e semanas perdidas em setup.

---

## A solucao

O **Go Boilerplate** e um template completo e opinado que resolve tudo isso. Clone, renomeie a entidade, e comece a desenvolver features — a infraestrutura ja esta pronta.

```bash
git clone ... && make setup && make dev
# Servidor rodando com hot reload em ~2 minutos
```

---

## O que vem incluso

### Codigo pronto para producao

| Feature | O que faz | Por que importa |
| ------- | --------- | --------------- |
| **CRUD completo** | Create, Get, List, Update, Delete | Endpoint funcional de exemplo para copiar |
| **PostgreSQL** | Writer/Reader split, connection pool tunado | Escala com read replicas sem mudar codigo |
| **Redis Cache** | Cache-aside + singleflight + pool config | Performance com protecao contra cache stampede |
| **Idempotencia** | Redis-backed, SHA-256 fingerprint, fail-open | Requests duplicados nao causam side effects |
| **UUID v7** | IDs ordenados por tempo, tipo nativo no Postgres | Performance de indice + unicidade global |
| **OpenTelemetry** | Traces + metrics + logs integrados | Observabilidade completa desde o dia 1 |
| **Service Key Auth** | Middleware de autenticacao service-to-service | Seguranca entre microservicos |
| **Rate Limiting** | Por IP com smart eviction e shutdown graceful | Protecao contra abuso |

### Qualidade automatizada

| Feature | O que faz | Quando roda |
| ------- | --------- | ----------- |
| **223 testes** | Unit + sqlmock + E2E com TestContainers | `make test` |
| **89% coverage** | Domain, usecases, middleware, pkg — tudo coberto | CI exige 60% minimo |
| **golangci-lint** | 50+ linters incluindo gosec | Pre-commit + CI |
| **govulncheck** | Scan de vulnerabilidades em dependencias | Pre-push + CI |
| **Lefthook** | 3 camadas: pre-commit (fmt), commit-msg (conventional), pre-push (lint+test+vuln) | Automatico |

### DevOps pronto

| Feature | O que faz | Comando |
| ------- | --------- | ------- |
| **Docker Compose** | DB + Redis + API tudo em Docker | `make run` |
| **Hot Reload** | Air com rebuild automatico | `make dev` |
| **Kubernetes** | Kustomize overlays (dev, hml, prd) | `make kind-setup` |
| **CI/CD** | 4 checks paralelos + Slack notifications | Bitbucket Pipelines |
| **Observabilidade** | ELK 8.13 + OTel + dashboard 20 paineis + 6 alertas | `make observability-up` |
| **Load Tests** | k6 com 4 cenarios (smoke, load, stress, spike) | `make load-smoke` |
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

## Numeros

| Metrica | Valor |
| ------- | ----- |
| Testes passando | **223** |
| Coverage (codigo com logica) | **89%** |
| Arquivos Go | **80** |
| Make targets | **40+** |
| ADRs documentados | **8** |
| Guides | **5** |
| Alerting rules | **6** |
| Dashboard panels | **20** |
| Vulnerabilidades conhecidas | **0** |

---

## Estrutura do projeto

O codigo e organizado em **camadas com responsabilidades claras**. Cada camada so depende da anterior — nunca o contrario.

```
Domain          Entidades, regras de negocio, value objects
    ^           (zero dependencias externas)
    |
Use Cases       Operacoes de negocio (1 arquivo por operacao)
    ^           (define interfaces, implementacao fica na camada abaixo)
    |
Infrastructure  Banco, cache, HTTP, mensageria
                (implementa as interfaces definidas acima)
```

**Por que isso importa na pratica?**

- **Testabilidade**: use cases testados com mocks simples, sem precisar de banco
- **Onboarding**: dev novo sabe exatamente onde colocar cada tipo de codigo
- **Manutenibilidade**: trocar Postgres por DynamoDB? So muda a infra, use cases nao mudam
- **Independencia**: 5 devs podem trabalhar em paralelo sem pisar um no outro

Nao e teoria — e como os servicos do ecossistema Appmax ja funcionam em producao.

> **Nota**: se preferir uma abordagem mais simples para servicos pequenos, o template funciona perfeitamente com uma estrutura flat — basta mover os use cases para dentro dos handlers. A arquitetura em camadas e uma sugestao, nao uma imposicao.

---

## Comparativo: sem template vs com template

| Tarefa | Sem template | Com Go Boilerplate |
| ------ | ------------ | ------------------ |
| Setup do projeto | 2-3 dias | `make setup` (2 min) |
| Primeiro endpoint | 1-2 dias | Ja vem pronto (CRUD completo) |
| CI/CD | 1 semana | Ja configurado (Bitbucket Pipelines) |
| Kubernetes | 1-2 semanas | `make kind-setup` (5 min) |
| Observabilidade | "a gente ve depois" | `make observability-setup` (1 min) |
| Testes | "a gente escreve depois" | 223 testes de exemplo |
| Padronizacao | Cada servico diferente | Mesmo padrao em todos |

---

## Como usar

### 1. Clone e renomeie

```bash
git clone https://bitbucket.org/appmax-space/go-boilerplate my-service
cd my-service

# Renomear entity_example para seu dominio
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
# Push para develop → CI roda → deploy automatico via ArgoCD
```

---

## Pacotes reutilizaveis (pkg/)

Estes pacotes podem ser importados por **qualquer servico Go** — nao so quem usa o template:

| Pacote | O que faz |
| ------ | --------- |
| `pkg/apperror` | Erros estruturados com codigo HTTP |
| `pkg/httputil` | Respostas JSON padronizadas |
| `pkg/cache` | Interface de cache + Redis + singleflight |
| `pkg/database` | PostgreSQL Writer/Reader cluster |
| `pkg/idempotency` | Idempotencia distribuida via Redis |
| `pkg/logutil` | Logging estruturado com contexto + PII masking |
| `pkg/telemetry` | Setup OTel (traces + HTTP metrics + DB pool) |
| `pkg/health` | Health checker com timeouts |

---

## Decisoes documentadas (ADRs)

Cada decisao tecnica tem um documento explicando o **por que**:

| ADR | Decisao |
| --- | ------- |
| 001 | Organizacao em camadas com DI manual |
| 002 | UUID v7 para IDs (nao auto-increment, nao UUID v4) |
| 003 | Configuracao via env vars (nao YAML, nao Viper) |
| 004 | Erros de dominio puros (sem HTTP no domain) |
| 005 | Service Key para auth entre servicos |
| 006 | Migrations via ArgoCD PreSync Job |
| 007 | Pacotes reutilizaveis em pkg/ |
| 008 | Formato padrao de resposta da API |

---

## Roadmap

O template esta em evolucao continua. Proximos passos planejados:

- [ ] Outbox pattern para eventos asincronos (SQS/SNS)
- [ ] gRPC support como alternativa ao REST
- [ ] Feature flags com LaunchDarkly/Unleash
- [ ] Uber Fx como opcao de DI (guide ja documentado)
- [ ] Template CLI para scaffold automatico (`boilerplate new my-service`)

---

## FAQ

**"Clean Architecture nao e over-engineering pra Go?"**

O template usa camadas simples com DI manual — sem frameworks, sem reflection, sem magia. Sao 3 diretorios (domain, usecases, infrastructure) com regras claras de dependencia. Se for muito pra seu caso, colapse as camadas. O valor real e a **padronizacao entre servicos**, nao a arquitetura em si.

**"Por que nao usar framework X?"**

O template usa Gin (HTTP), sqlx (DB), go-redis (cache) — bibliotecas maduras e amplamente adotadas. Nao usa ORMs, DI frameworks, ou geradores de codigo. Quanto menos magia, mais facil de debugar.

**"Posso usar so partes do template?"**

Sim. Os pacotes em `pkg/` sao independentes. Pode importar `pkg/cache` ou `pkg/apperror` em qualquer projeto Go sem usar o template inteiro.

**"Como atualizo meu servico quando o template evolui?"**

O template e um ponto de partida, nao um fork contínuo. Acompanhe o CHANGELOG e adote as melhorias que fizerem sentido para seu servico.

---

> **TL;DR**: Clone, renomeie, `make setup`, desenvolva features. A infraestrutura ja esta resolvida.
