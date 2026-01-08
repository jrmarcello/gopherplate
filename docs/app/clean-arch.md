# Arquitetura proposta baseada em Clean Architecture

## Introdução

Você provavelmente já ouviu falar do conceito de **"baixo acoplamento, alta coesão"**, mas raramente é óbvio como alcançá-lo na prática. A boa notícia é que esse é o principal benefício da Arquitetura Limpa (*Clean Architecture*).

Esta arquitetura fornece um guia claro sobre como construir aplicações Go que sejam fáceis de desenvolver, manter e agradáveis de usar a longo prazo, mantendo o foco nas regras de negócio e isolando detalhes de implementação.

---

## Benefícios

Adotamos essa abordagem devido aos benefícios tangíveis que ela entrega ao ciclo de vida do software:

1. **Independência de Frameworks:** A arquitetura não depende da existência de bibliotecas carregadas de funcionalidades ("baterias inclusas"). Isso permite usar frameworks como ferramentas, em vez de forçar seu sistema a se adaptar às limitações deles.
2. **Testabilidade:** As regras de negócio podem ser testadas sem a UI, Banco de Dados, Servidor Web ou qualquer outro elemento externo.
3. **Independência de Interface (UI):** A interface pode mudar facilmente sem alterar o restante do sistema. Uma UI Web poderia ser substituída por uma UI de Console (CLI), por exemplo, sem tocar nas regras de negócio.
4. **Independência de Banco de Dados:** Você pode trocar Oracle ou SQL Server por Mongo, BigTable, CouchDB ou qualquer outro. Suas regras de negócio não estão vinculadas ao banco de dados.
5. **Independência de Agentes Externos:** De fato, suas regras de negócio simplesmente não sabem nada sobre o mundo exterior.

### Outras Vantagens Notáveis

- **Padronização:** Uma estrutura de diretórios clara torna fácil para novos desenvolvedores se orientarem no projeto.
- **Velocidade a Longo Prazo:** Embora o setup inicial seja maior, o desenvolvimento e manutenção tornam-se mais rápidos à medida que o projeto cresce.
- **Testes Unitários Triviais:** A injeção de dependência torna a simulação (mocking) de componentes extremamente simples.
- **Evolução Flexível:** Facilidade na transição de protótipos para soluções robustas (ex: começar com repositório em memória e migrar para SQL sem afetar o core da aplicação).

---

## Desenvolvimento e Conceitos

Nossa abordagem para a Arquitetura Limpa combina conceitos fundamentais como **Portas e Adaptadores** (Arquitetura Hexagonal) e **Inversão de Dependência**. A regra de ouro é: **dependências de código fonte devem apontar apenas para dentro**, em direção às políticas de alto nível.

- **Entidades (Domain):** O núcleo da aplicação. Contém regras de negócio corporativas que não mudam frequentemente.
- **Casos de Uso (Usecase):** Orquestram o fluxo de dados para e das entidades.
- **Adaptadores de Interface (Infrastructure/Web):** Convertem dados do formato mais conveniente para os casos de uso e entidades para o formato mais conveniente para agentes externos (Web, DB).

---

## Conclusão

Ao seguir estes princípios, garantimos um software robusto, onde decisões de infraestrutura (qual banco usar, qual framework web usar) podem ser postergadas ou alteradas com mínimo impacto. O resultado é um sistema onde a lógica de negócio é a protagonista absoluta.
