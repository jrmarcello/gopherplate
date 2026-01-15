# Decisão de Arquitetura: Estratégia de Configuração

## Contexto

A aplicação precisa ser configurável em múltiplos ambientes: **Desenvolvimento Local**, **Infraestrutura Docker** e **Produção (Kubernetes)**. Precisamos de uma estratégia unificada que evite duplicidade e mantenha a conformidade com o 12-Factor App.

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

## Justificativa

1. **Single Source of Truth (Local)**: O arquivo `.env` na raiz é consumido simultaneamente pelo Docker Compose, Go Application e Makefile.
2. **Transparência em Produção**: O K8s injeta configurações via Env Vars, que têm precedência máxima.
3. **Simplicidade (DX)**: O desenvolvedor precisa apenas criar um arquivo `.env`.
4. **Leveza**: Sem dependências pesadas como Viper (~10 dependências transitivas).

## Consequências

- **Positivas**:
  - Eliminamos arquivos duplicados (`docker/.env`, `config.yaml`).
  - `make dev` e `make docker-up` funcionam em harmonia.
  - Comportamento determinístico em produção.
  - Binário menor (~3-5MB a menos).

- **Negativas**:
  - Sem suporte nativo a múltiplos formatos (YAML, TOML, JSON).
  - Sem hot reload de configuração.

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
        },
        DB: DBConfig{
            DSN: getEnv("DB_DSN", "postgres://..."),
        },
        // ...
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

| Struct Field | Env Var | Default |
| ------------ | ------- | ------- |
| `Server.Port` | `SERVER_PORT` | `8080` |
| `DB.DSN` | `DB_DSN` | `postgres://...` |
| `Redis.Enabled` | `REDIS_ENABLED` | `false` |
