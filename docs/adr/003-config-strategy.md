# ADR-003: Config Strategy

**Status**: Aceito  
**Data**: 2026-01-16  
**Autor**: Marcelo Jr

---

## Contexto

A aplicação precisa ser configurável em múltiplos ambientes: **Desenvolvimento Local**, **Infraestrutura Docker** e **Produção (Kubernetes)**. Precisamos de uma estratégia unificada que evite duplicidade e mantenha a conformidade com o 12-Factor App.

---

## Decisão

Adotamos **godotenv + pacote nativo `os`** como estratégia de configuração com prioridade para **Variáveis de Ambiente**, centralizando a configuração local em um único arquivo `.env`.

> [!NOTE]
> Anteriormente usávamos Viper, mas migramos para uma solução mais leve já que só precisávamos de leitura de `.env` + env vars.

### Hierarquia de Prioridade

| Prioridade | Fonte | Uso |
| ---------- | ----- | --- |
| 🥇 Alta | Variáveis de Ambiente | Kubernetes (ConfigMaps/Secrets), Docker Compose |
| 🥈 Média | Arquivo `.env` | Desenvolvimento local |
| 🥉 Baixa | Defaults no Código | Fallback seguro (`localhost`) |

---

## Alternativas Consideradas

| Estratégia | Veredicto | Motivo |
| ---------- | --------- | ------ |
| Viper (spf13) | ❌ Rejeitado | Pesado (muitas deps transitivas), overkill apenas para envs |
| Flags (cli) | ❌ Rejeitado | Verborragia no deploy (k8s args ficariam enormes) |
| **Godotenv + os** | ✅ **Escolhido** | Simples, leve e segue 12-factor app nativamente |

---

## Justificativa

1. **Single Source of Truth (Local)**: O arquivo `.env` na raiz é consumido simultaneamente pelo Docker Compose, Go Application e Makefile.
2. **Transparência em Produção**: O K8s injeta configurações via Env Vars, que têm precedência máxima.
3. **Simplicidade (DX)**: O desenvolvedor precisa apenas criar um arquivo `.env`.
4. **Leveza**: Sem dependências pesadas como Viper (~10 dependências transitivas).

---

## Consequências

### Positivas

- Eliminamos arquivos duplicados (`docker/.env`, `config.yaml`).
- `make dev` e `make docker-up` funcionam em harmonia.
- Comportamento determinístico em produção.
- Binário menor (~3-5MB a menos).

### Negativas

- Sem suporte nativo a múltiplos formatos (YAML, TOML, JSON).
- Sem hot reload de configuração.

---

## Implementação

### Configuração com godotenv

```go
// config/config.go
func Load() (*Config, error) {
    // 1. Carrega .env (opcional)
    _ = godotenv.Load()

    // 2. Lê variáveis de ambiente com fallback
    return &Config{
        Server: ServerConfig{
            Port: getEnv("SERVER_PORT", "8080"),
            Env:  getEnv("APP_ENV", "development"),
        },
        DB: DBConfig{
            Host:     getEnv("DB_HOST", "localhost"),
            Port:     getEnv("DB_PORT", "5432"),
            User:     getEnv("DB_USER", "user"),
            Password: getEnv("DB_PASSWORD", "password"),
            Name:     getEnv("DB_NAME", "users"),
            SSLMode:  getEnv("DB_SSLMODE", "disable"),
            // Pool
            MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
            MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
            ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
            ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 90*time.Second),
            // Replica
            ReplicaEnabled: getEnvBool("DB_REPLICA_ENABLED", false),
            ReplicaHost:    os.Getenv("DB_REPLICA_HOST"),
            // ...
        },
        Redis: RedisConfig{
            URL:          getEnv("REDIS_URL", "redis://localhost:6379"),
            TTL:          getEnv("REDIS_TTL", "5m"),
            Enabled:      getEnvBool("REDIS_ENABLED", false),
            PoolSize:     getEnvInt("REDIS_POOL_SIZE", 30),
            MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 5),
            DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 500ms),
            ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 200ms),
            WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 200ms),
        },
    }, nil
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

### Mapeamento de Variáveis

#### Servidor

| Struct Field | Env Var | Default |
| ------------ | ------- | ------- |
| `Server.Port` | `SERVER_PORT` | `8080` |
| `Server.Env` | `APP_ENV` | `development` |

#### Banco de Dados — Writer (primary)

| Struct Field | Env Var | Default |
| ------------ | ------- | ------- |
| `DB.Host` | `DB_HOST` | `localhost` |
| `DB.Port` | `DB_PORT` | `5432` |
| `DB.User` | `DB_USER` | `user` |
| `DB.Password` | `DB_PASSWORD` | `password` |
| `DB.Name` | `DB_NAME` | `users` |
| `DB.SSLMode` | `DB_SSLMODE` | `disable` |

#### Banco de Dados — Pool de Conexões

| Struct Field | Env Var | Default |
| ------------ | ------- | ------- |
| `DB.MaxOpenConns` | `DB_MAX_OPEN_CONNS` | `25` |
| `DB.MaxIdleConns` | `DB_MAX_IDLE_CONNS` | `10` |
| `DB.ConnMaxLifetime` | `DB_CONN_MAX_LIFETIME` | `5m` |
| `DB.ConnMaxIdleTime` | `DB_CONN_MAX_IDLE_TIME` | `90s` |

#### Banco de Dados — Replica (read)

Variáveis de replica fazem fallback para os valores do writer quando não definidas.

| Struct Field | Env Var | Default |
| ------------ | ------- | ------- |
| `DB.ReplicaEnabled` | `DB_REPLICA_ENABLED` | `false` |
| `DB.ReplicaHost` | `DB_REPLICA_HOST` | *(fallback: `DB_HOST`)* |
| `DB.ReplicaPort` | `DB_REPLICA_PORT` | *(fallback: `DB_PORT`)* |
| `DB.ReplicaUser` | `DB_REPLICA_USER` | *(fallback: `DB_USER`)* |
| `DB.ReplicaPassword` | `DB_REPLICA_PASSWORD` | *(fallback: `DB_PASSWORD`)* |
| `DB.ReplicaName` | `DB_REPLICA_NAME` | *(fallback: `DB_NAME`)* |
| `DB.ReplicaSSLMode` | `DB_REPLICA_SSLMODE` | *(fallback: `DB_SSLMODE`)* |
| `DB.ReplicaMaxOpenConns` | `DB_REPLICA_MAX_OPEN_CONNS` | `40` |
| `DB.ReplicaMaxIdleConns` | `DB_REPLICA_MAX_IDLE_CONNS` | `20` |
| `DB.ReplicaConnMaxLifetime` | `DB_REPLICA_CONN_MAX_LIFETIME` | `5m` |
| `DB.ReplicaConnMaxIdleTime` | `DB_REPLICA_CONN_MAX_IDLE_TIME` | `90s` |

#### Redis

| Struct Field | Env Var | Default |
| ------------ | ------- | ------- |
| `Redis.Enabled` | `REDIS_ENABLED` | `false` |
| `Redis.URL` | `REDIS_URL` | `redis://localhost:6379` |
| `Redis.TTL` | `REDIS_TTL` | `5m` |
| `Redis.PoolSize` | `REDIS_POOL_SIZE` | `30` |
| `Redis.MinIdleConns` | `REDIS_MIN_IDLE_CONNS` | `5` |
| `Redis.DialTimeout` | `REDIS_DIAL_TIMEOUT` | `500ms` |
| `Redis.ReadTimeout` | `REDIS_READ_TIMEOUT` | `200ms` |
| `Redis.WriteTimeout` | `REDIS_WRITE_TIMEOUT` | `200ms` |

#### Outros

| Struct Field | Env Var | Default |
| ------------ | ------- | ------- |
| `Otel.ServiceName` | `OTEL_SERVICE_NAME` | `gopherplate` |
| `Otel.CollectorURL` | `OTEL_COLLECTOR_URL` | *(vazio)* |
| `Otel.Insecure` | `OTEL_INSECURE` | `true` |
| `Auth.Enabled` | `SERVICE_KEYS_ENABLED` | `false` |
| `Auth.ServiceKeys` | `SERVICE_KEYS` | *(vazio)* |
| `Swagger.Enabled` | `SWAGGER_ENABLED` | `false` |
| `Swagger.Host` | `SWAGGER_HOST` | *(vazio — fallback: localhost:PORT)* |

### Exemplo de ConfigMap (Kubernetes)

```yaml
# deploy/overlays/develop/configmap.yaml (dados não sensíveis)
apiVersion: v1
kind: ConfigMap
metadata:
  name: gopherplate-config
data:
  APP_ENV: "development"
  SERVER_PORT: "8080"
  DB_HOST: "postgres-service"
  DB_PORT: "5432"
  DB_NAME: "users"
  DB_SSLMODE: "disable"
  DB_MAX_OPEN_CONNS: "25"
  DB_MAX_IDLE_CONNS: "10"
  DB_CONN_MAX_LIFETIME: "5m"
  DB_CONN_MAX_IDLE_TIME: "90s"
  DB_REPLICA_ENABLED: "false"
  OTEL_SERVICE_NAME: "gopherplate"
  REDIS_URL: "redis://redis-service:6379"
  REDIS_TTL: "5m"
  REDIS_ENABLED: "true"
  SERVICE_KEYS_ENABLED: "true"    # fail-closed: sem keys no Secret = 503
```

```yaml
# deploy/overlays/develop/secret.yaml (credenciais — em HML/PRD via ExternalSecret)
apiVersion: v1
kind: Secret
metadata:
  name: gopherplate-secrets
stringData:
  DB_USER: "user"
  DB_PASSWORD: "password"
  SERVICE_KEYS: "dev-service:sk_dev_service_key_12345"
```
