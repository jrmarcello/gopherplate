# ADR-007: Pacotes Reutilizáveis em pkg/

**Status**: Aceito  
**Data**: 2026-01-20  
**Autor**: Equipe de Engenharia

---

## Contexto

À medida que múltiplos serviços Go evoluem, padrões comuns (erros estruturados, respostas HTTP, logs, telemetria, cache, banco de dados) são replicados entre projetos, gerando inconsistências e retrabalho.

Precisamos de uma estratégia clara para organizar código reutilizável vs. código específico do serviço.

---

## Decisão

Adotar o diretório **`pkg/`** no nível raiz do projeto para pacotes reutilizáveis entre serviços, seguindo o [Go Project Layout](https://github.com/golang-standards/project-layout).

### Estrutura

```text
pkg/
├── apperror/      → Erros estruturados com código, mensagem e HTTP status
├── httputil/      → Helpers de resposta HTTP padronizada
├── ctxkeys/       → Chaves tipadas para context.Value
├── logutil/       → Logging estruturado com propagação de contexto
├── telemetry/     → Setup OpenTelemetry + HTTP metrics + DB pool metrics
├── cache/         → Interface de cache + implementação Redis
├── database/      → Conexão PostgreSQL com Writer/Reader cluster
└── idempotency/   → Store interface + RedisStore para idempotência distribuída
```

### Regras

| Regra | Descrição |
| ----- | --------- |
| **Sem dependências internas** | `pkg/` NUNCA importa de `internal/` |
| **Interface-first** | Pacotes expõem interfaces quando possível |
| **Zero config padrão** | Construtores com defaults razoáveis (`DefaultConfig()`) |
| **Opt-in** | Funcionalidades opcionais via builder pattern |

### pkg/ vs internal/

| Critério | `pkg/` | `internal/` |
| -------- | ------ | ----------- |
| Reutilizável entre serviços | ✅ | ❌ |
| Lógica de negócio | ❌ | ✅ |
| Infraestrutura genérica | ✅ | ❌ |
| Regras de domínio | ❌ | ✅ |

---

## Alternativas Consideradas

| Abordagem | Veredicto | Motivo |
| --------- | --------- | ------ |
| `internal/pkg/` | ❌ Rejeitado | Não permite reutilização entre serviços |
| Módulo Go separado | ❌ Rejeitado | Complexidade de versionamento prematura |
| **`pkg/` no projeto** | ✅ Aceito | Equilíbrio entre reutilização e simplicidade |

---

## Consequências

### Positivas

- Código padronizado entre serviços
- Menor duplicação de infraestrutura
- Pacotes testáveis de forma isolada
- Caminho claro para extração futura como módulo independente

### Negativas

- Disciplina necessária para manter `pkg/` sem dependências internas
- Mudanças em `pkg/` afetam todos os consumidores

### Riscos

- Se `pkg/` crescer demais, considerar extração como módulo Go separado

---

## Referências

- [Go Project Layout](https://github.com/golang-standards/project-layout)
- ADR-001: Clean Architecture
- ADR-009: Error Handling Refactor (supersede ADR-004)
