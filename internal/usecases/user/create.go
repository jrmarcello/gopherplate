package user

import (
	"context"
	"fmt"
	"time"

	userdomain "github.com/jrmarcello/go-boilerplate/internal/domain/user"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/dto"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/interfaces"

	ucshared "github.com/jrmarcello/go-boilerplate/internal/usecases/shared"
	"go.opentelemetry.io/otel/trace"
)

// CreateUseCase implementa o caso de uso de criação de user.
type CreateUseCase struct {
	Repo interfaces.Repository
}

// NewCreateUseCase cria uma nova instância do CreateUseCase.
func NewCreateUseCase(repo interfaces.Repository) *CreateUseCase {
	return &CreateUseCase{Repo: repo}
}

// Execute executa o caso de uso de criação de user.
//
// Fluxo:
//  1. Converte primitivos (string) para Value Objects (validação acontece aqui)
//  2. Cria a entidade User usando a Factory
//  3. Persiste no banco via Repository
//  4. Retorna DTO com ID e timestamp
func (uc *CreateUseCase) Execute(ctx context.Context, input dto.CreateInput) (*dto.CreateOutput, error) {
	span := trace.SpanFromContext(ctx)

	// PASSO 1: Converter primitivos para Value Objects
	emailVO, emailErr := vo.NewEmail(input.Email)
	if emailErr != nil {
		ucshared.ClassifyError(span, emailErr, createExpectedErrors, "creating user: invalid email")
		return nil, userToAppError(emailErr)
	}

	// PASSO 2: Criar Entidade usando a Factory
	e := userdomain.NewUser(input.Name, emailVO)

	// PASSO 3: Persistir no banco via Repository
	if saveErr := uc.Repo.Create(ctx, e); saveErr != nil {
		wrappedErr := fmt.Errorf("creating user: %w", saveErr)
		ucshared.ClassifyError(span, saveErr, createExpectedErrors, wrappedErr.Error())
		return nil, userToAppError(saveErr)
	}

	// PASSO 4: Retornar Output DTO
	return &dto.CreateOutput{
		ID:        e.ID.String(),
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
	}, nil
}
