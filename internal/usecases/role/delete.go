package role

import (
	"context"
	"fmt"
	"time"

	"github.com/jrmarcello/gopherplate/internal/domain/user/vo"
	"github.com/jrmarcello/gopherplate/internal/usecases/role/dto"
	"github.com/jrmarcello/gopherplate/internal/usecases/role/interfaces"
	ucshared "github.com/jrmarcello/gopherplate/internal/usecases/shared"
	"github.com/jrmarcello/gopherplate/pkg/apperror"
	"go.opentelemetry.io/otel/trace"
)

// DeleteUseCase implementa o caso de uso de delecao de role.
type DeleteUseCase struct {
	Repo interfaces.Repository
}

// NewDeleteUseCase cria uma nova instancia do DeleteUseCase.
func NewDeleteUseCase(repo interfaces.Repository) *DeleteUseCase {
	return &DeleteUseCase{
		Repo: repo,
	}
}

// Execute realiza a delecao de uma role.
//
// Fluxo:
//  1. Validar ID
//  2. Deletar role
func (uc *DeleteUseCase) Execute(ctx context.Context, input dto.DeleteInput) (*dto.DeleteOutput, error) {
	span := trace.SpanFromContext(ctx)

	// Validar e converter ID
	id, parseErr := vo.ParseID(input.ID)
	if parseErr != nil {
		ucshared.ClassifyError(span, parseErr, deleteExpectedErrors, "deleting role")
		return nil, apperror.New(apperror.CodeInvalidRequest, "invalid role ID")
	}

	// Deletar role
	if deleteErr := uc.Repo.Delete(ctx, id); deleteErr != nil {
		wrappedErr := fmt.Errorf("deleting role: %w", deleteErr)
		ucshared.ClassifyError(span, deleteErr, deleteExpectedErrors, "deleting role")
		return nil, roleToAppError(wrappedErr)
	}

	return &dto.DeleteOutput{
		ID:        input.ID,
		DeletedAt: time.Now().Format(time.RFC3339),
	}, nil
}
