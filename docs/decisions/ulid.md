# Decisão de Arquitetura: Uso de ULID

## Contexto

A escolha do formato de Identificador Único é crítica para sistemas distribuídos e bancos de dados. Consideramos UUID v4, UUID v7 e ULID.

## Decisão

Utilizar **ULID (Universally Unique Lexicographically Sortable Identifier)** para chaves primárias de entidades.

## Justificativa

1. **Ordenação Lexicográfica**: ULIDs são ordenáveis por tempo. Isso melhora significativamente a performance de inserção em índices B-Tree (como chave primária no Postgres), reduzindo a fragmentação de páginas comparado a UUIDs v4 totalmente aleatórios.
2. **Legibilidade**: Codificados em Base32 (Crockford's), são mais curtos e URL-safe (ex: `01ARZ3NDEKTSV4RRFFQ69G5FAV`) comparados à representação hex de UUIDs.
3. **Compatibilidade**: São 128-bit compatíveis com UUID. Podem ser armazenados em colunas `UUID` no Postgres sem perda de performance.
4. **Precisão de Tempo**: Contém um timestamp de milissegundos, permitindo saber quando o registro foi criado apenas olhando para o ID, sem precisar de uma coluna extra de data (embora mantenhamos `created_at` para explícita auditoria).

## Comparação

| Característica | UUID v4 | UUID v7 | ULID |
|---|---|---|---|
| Ordenável | ❌ Não | ✅ Sim (por tempo) | ✅ Sim (por tempo) |
| Colisão | Extremamente Rara | Extremamente Rara | Extremamente Rara |
| Tamanho (String) | 36 chars | 36 chars | 26 chars |
| Indexação DB | Ruim (fragmentação) | Ótima (sequencial) | Ótima (sequencial) |
| URL Safe | ❌ Não (hifens) | ❌ Não (hifens) | ✅ Sim |

## Implementação

Utilizamos a biblioteca `github.com/oklog/ulid/v2`.
