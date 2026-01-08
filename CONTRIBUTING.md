# Guia de Contribuição

Obrigado pelo interesse em contribuir com o **MS Boilerplate Go**!

## 🚀 Como começar

1. Faça um **Fork** do projeto.
2. Clone o seu fork: `git clone https://bitbucket.org/SEU_USUARIO/ms-boilerplate-go`.
3. Crie uma branch para sus feature ou fix: `git checkout -b feat/minha-feature`.

## 🛠️ Desenvolvimento

Certifique-se de ter as ferramentas instaladas (`Go 1.24`, `Docker`, `Make`).

```bash
# Setup inicial
make setup

# Rodar testes
make test

# Verificar lint
make lint-full
```

## 📝 Commits

Seguimos o padrão **Conventional Commits**:

- `feat:` Nova funcionalidade
- `fix:` Correção de bug
- `docs:` Documentação
- `chore:` Configurações, dependências
- `refactor:` Mudança de código que não altera funcionalidade
- `test:` Adição ou correção de testes

Exemplo: `feat(api): add new endpoint for user profile`

## ✅ Pull Requests

Ao abrir um PR:

1. Descreva claramente o que foi feito.
2. Garanta que os testes passaram.
3. Garanta que o Lint passou.
4. Se mudou algo na API, atualize o Swagger (`make setup` instala o swag, depois rode `swag init -g cmd/api/main.go`).

## 🧪 Testes

Novas funcionalidades devem vir acompanhadas de:

- Testes unitários (`internal/domain`, `internal/usecases`)
- Testes E2E se crítico (`tests/e2e`)

Bom código! 🚀
