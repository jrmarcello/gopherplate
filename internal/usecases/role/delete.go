package role

import (
	"context"
	"time"

	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/role/dto"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/role/interfaces"
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
	// Validar e converter ID
	id, parseErr := vo.ParseID(input.ID)
	if parseErr != nil {
		return nil, parseErr
	}

	// Deletar role
	if deleteErr := uc.Repo.Delete(ctx, id); deleteErr != nil {
		return nil, deleteErr
	}

	return &dto.DeleteOutput{
		ID:        input.ID,
		DeletedAt: time.Now().Format(time.RFC3339),
	}, nil
}
