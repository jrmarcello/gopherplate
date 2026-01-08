# Error Handling

Sistema de tratamento de erros com tradução centralizada e separação de camadas.

## Arquitetura

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                              FLUXO DE ERROS                                  │
└─────────────────────────────────────────────────────────────────────────────┘

    Domain (puro)           Use Case              Handler HTTP
    ───────────             ────────              ────────────
    vo.ErrInvalidCPF   →    person.ErrInvalidCPF  →   400 Bad Request
    person.ErrNotFound →    person.ErrNotFound    →   404 Not Found
    errors.New(...)    →    (passa como está)     →   Traduzido ou 500
```

---

## Camadas de Erro

### 1. Erros de Domínio (Puros)

```go
// domain/person/vo/errors.go - NÃO conhece HTTP
var ErrInvalidCPF = errors.New("CPF inválido")
```

O domínio não sabe o que é um HTTP status. Apenas diz "isso está errado".

**Arquivos:**
- `internal/domain/person/vo/errors.go`
- `internal/domain/person/errors.go`

---

### 2. Erros de Use Case (Application)

```go
// usecases/person/errors.go - Conhece HTTP
var ErrInvalidCPF = usecases.BadRequest("INVALID_CPF", "CPF informado é inválido")
```

A camada de aplicação define **como** o erro deve ser tratado em termos de HTTP.

**Arquivos:**
- `internal/usecases/errors.go` - AppError base
- `internal/usecases/person/errors.go` - Erros específicos

---

### 3. HandleError (Tradução)

```go
func HandleError(c *gin.Context, span trace.Span, err error) {
    // Passo 1: Já é um AppError? Usa diretamente
    var appErr *usecases.AppError
    if errors.As(err, &appErr) {
        respondWithAppError(c, span, appErr, traceID)
        return
    }

    // Passo 2: É um erro de domínio? Traduz para AppError
    if translated := translateDomainError(err); translated != nil {
        respondWithAppError(c, span, translated, traceID)
        return
    }

    // Passo 3: Erro desconhecido → 500
    c.JSON(500, ErrorResponse{...})
}
```

**Arquivo:** `internal/infrastructure/web/handler/error.go`

---

### 4. Translator (Mapeamento)

```go
func translateDomainError(err error) *usecases.AppError {
    switch {
    case errors.Is(err, vo.ErrInvalidCPF):      // domínio
        return personuc.ErrInvalidCPF           // → 400
    case errors.Is(err, person.ErrNotFound):    // domínio
        return personuc.ErrPersonNotFound       // → 404
    default:
        return nil  // não traduzido
    }
}
```

---

## Uso no Handler

```go
func (h *PersonHandler) Create(c *gin.Context) {
    res, err := h.CreateUC.Execute(ctx, req)
    if err != nil {
        HandleError(c, span, err)  // ← Uma linha só
        return
    }
    c.JSON(201, res)
}
```

---

## Formato de Resposta

```json
{
  "error": "CPF informado é inválido",
  "code": "INVALID_CPF",
  "details": null,
  "trace_id": "abc123..."
}
```

---

## Adicionando Novos Erros

### 1. Erro de Domínio Puro

```go
// domain/person/vo/errors.go
var ErrInvalidAddress = errors.New("endereço inválido")
```

### 2. Erro de Use Case

```go
// usecases/person/errors.go
var ErrInvalidAddress = usecases.BadRequest("INVALID_ADDRESS", "Endereço inválido")
```

### 3. Adicionar ao Translator

```go
// handler/error.go
case errors.Is(err, vo.ErrInvalidAddress):
    return personuc.ErrInvalidAddress
```

---

## Benefícios

| Antes | Depois |
|-------|--------|
| Switch gigante no handler | Função única `HandleError()` |
| Resposta `gin.H{}` inconsistente | `ErrorResponse` padronizado |
| Domínio misturado com HTTP | Domínio puro, tradução separada |
| 80+ linhas de mapeamento | ~20 linhas no translator |
