package user

import (
	"context"
	"fmt"
	"math"
	"time"

	userdomain "github.com/jrmarcello/go-boilerplate/internal/domain/user"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/dto"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/interfaces"

	ucshared "github.com/jrmarcello/go-boilerplate/internal/usecases/shared"
	"go.opentelemetry.io/otel/trace"
)

// ListUseCase implementa o caso de uso de listar users.
type ListUseCase struct {
	Repo interfaces.Repository
}

// NewListUseCase cria uma nova instância do ListUseCase.
func NewListUseCase(repo interfaces.Repository) *ListUseCase {
	return &ListUseCase{Repo: repo}
}

// Execute retorna uma lista paginada de users.
func (uc *ListUseCase) Execute(ctx context.Context, input dto.ListInput) (*dto.ListOutput, error) {
	span := trace.SpanFromContext(ctx)

	// Converter input para filtro de domínio
	filter := userdomain.ListFilter{
		Page:       input.Page,
		Limit:      input.Limit,
		Name:       input.Name,
		Email:      input.Email,
		ActiveOnly: input.ActiveOnly,
	}

	// Buscar no repositório
	result, listErr := uc.Repo.List(ctx, filter)
	if listErr != nil {
		wrappedErr := fmt.Errorf("listing users: %w", listErr)
		ucshared.ClassifyError(span, listErr, nil, wrappedErr.Error())
		return nil, userToAppError(listErr)
	}

	// Converter para DTOs de saída
	items := make([]dto.GetOutput, 0, len(result.Users))
	for _, e := range result.Users {
		items = append(items, dto.GetOutput{
			ID:        e.ID.String(),
			Name:      e.Name,
			Email:     e.Email.String(),
			Active:    e.Active,
			CreatedAt: e.CreatedAt.Format(time.RFC3339),
			UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
		})
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(result.Total) / float64(result.Limit)))

	return &dto.ListOutput{
		Data: items,
		Pagination: dto.PaginationOutput{
			Page:       result.Page,
			Limit:      result.Limit,
			Total:      result.Total,
			TotalPages: totalPages,
		},
	}, nil
}
