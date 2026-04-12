# Guia de Error Handling

Este guia descreve como o sistema de error handling funciona neste boilerplate e como estende-lo ao adicionar novos erros ou dominios.

> **ADR de referencia**: [ADR-009 - Refatoracao do Error Handling](../adr/009-error-handling.md)

---

## Principios

O error handling segue 3 camadas, cada uma com responsabilidade clara:

| Camada | O que faz | O que NAO faz |
| ------ | --------- | ------------- |
| **Domain** | Define erros semanticos puros (`errors.New(...)`) | Nao conhece HTTP, codigos de aplicacao, nem OTel |
| **Use Case** | Converte erros de dominio em `*apperror.AppError` + classifica spans | Nao retorna status HTTP diretamente |
| **Handler** | Resolve `*apperror.AppError` para HTTP status via mapa generico | Nao importa pacotes de dominio, nao classifica spans |

**Regra de ouro**: a informacao flui sempre de dentro para fora (domain -> use case -> handler). Nenhuma camada interna conhece conceitos da camada externa.

---

## Fluxo Completo de um Erro

```
1. Domain/VO gera erro puro
   vo.NewEmail("invalido") --> vo.ErrInvalidEmail

2. Use Case captura o erro
   |
   +-- ClassifyError(span, err, expectedErrors, "contexto")
   |       |
   |       +-- Erro esperado?  --> WarnSpan (atributo, sem marcar Error)
   |       +-- Erro inesperado? --> FailSpan (marca Error, RecordError)
   |
   +-- toAppError(err) --> *apperror.AppError{Code, Message, Err}
   |
   +-- return nil, appErr

3. Handler recebe o erro
   |
   +-- errors.As(err, &appErr) --> true
   |
   +-- codeToStatus[appErr.Code] --> HTTP status
   |
   +-- httpgin.SendError(c, status, appErr.Message)
```

---

## Anatomia do Error Handling em um Use Case

Cada use case possui 3 elementos de error handling:

### 1. `expectedErrors` — Lista de erros esperados

Define quais erros sao "normais" (validacao, not found, conflito) vs "inesperados" (timeout, connection reset).

```go
var createExpectedErrors = []error{
    vo.ErrInvalidEmail,
    vo.ErrInvalidID,
}
```

**Proposito**: Alimenta `ClassifyError()` para decidir se o span recebe Warning (esperado) ou Error (inesperado).

### 2. `toAppError()` — Conversao dominio -> aplicacao

Mapeia cada erro de dominio para um `*apperror.AppError` com codigo e mensagem user-friendly.

```go
func createToAppError(err error) *apperror.AppError {
    switch {
    case errors.Is(err, vo.ErrInvalidEmail):
        return apperror.Wrap(err, apperror.CodeValidationError, "invalid email")
    case errors.Is(err, vo.ErrInvalidID):
        return apperror.Wrap(err, apperror.CodeInvalidRequest, "invalid user ID")
    default:
        return apperror.Wrap(err, apperror.CodeInternalError, "failed to create user")
    }
}
```

**Importante**: Sempre use `apperror.Wrap(err, ...)` (nao `apperror.New()`), para preservar a cadeia de erros e permitir `errors.Is()` funcionar.

### 3. `ClassifyError()` — Classificacao de span

Chamada ANTES de retornar o erro, para que o span seja classificado corretamente.

```go
func (uc *CreateUseCase) Execute(ctx context.Context, input dto.CreateInput) (*dto.CreateOutput, error) {
    // ... logica de negocio ...

    emailVO, emailErr := vo.NewEmail(input.Email)
    if emailErr != nil {
        shared.ClassifyError(span, emailErr, createExpectedErrors, "CreateUseCase")
        return nil, createToAppError(emailErr)
    }

    if saveErr := uc.Repo.Create(ctx, entity); saveErr != nil {
        shared.ClassifyError(span, saveErr, createExpectedErrors, "CreateUseCase")
        return nil, createToAppError(saveErr)
    }

    return output, nil
}
```

---

## Mapa `codeToStatus` (Handler)

O handler usa um unico mapa para traduzir codigos de `AppError` em HTTP status:

```go
var codeToStatus = map[string]int{
    apperror.CodeInvalidRequest:  http.StatusBadRequest,     // 400
    apperror.CodeValidationError: http.StatusBadRequest,     // 400
    apperror.CodeNotFound:        http.StatusNotFound,       // 404
    apperror.CodeConflict:        http.StatusConflict,       // 409
    apperror.CodeUnauthorized:    http.StatusUnauthorized,   // 401
    apperror.CodeForbidden:       http.StatusForbidden,      // 403
    apperror.CodeInternalError:   http.StatusInternalServerError, // 500
}
```

**Se o codigo nao existir no mapa**, o handler retorna 500 Internal Server Error.

---

## Classificacao de Spans (OTel)

| Tipo de erro | Funcao | Efeito no span | Exemplo |
| ------------ | ------ | --------------- | ------- |
| **Esperado** | `telemetry.WarnSpan(span, key, value)` | Atributo semantico, span OK | `ErrInvalidEmail`, `ErrNotFound` |
| **Inesperado** | `telemetry.FailSpan(span, err, msg)` | Span marcado como Error + evento de erro | DB timeout, connection reset |

```go
// pkg/telemetry/span.go

func FailSpan(span trace.Span, err error, msg string) {
    span.SetStatus(codes.Error, msg)
    span.RecordError(err)
}

func WarnSpan(span trace.Span, key, value string) {
    span.SetAttributes(attribute.String(key, value))
}
```

**Regra**: O handler NUNCA chama `span.SetStatus()` nem `span.RecordError()`. Essa responsabilidade e exclusiva do use case.

---

## Checklist: Como Adicionar um Novo Erro de Dominio

Ao criar um novo erro de dominio (ex: `ErrEmailAlreadyExists`), siga estes passos:

- [ ] **1. Definir o erro no dominio**
  ```go
  // internal/domain/user/errors.go
  var ErrEmailAlreadyExists = errors.New("email already exists")
  ```

- [ ] **2. Adicionar mapeamento no `toAppError()` de cada use case que pode gerar esse erro**
  ```go
  // internal/usecases/user/create.go
  case errors.Is(err, userdomain.ErrEmailAlreadyExists):
      return apperror.Wrap(err, apperror.CodeConflict, "email already exists")
  ```

- [ ] **3. Adicionar na lista `expectedErrors` (se for um erro de negocio esperado)**
  ```go
  var createExpectedErrors = []error{
      vo.ErrInvalidEmail,
      userdomain.ErrEmailAlreadyExists, // novo
  }
  ```

- [ ] **4. Verificar que o `Code` usado ja existe no mapa `codeToStatus`**
  - `CodeConflict` -> 409: ja mapeado
  - Se nao existir, adicionar ao mapa (ver checklist abaixo)

- [ ] **5. Escrever teste unitario no use case** verificando que o erro correto e retornado

> **Nota**: Nao e necessario alterar o handler (`error.go`). O handler resolve genericamente via `errors.As()`.

---

## Checklist: Como Adicionar um Novo Codigo de Erro

Ao precisar de um novo codigo (ex: `CodeRateLimited`), siga estes passos:

- [ ] **1. Definir a constante em `pkg/apperror/apperror.go`**
  ```go
  const (
      // ... existentes ...
      CodeRateLimited = "RATE_LIMITED"
  )
  ```

- [ ] **2. Adicionar no mapa `codeToStatus` em `internal/infrastructure/web/handler/error.go`**
  ```go
  var codeToStatus = map[string]int{
      // ... existentes ...
      apperror.CodeRateLimited: http.StatusTooManyRequests, // 429
  }
  ```

- [ ] **3. Usar no `toAppError()` do use case**
  ```go
  case errors.Is(err, someDomain.ErrRateLimited):
      return apperror.Wrap(err, apperror.CodeRateLimited, "too many requests")
  ```

- [ ] **4. Documentar no Swagger** se gera um novo status HTTP no endpoint

---

## Codigos de Erro Disponiveis

| Constante | Valor | HTTP Status | Uso |
| --------- | ----- | ----------- | --- |
| `CodeInvalidRequest` | `INVALID_REQUEST` | 400 | Input invalido (ID, parametros) |
| `CodeValidationError` | `VALIDATION_ERROR` | 400 | Validacao de negocio (email, formato) |
| `CodeUnauthorized` | `UNAUTHORIZED` | 401 | Autenticacao ausente ou invalida |
| `CodeForbidden` | `FORBIDDEN` | 403 | Permissao insuficiente |
| `CodeNotFound` | `NOT_FOUND` | 404 | Recurso nao encontrado |
| `CodeConflict` | `CONFLICT` | 409 | Conflito (duplicata, estado invalido) |
| `CodeInternalError` | `INTERNAL_ERROR` | 500 | Erro interno nao mapeado |

---

## Exemplo Completo: Use Case com Error Handling

```go
package user

import (
    "context"
    "errors"

    userdomain "myapp/internal/domain/user"
    "myapp/internal/domain/user/vo"
    "myapp/internal/usecases/shared"
    "myapp/internal/usecases/user/dto"
    "myapp/internal/usecases/user/interfaces"
    "myapp/pkg/apperror"
    "go.opentelemetry.io/otel"
)

// Erros esperados para classificacao de span
var getExpectedErrors = []error{
    vo.ErrInvalidID,
    userdomain.ErrUserNotFound,
}

// toAppError converte erros de dominio em AppError
func getToAppError(err error) *apperror.AppError {
    switch {
    case errors.Is(err, vo.ErrInvalidID):
        return apperror.Wrap(err, apperror.CodeInvalidRequest, "invalid user ID")
    case errors.Is(err, userdomain.ErrUserNotFound):
        return apperror.Wrap(err, apperror.CodeNotFound, "user not found")
    default:
        return apperror.Wrap(err, apperror.CodeInternalError, "failed to get user")
    }
}

func (uc *GetUseCase) Execute(ctx context.Context, input dto.GetInput) (*dto.GetOutput, error) {
    ctx, span := otel.Tracer("usecase").Start(ctx, "GetUseCase.Execute")
    defer span.End()

    id, parseErr := vo.ParseID(input.ID)
    if parseErr != nil {
        shared.ClassifyError(span, parseErr, getExpectedErrors, "GetUseCase")
        return nil, getToAppError(parseErr)
    }

    entity, findErr := uc.Repo.FindByID(ctx, id)
    if findErr != nil {
        shared.ClassifyError(span, findErr, getExpectedErrors, "GetUseCase")
        return nil, getToAppError(findErr)
    }

    return toGetOutput(entity), nil
}
```

---

## Referencias

- [ADR-009: Refatoracao do Error Handling](../adr/009-error-handling.md) — Decisao arquitetural
- [ADR-004: Error Handling Layered Translation](../adr/004-error-handling.md) — Predecessor (superseded)
- `pkg/apperror/apperror.go` — Definicao de `AppError`, constantes de codigos, `Wrap()`
- `pkg/telemetry/span.go` — Helpers `FailSpan()` e `WarnSpan()`
- `internal/usecases/shared/classify.go` — `ClassifyError()` compartilhado entre use cases
