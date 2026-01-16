# ADR-002: Estratégia de Execução de Migrations

**Status**: Aceito  
**Data**: 2026-01-16  
**Autor**: Equipe de Engenharia

---

## Contexto

O serviço utiliza PostgreSQL e precisa de uma estratégia consistente para aplicar migrations de banco de dados em múltiplos ambientes (dev, hom, prd).

**Características do ambiente:**

- 3-5 réplicas do serviço em produção
- ArgoCD para GitOps e deploys
- Migrations tipicamente rápidas (<30s)

---

## Decisão

Adotar **ArgoCD PreSync Job** com **binário separado** para migrations.

### Por que binário separado?

O padrão idiomático Go para microserviços é criar binários separados em `cmd/`:

```text
cmd/
├── api/         → ./api (servidor HTTP)
└── migrate/     → ./migrate (migrations)
```

| Abordagem | Quando usar |
| ----------- | ----------- |
| `if os.Args[1]` | Não idiomático, difícil de testar |
| `cobra` / CLI libs | CLIs complexas com muitas flags |
| **Binários separados** ✅ | **Padrão Go para microserviços** |

> **Referências**: Kubernetes, Prometheus, e outros projetos Go usam binários separados.
> Ver [Go Project Layout](https://github.com/golang-standards/project-layout).

---

## Alternativas Consideradas

| Estratégia | Veredicto | Motivo |
| ---------- | ----------- | -------- |
| Manual (CLI) | ❌ Rejeitado | Erro humano, não escala |
| Pipeline CI/CD | ❌ Rejeitado | Requer acesso VPN/security groups ao banco |
| Auto-migrate no startup | ❌ Rejeitado | Race condition com múltiplas réplicas |
| Init Container | ⚠️ Alternativa | Válido, mas menos controle |
| **ArgoCD PreSync Job** | ✅ **Escolhido** | Nativo do ArgoCD, roda 1x antes do deploy |

---

## Implementação

### Arquitetura

```text
┌─────────────────────────────────────────────────────────────┐
│                      ArgoCD Sync                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. PreSync Hook                                            │
│     ┌─────────────────────┐                                 │
│     │  Migration Job      │──────► PostgreSQL               │
│     │  ./migrate          │        (goose up)               │
│     └─────────────────────┘                                 │
│              │                                              │
│              ▼                                              │
│         ✅ Sucesso ──────────────────────────────────────►  │
│         ❌ Falha ───► Sync PARA, Deployment NÃO atualiza    │
│                                                             │
│  2. Sync (se PreSync OK)                                    │
│     ┌─────────────────────┐                                 │
│     │  Deployment         │ (3-5 réplicas)                  │
│     │  Service            │                                 │
│     │  ConfigMap/Secrets  │                                 │
│     └─────────────────────┘                                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Componentes

1. **Binário `migrate`** (`cmd/migrate/main.go`)
   - Single-purpose: aplica migrations pendentes
   - Usa goose com PostgreSQL
   - Exit code 0 = sucesso, 1 = falha

2. **Kubernetes Job com PreSync Hook**
   - Roda automaticamente antes de cada sync
   - Usa mesma imagem do app (contém ambos binários)
   - Falha bloqueia deploy

3. **Desenvolvimento local (Kind)**
   - `make migrate-up` para dev com Docker Compose
   - `make kind-migrate` para dev com Kind
   - `go run ./cmd/migrate` para testar binário

> [!IMPORTANT]
> **O hook PreSync é específico do ArgoCD.** No Kind puro sem ArgoCD, o Job é
> aplicado junto com os outros recursos (sem ordenação). Por isso usamos
> `make kind-migrate` manualmente em desenvolvimento local.

### Fluxo por Ambiente

| Ambiente | Ferramenta | Fluxo de Migrations |
| ---------- | ---------- | --------------------- |
| **Dev Local (Docker)** | Docker Compose | `make migrate-up` manual |
| **Dev Local (Kind)** | Kind + kubectl | `make kind-migrate` manual |
| **HOM/PROD** | ArgoCD | Job PreSync **automático** |

---

## Consequências

### Positivas

- Migrations executadas de forma consistente
- Falha na migration impede deploy quebrado
- Auditável via logs do ArgoCD
- Binário separado = testável isoladamente
- Segue padrão idiomático Go

### Negativas

- Dois binários para buildar/manter
- Migrations longas podem atrasar deploy

### Mitigações

- Dockerfile já builda ambos binários
- Monitorar tempo de migration; quebrar grandes migrations

---

## Referências

- [ArgoCD Resource Hooks](https://argo-cd.readthedocs.io/en/stable/user-guide/resource_hooks/)
- [Goose Migrations](https://github.com/pressly/goose)
- [Go Project Layout](https://github.com/golang-standards/project-layout)
