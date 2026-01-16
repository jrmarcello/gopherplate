# Kubernetes Deploy - Explicação dos Arquivos

Este documento explica a estrutura de deploy do `go-boilerplate` no EKS.

## Estrutura de Diretórios

```text
deploy/
├── base/                      # Manifests base (shared)
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── hpa.yaml
│   ├── serviceaccount.yaml
│   └── kustomization.yaml
└── overlays/
    ├── dev-local/             # Overlay para desenvolvimento local (kind)
    │   ├── kustomization.yaml
    │   ├── configmap.yaml
    │   ├── secret.yaml
    │   ├── deployment-patch.yaml
    │   ├── ingress-patch.yaml
    │   ├── kind-config.yaml   # Config do cluster kind
    │   └── kind-postgres.yaml # PostgreSQL local
    └── homologacao/           # Overlay para ambiente de homologação
        ├── kustomization.yaml
        ├── configmap.yaml
        ├── secret.yaml
        ├── deployment-patch.yaml
        └── ingress-host-patch.yaml
```

---

## Base (deploy/base/)

### deployment.yaml

Define COMO a aplicação roda no cluster.

| Campo | Propósito |
|-------|-----------|
| `replicas` | Instâncias simultâneas |
| `image` | Imagem Docker (ECR) |
| `resources` | CPU/memória (requests/limits) |
| `livenessProbe` | Reinicia pod se `/health` falhar |
| `readinessProbe` | Só envia tráfego se `/ready` OK |

### service.yaml

Cria DNS interno para os pods.

### ingress.yaml

Expõe o Service via HTTP externamente.

### hpa.yaml

Escala automaticamente baseado em CPU/memória (3-9 pods).

### serviceaccount.yaml

Identidade do pod (RBAC, IRSA).

---

## Overlays

### dev-local/

Configuração para rodar localmente com **kind** (Kubernetes in Docker).

```bash
# Setup inicial
make kind-up

# Deploy
make kind-deploy

# Acessar
curl http://entities.localhost/health

# Logs
make kind-logs

# Cleanup
make kind-down
```

### homologacao/

Sobrescreve valores para ambiente de homologação AWS:
- Namespace: `go-boilerplate-homologacao`
- Host: `*.max-homolog.internal`
- ExternalSecret do AWS Secrets Manager

---

## Comandos Úteis

```bash
# Renderizar manifests base
kubectl kustomize deploy/base/

# Renderizar overlay dev-local
kubectl kustomize deploy/overlays/dev-local/

# Renderizar overlay homologacao
kubectl kustomize deploy/overlays/homologacao/

# Aplicar (dry-run)
kubectl apply -k deploy/overlays/homologacao/ --dry-run=client
```