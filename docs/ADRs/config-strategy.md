# Decisão de Arquitetura: Estratégia de Configuração (Viper + .env)

## Contexto

A aplicação precisa ser configurável em múltiplos ambientes: **Desenvolvimento Local**, **Infraestrutura Docker** e **Produção (Kubernetes)**. Precisamos de uma estratégia unificada que evite duplicidade e mantenha a conformidade com o 12-factor App.

## Decisão

Adotamos uma abordagem híbrida utilizando **Viper** com prioridade para **Variáveis de Ambiente**, centralizando a configuração local em um único arquivo `.env`.

### Estrutura de Prioridade (Viper)

1. 🥇 **Variáveis de Ambiente**: Precedência máxima. Injetadas pelo Kubernetes (ConfigMaps/Secrets) ou Docker Compose.
2. 🥈 **Arquivo .env**: Fonte da verdade para desenvolvimento local.
3. 🥉 **Defaults no Código**: Fallback seguro (ex: `localhost`) para garantir que o app rode mesmo sem configuração explícita em dev.

## Justificativa

1. **Single Source of Truth (Local)**:
    - O arquivo `.env` na raiz é consumido simultaneamente pelo:
        - **Docker Compose**: Para subir infra (Postgres, Redis).
        - **Go Application**: Via Viper (modo `SetConfigType("env")`).
        - **Makefile**: Para injetar variáveis nos comandos.
    - Eliminamos a necessidade de arquivos duplicados (`docker/.env`, `config.yaml`).

2. **Transparência em Produção (K8s)**:
    - O Viper mapeia automaticamente env vars (ex: `SERVER_PORT` -> `server.port`).
    - O K8s injeta configurações exclusivamente via Variáveis de Ambiente.
    - Como Env Vars têm prioridade máxima, o comportamento em produção é determinístico e desacoplado de arquivos locais.

3. **Simplicidade (DX)**:
    - O desenvolvedor precisa apenas criar um arquivo: `.env`.
    - `make dev` e `make docker-up` funcionam em harmonia.

## Implementação

- **Biblioteca**: `github.com/spf13/viper`
- **Mapping**: `v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))`
- **Arquivo**: `v.SetConfigFile(".env")` (Ignorado se não existir, sem quebrar a app).
