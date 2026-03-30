# gRPC como Alternativa ao REST

Guia completo para adicionar suporte gRPC ao template, rodando lado a lado com o servidor HTTP (Gin).

---

## Índice

1. [Quando usar gRPC vs REST](#1-quando-usar-grpc-vs-rest)
2. [Arquitetura](#2-arquitetura)
3. [Setup de tooling (buf)](#3-setup-de-tooling)
4. [Proto files](#4-proto-files)
5. [Code generation](#5-code-generation)
6. [Server gRPC](#6-server-grpc)
7. [Handlers gRPC](#7-handlers-grpc)
8. [Interceptors](#8-interceptors)
9. [Health check gRPC](#9-health-check-grpc)
10. [Reflection](#10-reflection)
11. [Dual server (HTTP + gRPC)](#11-dual-server)
12. [OpenTelemetry](#12-opentelemetry)
13. [Testes](#13-testes)
14. [Kubernetes](#14-kubernetes)
15. [Makefile](#15-makefile)
16. [gRPC-Gateway (opcional)](#16-grpc-gateway)
17. [Referências](#17-referências)

---

## 1. Quando usar gRPC vs REST

| Aspecto | REST (Gin) | gRPC |
| ------- | ---------- | ---- |
| Protocolo | HTTP/1.1 (JSON) | HTTP/2 (Protobuf) |
| Performance | Bom para a maioria dos casos | ~10x menor latência em serialização, multiplexing |
| Contrato | OpenAPI/Swagger (opcional) | Proto files (obrigatório, type-safe) |
| Streaming | WebSocket, SSE | Bidirecional nativo |
| Browser | Nativo | Precisa de proxy (gRPC-Web ou gRPC-Gateway) |
| Tooling | curl, Postman | grpcurl, grpcui, buf |
| Ecossistema | Universal | Forte em comunicação entre serviços |

**Use REST** para: APIs públicas, integrações com front-end, webhooks, serviços simples.

**Use gRPC** para: comunicação entre microsserviços com alta frequência, streaming (logs, eventos, métricas real-time), contratos fortes entre times, e quando latência importa.

**Na prática**: muitos serviços expõem ambos. REST para consumidores externos, gRPC para comunicação interna. É exatamente o que o dual server permite.

---

## 2. Arquitetura

A Clean Architecture do template torna gRPC uma adição natural. Domain e use cases não mudam — gRPC é apenas mais um adapter na camada de infraestrutura:

```text
                    ┌──────────────────────────────────┐
                    │         Infrastructure            │
                    │                                   │
                    │  ┌─────────┐    ┌──────────────┐  │
                    │  │  web/   │    │    grpc/      │  │
                    │  │  (Gin)  │    │  (gRPC-Go)   │  │
                    │  │ handler │    │   handler     │  │
                    │  │ router  │    │  interceptor  │  │
                    │  │ middle  │    │   server      │  │
                    │  └────┬────┘    └──────┬────────┘  │
                    │       │               │            │
                    │       └───────┬───────┘            │
                    │               │                    │
                    │       ┌───────▼───────┐            │
                    │       │   Use Cases   │            │
                    │       │ (inalterados) │            │
                    │       └───────┬───────┘            │
                    │               │                    │
                    │       ┌───────▼───────┐            │
                    │       │    Domain     │            │
                    │       │ (inalterado)  │            │
                    │       └───────────────┘            │
                    └──────────────────────────────────┘
```

### Estrutura de diretórios

```text
├── cmd/api/
│   └── server.go              # Inicia HTTP + gRPC (dual server)
├── internal/infrastructure/
│   ├── web/                   # REST (já existe)
│   │   ├── handler/
│   │   ├── router/
│   │   └── middleware/
│   └── grpc/                  # gRPC (novo)
│       ├── handler/           # Implementações dos serviços gRPC
│       │   ├── user.go
│       │   └── role.go
│       ├── interceptor/       # Auth, logging, metrics, recovery
│       │   ├── auth.go
│       │   ├── logging.go
│       │   └── recovery.go
│       └── server.go          # Setup do gRPC server + interceptors
├── proto/                     # Proto files
│   └── appmax/
│       ├── user/v1/
│       │   ├── user.proto
│       │   └── user_service.proto
│       ├── role/v1/
│       │   ├── role.proto
│       │   └── role_service.proto
│       └── common/v1/
│           └── pagination.proto
├── gen/proto/                 # Código gerado (gitignored ou commitado)
│   └── appmax/
│       ├── user/v1/
│       │   ├── user.pb.go
│       │   ├── user_service.pb.go
│       │   └── user_service_grpc.pb.go
│       └── ...
├── buf.yaml                   # Configuração buf
└── buf.gen.yaml               # Geração de código
```

---

## 3. Setup de tooling

### Instalar buf

```bash
# macOS
brew install bufbuild/buf/buf

# Linux
curl -sSL https://github.com/bufbuild/buf/releases/latest/download/buf-Linux-x86_64 \
  -o /usr/local/bin/buf && chmod +x /usr/local/bin/buf

# Verificar
buf --version
```

### Instalar plugins de geração

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Garantir que `$GOPATH/bin` está no `PATH`.

### buf.yaml

```yaml
version: v2
modules:
  - path: proto
    name: buf.build/appmax/go-boilerplate
deps:
  - buf.build/googleapis/googleapis
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

### buf.gen.yaml

```yaml
version: v2
clean: true
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: bitbucket.org/appmax-space/go-boilerplate/gen/proto
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen/proto
    opt:
      - paths=source_relative
  - remote: buf.build/grpc/go
    out: gen/proto
    opt:
      - paths=source_relative
      - require_unimplemented_servers=false
inputs:
  - directory: proto
```

**Nota sobre `managed.enabled: true`**: o managed mode configura automaticamente `go_package` e outras opções language-specific. Isso mantém os proto files limpos (sem `option go_package` hardcoded) — o `override` acima define o prefixo para todos os arquivos.

---

## 4. Proto files

### Convenções de naming (buf lint STANDARD)

- Pacote: `organization.domain.version` (ex: `appmax.user.v1`)
- Diretório espelha o pacote: `proto/appmax/user/v1/`
- Arquivos: `snake_case.proto`
- Messages: `PascalCase`
- Fields: `snake_case`
- RPCs: `PascalCase` (verbo + substantivo: `CreateUser`, `GetUser`)
- Enums: `UPPER_SNAKE_CASE` com prefixo do enum name
- Enum zero value: sufixo `_UNSPECIFIED`
- Request/Response: `{MethodName}Request` / `{MethodName}Response`

### proto/appmax/user/v1/user.proto

```protobuf
syntax = "proto3";

package appmax.user.v1;

import "google/protobuf/timestamp.proto";

// User representa um usuário no sistema.
message User {
  string id = 1;
  string name = 2;
  string email = 3;
  google.protobuf.Timestamp created_at = 4;
  google.protobuf.Timestamp updated_at = 5;
}
```

### proto/appmax/user/v1/user_service.proto

```protobuf
syntax = "proto3";

package appmax.user.v1;

import "appmax/user/v1/user.proto";
import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

// UserService define as operações sobre usuários.
service UserService {
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
  rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);
  rpc DeleteUser(DeleteUserRequest) returns (google.protobuf.Empty);
}

// CreateUser

message CreateUserRequest {
  string name = 1;
  string email = 2;
}

message CreateUserResponse {
  string id = 1;
  google.protobuf.Timestamp created_at = 2;
}

// GetUser

message GetUserRequest {
  string id = 1;
}

message GetUserResponse {
  User user = 1;
}

// ListUsers

message ListUsersRequest {
  int32 limit = 1;
  int32 offset = 2;
}

message ListUsersResponse {
  repeated User users = 1;
}

// UpdateUser

message UpdateUserRequest {
  string id = 1;
  string name = 2;
  string email = 3;
}

message UpdateUserResponse {
  User user = 1;
}

// DeleteUser

message DeleteUserRequest {
  string id = 1;
}
```

### proto/appmax/role/v1/role_service.proto

```protobuf
syntax = "proto3";

package appmax.role.v1;

import "google/protobuf/empty.proto";

service RoleService {
  rpc CreateRole(CreateRoleRequest) returns (CreateRoleResponse);
  rpc ListRoles(ListRolesRequest) returns (ListRolesResponse);
  rpc DeleteRole(DeleteRoleRequest) returns (google.protobuf.Empty);
}

message Role {
  string id = 1;
  string name = 2;
  string description = 3;
}

message CreateRoleRequest {
  string name = 1;
  string description = 2;
}

message CreateRoleResponse {
  string id = 1;
}

message ListRolesRequest {
  int32 limit = 1;
  int32 offset = 2;
}

message ListRolesResponse {
  repeated Role roles = 1;
}

message DeleteRoleRequest {
  string id = 1;
}
```

### Lint e breaking change detection

```bash
buf lint                            # Validar proto files
buf breaking --against .git#branch=main  # Detectar breaking changes vs main
```

---

## 5. Code generation

```bash
buf generate
```

Isso gera em `gen/proto/`:
- `*.pb.go` — tipos de mensagem, serialização
- `*_grpc.pb.go` — stubs de client e server, interfaces de serviço

### Commitar ou gitignore?

| Abordagem | Pro | Contra |
| --------- | --- | ------ |
| **Commitar** | CI não precisa de buf/plugins, diff visível em PRs | Código gerado polui o diff |
| **Gitignore** | Repo limpo, single source of truth nos .proto | CI precisa de buf, devs precisam rodar `buf generate` |

**Recomendação**: commitar o código gerado. Simplifica CI e onboarding. Adicione um check no CI que verifica que o gerado está atualizado:

```bash
buf generate && git diff --exit-code gen/
```

---

## 6. Server gRPC

### internal/infrastructure/grpc/server.go

```go
package grpc

import (
    "log/slog"
    "net"

    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/reflection"
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

    "bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/grpc/handler"
    "bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/grpc/interceptor"
)

// Config contém configuração do servidor gRPC.
type Config struct {
    Port              string
    EnableReflection  bool   // true em dev, false em prod
    ServiceKeysEnabled bool
    ServiceKeys       map[string]string
}

// Dependencies agrupa dependências para os handlers gRPC.
type Dependencies struct {
    UserHandler *handler.UserHandler
    RoleHandler *handler.RoleHandler
    Health      *health.Server
}

// NewServer cria e configura o servidor gRPC com interceptors.
func NewServer(cfg Config, deps Dependencies) (*grpc.Server, net.Listener, error) {
    lis, lisErr := net.Listen("tcp", ":"+cfg.Port)
    if lisErr != nil {
        return nil, nil, lisErr
    }

    server := grpc.NewServer(
        // OpenTelemetry via StatsHandler (não interceptor — StatsHandler é o padrão atual)
        grpc.StatsHandler(otelgrpc.NewServerHandler()),

        // Interceptors unary (ordem: metrics → logging → auth → recovery)
        grpc.ChainUnaryInterceptor(
            interceptor.Logging(slog.Default()),
            interceptor.ServiceKeyAuth(cfg.ServiceKeysEnabled, cfg.ServiceKeys),
            interceptor.Recovery(),
        ),

        // Interceptors stream (mesma cadeia)
        grpc.ChainStreamInterceptor(
            interceptor.StreamLogging(slog.Default()),
            interceptor.StreamServiceKeyAuth(cfg.ServiceKeysEnabled, cfg.ServiceKeys),
            interceptor.StreamRecovery(),
        ),
    )

    // Registrar serviços
    deps.UserHandler.Register(server)
    deps.RoleHandler.Register(server)

    // Health check
    healthpb.RegisterHealthServer(server, deps.Health)

    // Reflection (dev/staging — desabilitar em prod se API for pública)
    if cfg.EnableReflection {
        reflection.Register(server)
    }

    return server, lis, nil
}
```

---

## 7. Handlers gRPC

Os handlers gRPC chamam os **mesmos use cases** que os handlers HTTP. A diferença é apenas a tradução proto ↔ DTO.

### internal/infrastructure/grpc/handler/user.go

```go
package handler

import (
    "context"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/emptypb"
    "google.golang.org/protobuf/types/known/timestamppb"

    userpb "bitbucket.org/appmax-space/go-boilerplate/gen/proto/appmax/user/v1"
    userdomain "bitbucket.org/appmax-space/go-boilerplate/internal/domain/user"
    useruc "bitbucket.org/appmax-space/go-boilerplate/internal/usecases/user"
    "bitbucket.org/appmax-space/go-boilerplate/internal/usecases/user/dto"
)

// UserHandler implementa o serviço gRPC de User.
type UserHandler struct {
    CreateUC *useruc.CreateUseCase
    GetUC    *useruc.GetUseCase
    ListUC   *useruc.ListUseCase
    UpdateUC *useruc.UpdateUseCase
    DeleteUC *useruc.DeleteUseCase
}

func NewUserHandler(
    createUC *useruc.CreateUseCase,
    getUC *useruc.GetUseCase,
    listUC *useruc.ListUseCase,
    updateUC *useruc.UpdateUseCase,
    deleteUC *useruc.DeleteUseCase,
) *UserHandler {
    return &UserHandler{
        CreateUC: createUC,
        GetUC:    getUC,
        ListUC:   listUC,
        UpdateUC: updateUC,
        DeleteUC: deleteUC,
    }
}

// Register registra o handler no servidor gRPC.
func (h *UserHandler) Register(server *grpc.Server) {
    userpb.RegisterUserServiceServer(server, h)
}

func (h *UserHandler) CreateUser(ctx context.Context, req *userpb.CreateUserRequest) (*userpb.CreateUserResponse, error) {
    output, execErr := h.CreateUC.Execute(ctx, dto.CreateInput{
        Name:  req.Name,
        Email: req.Email,
    })
    if execErr != nil {
        return nil, toGRPCError(execErr)
    }
    return &userpb.CreateUserResponse{
        Id:        output.ID,
        CreatedAt: mustParseTimestamp(output.CreatedAt),
    }, nil
}

func (h *UserHandler) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
    output, execErr := h.GetUC.Execute(ctx, req.Id)
    if execErr != nil {
        return nil, toGRPCError(execErr)
    }
    return &userpb.GetUserResponse{
        User: userToProto(output),
    }, nil
}

func (h *UserHandler) ListUsers(ctx context.Context, req *userpb.ListUsersRequest) (*userpb.ListUsersResponse, error) {
    output, execErr := h.ListUC.Execute(ctx, dto.ListInput{
        Limit:  int(req.Limit),
        Offset: int(req.Offset),
    })
    if execErr != nil {
        return nil, toGRPCError(execErr)
    }
    users := make([]*userpb.User, len(output))
    for i, u := range output {
        users[i] = userToProto(u)
    }
    return &userpb.ListUsersResponse{Users: users}, nil
}

func (h *UserHandler) UpdateUser(ctx context.Context, req *userpb.UpdateUserRequest) (*userpb.UpdateUserResponse, error) {
    output, execErr := h.UpdateUC.Execute(ctx, dto.UpdateInput{
        ID:    req.Id,
        Name:  req.Name,
        Email: req.Email,
    })
    if execErr != nil {
        return nil, toGRPCError(execErr)
    }
    return &userpb.UpdateUserResponse{
        User: userToProto(output),
    }, nil
}

func (h *UserHandler) DeleteUser(ctx context.Context, req *userpb.DeleteUserRequest) (*emptypb.Empty, error) {
    if execErr := h.DeleteUC.Execute(ctx, req.Id); execErr != nil {
        return nil, toGRPCError(execErr)
    }
    return &emptypb.Empty{}, nil
}

// --- Helpers ---

// toGRPCError converte erros de domínio para gRPC status codes.
// Equivalente ao handler.HandleError() do REST, mas para gRPC.
func toGRPCError(err error) error {
    switch {
    case userdomain.IsNotFound(err):
        return status.Error(codes.NotFound, err.Error())
    case userdomain.IsDuplicateEmail(err):
        return status.Error(codes.AlreadyExists, err.Error())
    case userdomain.IsValidation(err):
        return status.Error(codes.InvalidArgument, err.Error())
    default:
        return status.Error(codes.Internal, "internal error")
    }
}

func userToProto(u *dto.GetOutput) *userpb.User {
    return &userpb.User{
        Id:        u.ID,
        Name:      u.Name,
        Email:     u.Email,
        CreatedAt: mustParseTimestamp(u.CreatedAt),
        UpdatedAt: mustParseTimestamp(u.UpdatedAt),
    }
}

func mustParseTimestamp(rfc3339 string) *timestamppb.Timestamp {
    t, _ := time.Parse(time.RFC3339, rfc3339)
    return timestamppb.New(t)
}
```

**Ponto-chave**: os handlers gRPC são mais finos que os HTTP — sem bind de JSON, sem validação de Content-Type, sem response envelope. O Protobuf cuida da serialização e o proto schema cuida da validação de tipos.

---

## 8. Interceptors

Interceptors são o equivalente gRPC dos middlewares HTTP. Cada um é independente — sem compartilhamento com os middlewares do Gin.

### internal/infrastructure/grpc/interceptor/auth.go

```go
package interceptor

import (
    "context"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
)

// ServiceKeyAuth valida X-Service-Name + X-Service-Key via gRPC metadata.
// Metadata é o equivalente gRPC de HTTP headers.
func ServiceKeyAuth(enabled bool, keys map[string]string) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        // Skip para health check e reflection
        if isPublicMethod(info.FullMethod) {
            return handler(ctx, req)
        }

        if !enabled {
            return handler(ctx, req)
        }

        // Fail-closed: se enabled mas sem keys configuradas
        if len(keys) == 0 {
            return nil, status.Error(codes.Unavailable, "service unavailable")
        }

        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            return nil, status.Error(codes.Unauthenticated, "missing metadata")
        }

        serviceName := firstValue(md, "x-service-name")
        serviceKey := firstValue(md, "x-service-key")

        if serviceName == "" || serviceKey == "" {
            return nil, status.Error(codes.Unauthenticated, "missing service credentials")
        }

        expectedKey, exists := keys[serviceName]
        if !exists || expectedKey != serviceKey {
            return nil, status.Error(codes.PermissionDenied, "invalid service credentials")
        }

        return handler(ctx, req)
    }
}

// StreamServiceKeyAuth é a versão stream do interceptor de auth.
func StreamServiceKeyAuth(enabled bool, keys map[string]string) grpc.StreamServerInterceptor {
    return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
        if isPublicMethod(info.FullMethod) || !enabled {
            return handler(srv, ss)
        }
        if len(keys) == 0 {
            return status.Error(codes.Unavailable, "service unavailable")
        }
        md, ok := metadata.FromIncomingContext(ss.Context())
        if !ok {
            return status.Error(codes.Unauthenticated, "missing metadata")
        }
        serviceName := firstValue(md, "x-service-name")
        serviceKey := firstValue(md, "x-service-key")
        if serviceName == "" || serviceKey == "" {
            return status.Error(codes.Unauthenticated, "missing service credentials")
        }
        expectedKey, exists := keys[serviceName]
        if !exists || expectedKey != serviceKey {
            return status.Error(codes.PermissionDenied, "invalid service credentials")
        }
        return handler(srv, ss)
    }
}

func isPublicMethod(fullMethod string) bool {
    return fullMethod == "/grpc.health.v1.Health/Check" ||
        fullMethod == "/grpc.health.v1.Health/Watch" ||
        fullMethod == "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo" ||
        fullMethod == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo"
}

func firstValue(md metadata.MD, key string) string {
    vals := md.Get(key)
    if len(vals) == 0 {
        return ""
    }
    return vals[0]
}
```

### internal/infrastructure/grpc/interceptor/logging.go

```go
package interceptor

import (
    "context"
    "log/slog"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/status"
)

// Logging registra cada chamada gRPC com duração e status code.
func Logging(logger *slog.Logger) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        start := time.Now()
        resp, err := handler(ctx, req)
        duration := time.Since(start)

        st, _ := status.FromError(err)
        logger.InfoContext(ctx, "grpc request",
            "method", info.FullMethod,
            "code", st.Code().String(),
            "duration_ms", duration.Milliseconds(),
        )
        return resp, err
    }
}

// StreamLogging é a versão stream do interceptor de logging.
func StreamLogging(logger *slog.Logger) grpc.StreamServerInterceptor {
    return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
        start := time.Now()
        err := handler(srv, ss)
        duration := time.Since(start)

        st, _ := status.FromError(err)
        logger.InfoContext(ss.Context(), "grpc stream",
            "method", info.FullMethod,
            "code", st.Code().String(),
            "duration_ms", duration.Milliseconds(),
        )
        return err
    }
}
```

### internal/infrastructure/grpc/interceptor/recovery.go

```go
package interceptor

import (
    "context"
    "log/slog"
    "runtime/debug"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// Recovery captura panics e retorna Internal error.
// Deve ser o último interceptor na chain.
func Recovery() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
        defer func() {
            if r := recover(); r != nil {
                slog.ErrorContext(ctx, "grpc panic recovered",
                    "panic", r,
                    "method", info.FullMethod,
                    "stack", string(debug.Stack()),
                )
                err = status.Error(codes.Internal, "internal error")
            }
        }()
        return handler(ctx, req)
    }
}

// StreamRecovery é a versão stream do interceptor de recovery.
func StreamRecovery() grpc.StreamServerInterceptor {
    return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
        defer func() {
            if r := recover(); r != nil {
                slog.ErrorContext(ss.Context(), "grpc stream panic recovered",
                    "panic", r,
                    "method", info.FullMethod,
                    "stack", string(debug.Stack()),
                )
                err = status.Error(codes.Internal, "internal error")
            }
        }()
        return handler(srv, ss)
    }
}
```

### Ordem dos interceptors

```text
Request → OTel (StatsHandler) → Logging → Auth → Recovery → Handler
                                                                │
Response ← OTel (StatsHandler) ← Logging ← Auth ← Recovery ←──┘
```

- **OTel como StatsHandler** (não interceptor) — garante trace context para todos os interceptors
- **Logging** primeiro — registra todas as chamadas, incluindo as rejeitadas por auth
- **Auth** segundo — rejeita requests não autorizados antes de chegar no handler
- **Recovery** último — captura panics do handler e de interceptors anteriores

---

## 9. Health check gRPC

O protocolo `grpc.health.v1.Health` é o padrão para health checks gRPC, suportado nativamente pelo Kubernetes desde v1.24.

```go
import (
    "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// No server setup
healthServer := health.NewServer()
healthpb.RegisterHealthServer(grpcServer, healthServer)

// Overall server health
healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

// Per-service health
healthServer.SetServingStatus("appmax.user.v1.UserService",
    healthpb.HealthCheckResponse_SERVING)
```

### Monitoramento dinâmico de dependências

Uma goroutine que verifica periodicamente as dependências e atualiza o status:

```go
func monitorHealth(ctx context.Context, healthServer *health.Server, checker *health.Checker) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            healthServer.Shutdown() // marca todos como NOT_SERVING
            return
        case <-ticker.C:
            healthy, _ := checker.RunAll(ctx)
            if healthy {
                healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
            } else {
                healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
            }
        }
    }
}
```

### Testar manualmente

```bash
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
# {"status":"SERVING"}

grpcurl -plaintext -d '{"service":"appmax.user.v1.UserService"}' \
  localhost:50051 grpc.health.v1.Health/Check
```

---

## 10. Reflection

Reflection permite que ferramentas como grpcurl e grpcui descubram os serviços e métodos disponíveis sem ter os proto files.

```go
import "google.golang.org/grpc/reflection"

// Habilitar em dev/staging
if cfg.GRPCReflection {
    reflection.Register(server)
}
```

### Quando habilitar

| Ambiente | Reflection | Motivo |
| -------- | ---------- | ------ |
| Development | Sim | Debug com grpcurl/grpcui |
| Staging/HML | Sim | Debugging operacional |
| Production (interno) | Sim | Serviços internos atrás de service mesh |
| Production (público) | Não | Information disclosure |

### Configuração

```bash
# .env
GRPC_REFLECTION_ENABLED=true  # default: true em dev
```

### Ferramentas

**grpcurl** (CLI — equivalente ao curl para gRPC):

```bash
# Instalar
brew install grpcurl  # ou: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# Listar serviços
grpcurl -plaintext localhost:50051 list

# Listar métodos de um serviço
grpcurl -plaintext localhost:50051 list appmax.user.v1.UserService

# Chamar um método
grpcurl -plaintext \
  -H "x-service-name: myservice" \
  -H "x-service-key: sk_myservice_abc123" \
  -d '{"name": "John", "email": "john@example.com"}' \
  localhost:50051 appmax.user.v1.UserService/CreateUser
```

**grpcui** (Web UI — equivalente ao Swagger UI):

```bash
# Instalar
go install github.com/fullstorydev/grpcui/cmd/grpcui@latest

# Abrir (abre no browser automaticamente)
grpcui -plaintext localhost:50051
```

---

## 11. Dual server

### Wiring no server.go

O `cmd/api/server.go` passa a iniciar **dois servers** em portas separadas:

```go
// Em buildDependencies() ou Start():

// --- gRPC Server ---
grpcHealthServer := health.NewServer()
grpcDeps := grpcpkg.Dependencies{
    UserHandler: grpchandler.NewUserHandler(createUC, getUC, listUC, updateUC, deleteUC),
    RoleHandler: grpchandler.NewRoleHandler(roleCreateUC, roleListUC, roleDeleteUC),
    Health:      grpcHealthServer,
}

grpcServer, grpcLis, grpcErr := grpcpkg.NewServer(grpcpkg.Config{
    Port:              cfg.GRPC.Port,          // ex: "50051"
    EnableReflection:  cfg.GRPC.Reflection,
    ServiceKeysEnabled: cfg.Auth.Enabled,
    ServiceKeys:       middleware.ParseServiceKeys(cfg.Auth.ServiceKeys),
}, grpcDeps)
if grpcErr != nil {
    return grpcErr
}

// --- HTTP Server (Gin) --- (já existe)
httpSrv := newServer(cfg.Server.Port, r)

// --- Start Both ---
g, gCtx := errgroup.WithContext(ctx)

g.Go(func() error {
    slog.Info("Starting gRPC server", "port", cfg.GRPC.Port)
    return grpcServer.Serve(grpcLis)
})

g.Go(func() error {
    slog.Info("Starting HTTP server", "port", cfg.Server.Port)
    if listenErr := httpSrv.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
        return listenErr
    }
    return nil
})

// --- Graceful Shutdown ---
g.Go(func() error {
    <-gCtx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Shutdown em paralelo
    grpcServer.GracefulStop()
    httpSrv.Shutdown(shutdownCtx)
    grpcHealthServer.Shutdown()
    return nil
})

return g.Wait()
```

### Configuração

```bash
# .env
GRPC_PORT=50051
GRPC_REFLECTION_ENABLED=true
```

---

## 12. OpenTelemetry

### Server — StatsHandler (padrão atual)

O StatsHandler substitui os interceptors de OTel (que estão deprecated):

```go
import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

server := grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler()),
)
```

Isso automaticamente:
- Cria spans para cada RPC (unary e stream)
- Propaga trace context entre serviços
- Coleta métricas: `rpc.server.duration`, `rpc.grpc.status_code`
- Registra atributos: `rpc.system`, `rpc.service`, `rpc.method`

### Client — StatsHandler

```go
conn, _ := grpc.NewClient("payment-service:50051",
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
```

### Filtrar health checks

Para evitar spans de health check poluindo os traces:

```go
otelgrpc.NewServerHandler(
    otelgrpc.WithFilter(func(info *stats.RPCTagInfo) bool {
        return info.FullMethodName != "/grpc.health.v1.Health/Check"
    }),
)
```

### Integração com o OTel existente

O `pkg/telemetry` do template já configura TracerProvider e MeterProvider globais. O `otelgrpc.NewServerHandler()` usa os providers globais por default — zero configuração adicional necessária. Os traces gRPC aparecem no mesmo pipeline que os traces HTTP.

---

## 13. Testes

### Unit tests (handler direto)

Testar o handler gRPC sem rede, com mocks dos use cases:

```go
func TestCreateUser(t *testing.T) {
    mockRepo := &mockRepository{
        CreateFunc: func(ctx context.Context, u *userdomain.User) error {
            return nil
        },
    }
    createUC := useruc.NewCreateUseCase(mockRepo)
    handler := grpchandler.NewUserHandler(createUC, nil, nil, nil, nil)

    resp, err := handler.CreateUser(context.Background(), &userpb.CreateUserRequest{
        Name:  "John",
        Email: "john@example.com",
    })

    assert.NoError(t, err)
    assert.NotEmpty(t, resp.Id)
}
```

### Integration tests (bufconn)

Testa o stack gRPC completo (serialização, interceptors, status codes) **sem rede real**:

```go
import "google.golang.org/grpc/test/bufconn"

func setupTestServer(t *testing.T) (userpb.UserServiceClient, func()) {
    t.Helper()
    lis := bufconn.Listen(1024 * 1024)

    server := grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            interceptor.Recovery(),
        ),
    )
    userpb.RegisterUserServiceServer(server, handler)
    go server.Serve(lis)

    conn, _ := grpc.NewClient("passthrough:///bufnet",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return lis.DialContext(ctx)
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    client := userpb.NewUserServiceClient(conn)

    cleanup := func() {
        conn.Close()
        server.Stop()
        lis.Close()
    }
    return client, cleanup
}

func TestCreateUser_Integration(t *testing.T) {
    client, cleanup := setupTestServer(t)
    defer cleanup()

    resp, err := client.CreateUser(context.Background(), &userpb.CreateUserRequest{
        Name:  "John",
        Email: "john@example.com",
    })

    assert.NoError(t, err)
    assert.NotEmpty(t, resp.Id)
}

func TestCreateUser_InvalidEmail(t *testing.T) {
    client, cleanup := setupTestServer(t)
    defer cleanup()

    _, err := client.CreateUser(context.Background(), &userpb.CreateUserRequest{
        Name:  "John",
        Email: "invalid",
    })

    st, ok := status.FromError(err)
    assert.True(t, ok)
    assert.Equal(t, codes.InvalidArgument, st.Code())
}
```

### E2E tests (TestContainers)

Para E2E completo com banco real, use `grpc.NewClient` apontando para o server real (mesmo padrão dos E2E HTTP, mas com client gRPC).

---

## 14. Kubernetes

### Probes gRPC nativas (K8s 1.24+)

Sem necessidade de sidecar ou HTTP proxy para health check:

```yaml
# deploy/base/deployment.yaml
containers:
  - name: api
    ports:
      - containerPort: 8080
        name: http
      - containerPort: 50051
        name: grpc
    livenessProbe:
      grpc:
        port: 50051
      periodSeconds: 10
      failureThreshold: 3
    readinessProbe:
      grpc:
        port: 50051
      periodSeconds: 5
      failureThreshold: 2
```

### Service

```yaml
# deploy/base/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  ports:
    - name: http
      port: 8080
      targetPort: http
    - name: grpc
      port: 50051
      targetPort: grpc
```

### NetworkPolicy

```yaml
# Permitir tráfego gRPC entre serviços internos
ingress:
  - ports:
      - port: 50051
        protocol: TCP
    from:
      - podSelector:
          matchLabels:
            app.kubernetes.io/part-of: appmax
```

---

## 15. Makefile

Targets para adicionar ao Makefile:

```makefile
# --- gRPC ---

.PHONY: proto proto-lint proto-breaking proto-install

## proto: Generate Go code from proto files
proto:
	buf generate

## proto-lint: Lint proto files
proto-lint:
	buf lint

## proto-breaking: Check for breaking changes against main
proto-breaking:
	buf breaking --against '.git#branch=main'

## proto-install: Install protoc plugins
proto-install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

## grpcurl-list: List available gRPC services (requires reflection)
grpcurl-list:
	grpcurl -plaintext localhost:$(GRPC_PORT) list

## grpcui: Open gRPC web UI (requires reflection)
grpcui:
	grpcui -plaintext localhost:$(GRPC_PORT)
```

---

## 16. gRPC-Gateway (opcional)

Se for necessário expor os serviços gRPC como REST **a partir dos proto files** (single source of truth), sem manter handlers Gin separados:

### Como funciona

```text
Client (HTTP/JSON) → gRPC-Gateway (reverse proxy) → gRPC Server
```

O gateway é gerado automaticamente a partir de anotações `google.api.http` nos proto files.

### Quando usar

- gRPC é o transporte **primário** e REST é necessário para compatibilidade
- Quer **uma única definição** (proto) gerando tanto gRPC quanto REST
- Quer eliminar os handlers Gin e usar só proto como contrato

### Quando NÃO usar

- REST (Gin) já é o transporte primário e gRPC é complementar
- Precisa de features específicas do Gin (middleware ecosystem, template rendering)
- A complexidade de anotações HTTP nos protos não compensa

**Para o boilerplate**: como o Gin já existe e funciona bem para REST, gRPC-Gateway provavelmente não é necessário. A recomendação é manter ambos os stacks separados (Gin para REST, gRPC-Go para gRPC) — mais simples de entender e operar.

Se houver interesse futuro, vale pesquisar também o **ConnectRPC** (`connectrpc.com/connect`) — desenvolvido pelo time do buf, suporta gRPC + HTTP/JSON + gRPC-Web com uma única implementação, sem proxy.

---

## 17. Referências

- [gRPC Go Quick Start](https://grpc.io/docs/languages/go/quickstart/)
- [buf.build — Documentação](https://buf.build/docs/)
- [buf.yaml v2](https://buf.build/docs/configuration/v2/buf-yaml/)
- [gRPC Go — Anti-Patterns](https://github.com/grpc/grpc-go/blob/master/Documentation/anti-patterns.md)
- [go-grpc-middleware v2](https://github.com/grpc-ecosystem/go-grpc-middleware)
- [OTel gRPC Instrumentation](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc)
- [gRPC Health Checking Protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md)
- [gRPC-Gateway](https://github.com/grpc-ecosystem/grpc-gateway)
- [ConnectRPC](https://connectrpc.com/)
- [K8s gRPC Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-a-grpc-liveness-probe)
- [bufconn — In-Process Testing](https://pkg.go.dev/google.golang.org/grpc/test/bufconn)
- [Google AIP-191 — Proto File Layout](https://google.aip.dev/191)
