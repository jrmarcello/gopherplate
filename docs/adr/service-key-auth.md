# ADR: Autenticação entre Microserviços via Service Keys

**Status**: Aceito  
**Data**: 2026-01-13  
**Autores**: Marcelo Jr

## Contexto

O `people-service-registry` é um serviço interno que será consumido por outros microserviços da organização:

- **banking-router** (primeiro consumidor - piloto)
- **ledger** (próximo consumidor planejado)
- Potencialmente 3-4 serviços no futuro

Precisamos definir uma estratégia de autenticação que:

1. Proteja as rotas contra acessos não autorizados
2. Permita auditoria granular (qual serviço fez qual operação)
3. Seja simples de implementar e manter
4. Escale para múltiplos consumidores

## Decisão

Implementaremos **autenticação via Service Keys com suporte a múltiplas chaves**.

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

### Rotas Protegidas

- **Protegidas**: Todas as rotas de negócio (`/people/*`, `/companies/*`)
- **Públicas**: `/health`, `/ready` (para probes do Kubernetes)

## Alternativas Consideradas

### 1. Chave Única Compartilhada

- **Prós**: Mais simples
- **Contras**: Sem auditoria granular, revogação afeta todos

### 2. JWT Interno

- **Prós**: Pode carregar claims, padrão conhecido
- **Contras**: Overhead de assinatura/verificação, complexidade desnecessária sem propagação de identidade

### 3. mTLS via Service Mesh

- **Prós**: Segurança máxima, sem código adicional
- **Contras**: Requer Istio/Linkerd, overhead operacional significativo

## Consequências

### Positivas

- **Auditoria**: Logs incluem `caller_service` identificando o chamador
- **Revogação Granular**: Pode revogar acesso de um serviço sem afetar outros
- **Simplicidade**: Implementação em ~50 linhas de código
- **Preparação**: Base para futura feature de Auditoria completa

### Negativas

- **Rotação Manual**: Requer coordenação para rotacionar chaves
- **Segurança Limitada**: Não há renovação automática como em OAuth

### Mitigações

- Chaves armazenadas em Kubernetes Secrets (não em ConfigMap)
- Futura integração com AWS Secrets Manager para rotação

## Implementação

### Componentes

1. **Middleware** `ServiceKeyAuth` em `internal/infrastructure/web/middleware/`
2. **Configuração** parseada no bootstrap da aplicação
3. **Contexto** com `caller_service` para logging

### Formato da Chave

```text
sk_<service-name>_<random-32-chars>
```

Exemplo: `sk_banking_router_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6`

## Referências

- [Zero Trust Architecture - NIST](https://www.nist.gov/publications/zero-trust-architecture)
- [API Key Best Practices](https://cloud.google.com/docs/authentication/api-keys)
