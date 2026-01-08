# Roadmap do Projeto

Um boilerplate para ser usado como base para criar microservices em Go.

---

## 1. Estrutura de Pastas (Refinando)

...

---

## O que essa arquitetura entrega?

...

## Lista de coisa para add ao projeto

- [x] air
- [x] Make
- [x] Lefthook
- [x] ULID
- [x] Clean Arch minimamente implementada (dominios ricos isolando a regra de negocio, VOs, DTOs)
- [x] Deploy Ready com Docker, hot reload e health checks incluídos
- [x] SQLx otimizado
- [x] Migrações - Sistema de migrações com Goose
- [x] Configuração Flexível - Viper com suporte a arquivos e variáveis de ambiente
- [x] Logging Inteligente - Sistema avançado com correlação automática de traces
- [x] Observabilidade Completa - OpenTelemetry integrado (traces, métricas, logs)
- [x] Gin para performance e praticidade de (Middleware Ecosystem: Idempotency, CORS, Rate Limiting, Cahche, etc...)
- [x] Testes unitarios rigorosos para a camada de dominio e testes e2e pontuais
- [x] openapi docs de api
- [x] tratamento de erros avançado

- [x] Deixar main.go com inicialização mínima (clean)

- [x] Remover arquivos
- [x] Revisar cmd/api/doc.go (Podemos add em server.go ou main.go?)
- [x] Revisar config/config.go
- [x] Revisar config/config_test.go
- [x] Revisar docker
- [x] Revisar arquivos swagger (não estamos em licença MIT)
- [x] Add nas docs um arquivo explicando o por que de usar ulid e suas vantagens em relação a uuid
- [x] Abilitar o endpoint de listagem (eu avia desabilitado)
- [ ]

- [x] Add instrucoes sobre como contribuir e propor melhorias no projeto
