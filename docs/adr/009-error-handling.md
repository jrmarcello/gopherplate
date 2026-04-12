# ADR-009: Refatoracao do Error Handling — Classificacao no Use Case

**Status**: Aceito  
**Data**: 2026-04-11  
**Autor**: Marcelo Jr

**Supersede**: ADR-004 (Error Handling Layered Translation)

---

## Contexto

O sistema de error handling definido no ADR-004 estabeleceu a traducao em camadas como padrao, mantendo o dominio puro e delegando a traducao para HTTP ao handler. Na pratica, essa abordagem gerou 4 requisitos conflitantes:

| Requisito | Descricao | Tensao |
| --------- | --------- | ------ |
| **Pureza do dominio** | Erros de dominio nao podem conhecer HTTP nem codigos de aplicacao | Dominio deve ser 100% isolado |
| **Traducao HTTP** | Handler precisa mapear erros para status codes HTTP | Requer conhecimento de TODOS os erros de dominio |
| **Observabilidade** | Spans OTel precisam ser classificados (Error vs Warning) | Quem decide se o erro e esperado ou inesperado? |
| **Extensibilidade (OCP)** | Adicionar novo erro nao deve exigir alterar handler | `translateError()` requer switch-case crescente |

O problema concreto: a funcao `translateError()` no handler importava diretamente pacotes de dominio (`userdomain`, `roledomain`, `vo`) e crescia a cada novo erro adicionado. Isso viola o Open-Closed Principle e acopla a camada de infraestrutura ao dominio.

---

## Decisao

Adotamos um sistema de **classificacao de erros no use case** com resolucao generica no handler:

1. **Use cases** convertem erros de dominio em `*apperror.AppError` via funcao local `toAppError()`
2. **Handler** resolve erros genericamente via `errors.As(err, &appErr)` + mapa `codeToStatus`
3. **Classificacao de span** acontece no use case via `shared.ClassifyError()`

### Responsabilidades Atualizadas

| Camada | Responsabilidade | Conhece HTTP? | Conhece OTel? |
| ------ | ---------------- | ------------- | -------------- |
| **Domain** | Erros semanticos puros (`ErrNotFound`, `ErrInvalidEmail`) | Nao | Nao |
| **Use Case** | Converte para `*apperror.AppError` + classifica span | Nao | Sim (spans) |
| **Handler** | Resolve `*apperror.AppError` via `errors.As()` + `codeToStatus` | Sim | Nao* |

> \* Handler nao chama `span.SetStatus()` nem `span.RecordError()` diretamente — a classificacao de spans e responsabilidade do use case.

### Fluxo

```
Domain Error (puro)
    |
    v
Use Case: toAppError() --> *apperror.AppError (com Code + Message)
    |
    v
Use Case: ClassifyError(span, err, expectedErrors) --> Span classificado
    |
    v
Handler: errors.As(err, &appErr) --> codeToStatus[appErr.Code] --> HTTP Response
```

---

## Alternativas Consideradas

| Estrategia | Veredicto | Motivo |
| ---------- | --------- | ------ |
| God-switch no handler (ADR-004) | Rejeitado | Viola OCP, handler importa dominio, switch cresce indefinidamente |
| Codigos de erro no dominio | Rejeitado | Polui dominio com conceitos de aplicacao (`Code string` na entidade) |
| Middleware de erro global | Rejeitado | Perde contexto especifico do use case para classificacao de span |
| **Erros estruturados no use case** | **Escolhido** | OCP compliant, handler generico, span classificado pelo dono do contexto |

---

## Justificativa

1. **Open-Closed Principle**: Novo erro de dominio requer mudanca apenas no use case (`toAppError` + `expectedErrors`), nao no handler.
2. **Handler generico**: `HandleError()` resolve qualquer `*apperror.AppError` sem importar pacotes de dominio. Zero `switch` sobre erros especificos.
3. **Span classification no use case**: O use case e o unico que sabe se um erro e esperado (validacao, not found) ou inesperado (timeout, connection reset). Essa decisao nao pode ser delegada ao handler.
4. **Preservacao da cadeia de erros**: `apperror.Wrap(err, code, message)` mantem o erro original via `Unwrap()`, permitindo `errors.Is()` funcionar atraves da cadeia inteira.

---

## Consequencias

### Positivas

- Handler nao importa nenhum pacote de dominio — zero acoplamento.
- Adicionar novo erro de dominio nao exige alterar `error.go` do handler.
- Classificacao de span (Error vs Warning) e decidida pelo use case, que tem contexto de negocio.
- `errors.Is()` funciona atraves da cadeia inteira (AppError wraps domain error).
- Mapa `codeToStatus` e a unica fonte de verdade para traducao code-to-HTTP.

### Negativas

- Cada use case precisa de boilerplate: `toAppError()` + `expectedErrors`.
- Dois pontos de manutencao por dominio: erros de dominio E mapeamento no use case.
- Use cases passam a ter dependencia leve do OpenTelemetry (para classificacao de span).

---

## Implementacao

### 1. Erros de Dominio (sem mudanca)

```go
// internal/domain/user/errors.go
var (
    ErrUserNotFound = errors.New("user not found")
)

// internal/domain/user/vo/errors.go
var (
    ErrInvalidEmail = errors.New("email invalido")
    ErrInvalidID    = errors.New("invalid ID")
)
```

### 2. Conversao no Use Case (toAppError)

```go
// internal/usecases/user/create.go

// expectedErrors define erros esperados para classificacao de span.
var createExpectedErrors = []error{
    vo.ErrInvalidEmail,
}

// toAppError converte erros de dominio em *apperror.AppError.
func createToAppError(err error) *apperror.AppError {
    switch {
    case errors.Is(err, vo.ErrInvalidEmail):
        return apperror.Wrap(err, apperror.CodeValidationError, "invalid email")
    default:
        return apperror.Wrap(err, apperror.CodeInternalError, "failed to create user")
    }
}

func (uc *CreateUseCase) Execute(ctx context.Context, input dto.CreateInput) (*dto.CreateOutput, error) {
    // ... logica de negocio ...
    if execErr != nil {
        shared.ClassifyError(span, execErr, createExpectedErrors, "CreateUseCase")
        return nil, createToAppError(execErr)
    }
    // ...
}
```

### 3. Classificacao de Span (shared.ClassifyError)

```go
// internal/usecases/shared/classify.go

func ClassifyError(span trace.Span, err error, expectedErrors []error, context string) {
    for _, expected := range expectedErrors {
        if errors.Is(err, expected) {
            telemetry.WarnSpan(span, "expected_error", err.Error())
            return
        }
    }
    telemetry.FailSpan(span, err, context)
}
```

### 4. Helpers de Span (pkg/telemetry)

```go
// pkg/telemetry/span.go

// FailSpan marca o span como Error e registra o evento de erro.
func FailSpan(span trace.Span, err error, msg string) {
    span.SetStatus(codes.Error, msg)
    span.RecordError(err)
}

// WarnSpan adiciona atributo semantico sem marcar o span como Error.
func WarnSpan(span trace.Span, key, value string) {
    span.SetAttributes(attribute.String(key, value))
}
```

### 5. Handler Generico (sem imports de dominio)

```go
// internal/infrastructure/web/handler/error.go

var codeToStatus = map[string]int{
    apperror.CodeInvalidRequest:  http.StatusBadRequest,
    apperror.CodeValidationError: http.StatusBadRequest,
    apperror.CodeNotFound:        http.StatusNotFound,
    apperror.CodeConflict:        http.StatusConflict,
    apperror.CodeUnauthorized:    http.StatusUnauthorized,
    apperror.CodeForbidden:       http.StatusForbidden,
    apperror.CodeInternalError:   http.StatusInternalServerError,
}

func HandleError(c *gin.Context, span trace.Span, err error) {
    var appErr *apperror.AppError
    if errors.As(err, &appErr) {
        status := codeToStatus[appErr.Code]
        if status == 0 {
            status = http.StatusInternalServerError
        }
        httpgin.SendError(c, status, appErr.Message)
        return
    }
    // Fallback para erros nao estruturados
    httpgin.SendError(c, http.StatusInternalServerError, "internal server error")
}
```

---

## Referencias

- **ADR-004**: Error Handling Layered Translation (predecessor, agora superseded)
- **ADR-008**: Formato Padronizado de Resposta HTTP
- `pkg/apperror/apperror.go`: Definicao de `AppError`, construtores, `Wrap()`
- `pkg/telemetry/span.go`: Helpers `FailSpan()` e `WarnSpan()`
- `internal/usecases/shared/classify.go`: `ClassifyError()` compartilhado
- `docs/guides/error-handling.md`: Guia pratico de error handling
