# ADR-005: Service Key Auth

**Status**: Aceito  
**Data**: 2026-01-16  
**Autor**: Marcelo Jr

---

## Contexto

O serviço será consumido por outros microserviços da organização. Precisamos definir uma estratégia de autenticação que:

1. Proteja as rotas contra acessos não autorizados
2. Permita auditoria granular (qual serviço fez qual operação)
3. Seja simples de implementar e manter
4. Escale para múltiplos consumidores

---

## Decisão

Implementamos **autenticação via Service Keys com suporte a múltiplas chaves**.

### Mecanismo

Cada serviço consumidor recebe uma chave única. As requisições devem incluir dois headers:

```http
X-Service-Name: banking-router
X-Service-Key: sk_banking_router_abc123def456
```

### Configuração

As chaves são configuradas via variável de ambiente:

```bash
SERVICE_KEYS="banking-router:sk_banking_...,ledger:sk_ledger_..."
```

### Rotas

- **Protegidas**: Todas as rotas de negócio (`/users/*`, `/roles/*`)
- **Públicas**: `/health`, `/ready`, `/swagger/*` (probes do Kubernetes e docs)

### Dev Mode

Se `SERVICE_KEYS` estiver vazio ou não configurado, o middleware permite todas as requisições. Isso facilita o desenvolvimento local.

---

## Alternativas Consideradas

| Estratégia | Veredicto | Motivo |
| ---------- | --------- | ------ |
| OAuth2 / JWT (Keycloak) | ❌ Rejeitado | Complexidade alta de infraestrutura para o estágio atual |
| Basic Auth | ❌ Rejeitado | Menos flexível para auditoria e rotação granular |
| mTLS (Service Mesh) | ❌ Rejeitado | Complexidade operacional excessiva sem time de SRE dedicado |
| **Service Keys** | ✅ **Escolhido** | Simples, auditável, fácil de rotacionar (redesploi ou configmap) |

---

## Consequências

### Positivas

- **Auditoria**: Logs incluem `caller_service` identificando o chamador
- **Revogação Granular**: Pode revogar acesso de um serviço sem afetar outros
- **Simplicidade**: Implementação em ~100 linhas de código
- **Dev-friendly**: Sem auth em desenvolvimento local

### Negativas

- **Rotação Manual**: Requer coordenação para rotacionar chaves
- **Segurança Limitada**: Não há renovação automática como em OAuth

### Mitigações

- Chaves armazenadas em Kubernetes Secrets (não em ConfigMap)
- Futura integração com AWS Secrets Manager para rotação

---

## Implementação

### Componentes

1. **Middleware** `ServiceKeyAuth` em `internal/infrastructure/web/middleware/service_key.go`
2. **Configuração** `Auth.ServiceKeys` em `config/config.go`
3. **Router** aplica middleware em grupo protegido

### Formato da Chave

```text
sk_<service-name>_<random-32-chars>
```

Exemplo: `sk_banking_router_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6`
