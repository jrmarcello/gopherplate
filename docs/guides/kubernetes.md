# Kubernetes Deploy - Explicação dos Arquivos

Este documento explica a estrutura de deploy do `gopherplate` no EKS.

## Estrutura de Diretórios

```text
deploy/
├── base/                        # Manifests base (shared)
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── hpa.yaml
│   ├── serviceaccount.yaml
│   ├── migration-job.yaml       # Job de migration (ArgoCD PreSync)
│   ├── networkpolicy.yaml       # Regras de rede (Ingress/Egress)
│   └── kustomization.yaml
└── overlays/
    ├── develop/                 # Overlay para desenvolvimento local (Kind)
    │   ├── kustomization.yaml
    │   ├── configmap.yaml
    │   ├── secret.yaml
    │   ├── deployment-patch.yaml
    │   ├── hpa-patch.yaml
    │   ├── ingress-patch.yaml
    │   ├── kind-config.yaml     # Config do cluster Kind
    │   ├── kind-postgres.yaml   # PostgreSQL local
    │   └── kind-redis.yaml      # Redis local
    ├── homologacao/             # Overlay para homologação (AWS EKS)
    │   ├── kustomization.yaml
    │   ├── configmap.yaml
    │   ├── secret.yaml
    │   ├── deployment-patch.yaml
    │   ├── hpa-patch.yaml
    │   └── ingress-host-patch.yaml
    └── producao/                # Overlay para produção (AWS EKS)
        ├── kustomization.yaml
        ├── configmap.yaml
        ├── secret.yaml
        ├── deployment-patch.yaml
        ├── hpa-patch.yaml
        └── ingress-host-patch.yaml
```

---

## Base (deploy/base/)

### deployment.yaml

Define COMO a aplicação roda no cluster.

| Campo | Propósito |
| --- | --- |
| `replicas` | Instâncias simultâneas (default: 3) |
| `image` | Imagem Docker (ECR) |
| `resources` | CPU/memória (requests/limits) |
| `livenessProbe` | Reinicia pod se `/health` falhar |
| `readinessProbe` | Só envia tráfego se `/ready` OK |
| `securityContext` (pod) | `runAsNonRoot: true`, `runAsUser: 1000`, `fsGroup: 1000` |
| `securityContext` (container) | `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, `drop: ["ALL"]` |
| `topologySpreadConstraints` | Distribui pods entre zonas de disponibilidade |
| `podAntiAffinity` | Evita múltiplos pods no mesmo node |

#### Security Context do Container

O container da aplicação roda com segurança reforçada:

```yaml
securityContext:
  readOnlyRootFilesystem: true      # Filesystem somente leitura
  allowPrivilegeEscalation: false   # Impede escalação de privilégios
  capabilities:
    drop: ["ALL"]                   # Remove todas as Linux capabilities
```

Um volume `emptyDir` é montado em `/app/data` para escrita temporária quando necessário.

### service.yaml

Cria DNS interno para os pods.

### ingress.yaml

Expõe o Service via HTTP externamente.

### hpa.yaml

Escala automaticamente baseado em CPU/memória (3-9 pods).

### serviceaccount.yaml

Identidade do pod (RBAC, IRSA).

### migration-job.yaml

Job Kubernetes para executar migrations de banco de dados. Em ambientes com ArgoCD, configurado como PreSync Hook (roda antes do deploy). Ver [ADR-006](../adr/006-migration-strategy.md).

### networkpolicy.yaml

Define regras de rede para o pod da aplicação, restringindo tráfego de entrada e saída:

| Direção | Regra | Porta |
| ------- | ----- | ----- |
| **Ingress** | Aceita tráfego apenas do namespace `ingress-nginx` | TCP 8080 |
| **Egress** | DNS (CoreDNS) | UDP/TCP 53 |
| **Egress** | PostgreSQL | TCP 5432 |
| **Egress** | Redis | TCP 6379 |
| **Egress** | OpenTelemetry Collector (gRPC + HTTP) | TCP 4317, 4318 |

> **Importante:** Qualquer nova dependência de rede (ex: outro serviço, API externa) precisa ser adicionada às regras de Egress do NetworkPolicy.

---

## Overlays

### develop/

Configuração para rodar localmente com **Kind** (Kubernetes in Docker).

```bash
# Setup completo (cluster + postgres + migrations + deploy)
make kind-setup

# Ou passo a passo:
make kind-up         # Cria cluster Kind com NGINX Ingress
make kind-deploy     # Build, load image, migrate e deploy
make kind-migrate    # Roda migrations via port-forward

# Operação
make kind-logs       # Ver logs da aplicação
make kind-status     # Status dos pods/services/ingress/hpa

# Cleanup
make kind-down       # Remove cluster Kind
```

### homologacao/

Sobrescreve valores para ambiente de homologação AWS:

- Namespace: `gopherplate-homologacao`
- Host: `*.max-homolog.internal`
- ExternalSecret do AWS Secrets Manager

### producao/

Sobrescreve valores para ambiente de produção AWS:

- Namespace: `gopherplate-producao`
- SSL: `DB_SSLMODE: "require"`
- OpenTelemetry: Collector URL configurado
- ExternalSecret do AWS Secrets Manager

---

## Comandos Úteis

```bash
# Renderizar manifests base
kubectl kustomize deploy/base/

# Renderizar overlay develop (local)
kubectl kustomize deploy/overlays/develop/

# Renderizar overlay homologacao
kubectl kustomize deploy/overlays/homologacao/

# Renderizar overlay producao
kubectl kustomize deploy/overlays/producao/

# Aplicar (dry-run)
kubectl apply -k deploy/overlays/homologacao/ --dry-run=client
```
