# ADR-008: Formato Padronizado de Resposta HTTP

**Status**: Aceito  
**Data**: 2026-01-20  
**Autor**: Equipe de Engenharia

---

## Contexto

APIs sem formato padronizado de resposta geram inconsistências que dificultam a integração por consumidores. Cada endpoint retornando um formato diferente aumenta a complexidade do front-end e de integrações entre serviços.

---

## Decisão

Adotar formato padronizado de resposta HTTP usando os helpers de **`pkg/httputil`**.

### Formato de Sucesso

```json
{
  "data": { ... }
}
```

### Formato de Sucesso com Metadados (Listagens)

```json
{
  "data": [ ... ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 100,
    "total_pages": 10
  },
  "links": {
    "next": "/api/v1/users?page=2",
    "prev": null
  }
}
```

### Formato de Erro

```json
{
  "errors": {
    "message": "descrição do erro",
    "code": "VALIDATION_ERROR",
    "details": {
      "email": "formato inválido"
    }
  }
}
```

### Uso nos Handlers

```go
// Sucesso simples
httputil.SendSuccess(c, http.StatusOK, user)

// Sucesso com metadados (listagem)
httputil.SendSuccessWithMeta(c, http.StatusOK, items, pagination, links)

// Erro simples
httputil.SendError(c, http.StatusBadRequest, "campo obrigatório")

// Erro com código
httputil.SendErrorWithCode(c, http.StatusConflict, "DUPLICATE", "registro já existe")

// Erro com detalhes
httputil.SendErrorWithDetails(c, http.StatusUnprocessableEntity, "validação falhou", details)
```

### Regras

| Regra | Descrição |
| ----- | --------- |
| **Sempre usar helpers** | NUNCA usar `c.JSON()` diretamente |
| **Dados em `data`** | Resposta de sucesso sempre envelopa em `data` |
| **Erros em `errors`** | Resposta de erro sempre envelopa em `errors` |
| **Meta para listagens** | Paginação vai em `meta`, não misturada com dados |
| **HTTP status semântico** | Status code reflete a operação (201 Create, 204 Delete) |

---

## Alternativas Consideradas

| Abordagem | Veredicto | Motivo |
| --------- | --------- | ------ |
| JSON:API spec | ❌ Rejeitado | Complexidade desnecessária para APIs internas |
| Sem envelope | ❌ Rejeitado | Dificulta distinção entre dados e metadados |
| **Envelope simples** | ✅ Aceito | Flexível, simples, consistente |

---

## Consequências

### Positivas

- Formato previsível para todos os consumidores
- Front-end pode criar wrappers genéricos de resposta
- Erros estruturados facilitam debugging
- Paginação separada dos dados

### Negativas

- Leve overhead de serialização pelo envelope
- Exige disciplina para não usar `c.JSON` diretamente

---

## Referências

- `pkg/httputil/response.go`: Implementação dos helpers
- ADR-004: Error Handling
- ADR-007: Pacotes Reutilizáveis em pkg/
