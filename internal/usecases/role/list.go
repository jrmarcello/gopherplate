package role

import (
	"context"
	"fmt"
	"math"
	"time"

	roledomain "github.com/jrmarcello/gopherplate/internal/domain/role"
	"github.com/jrmarcello/gopherplate/internal/usecases/role/dto"
	"github.com/jrmarcello/gopherplate/internal/usecases/role/interfaces"
	ucshared "github.com/jrmarcello/gopherplate/internal/usecases/shared"
	"go.opentelemetry.io/otel/trace"
)

// ListUseCase implementa o caso de uso de listar roles.
type ListUseCase struct {
	Repo interfaces.Repository
}

// NewListUseCase cria uma nova instancia do ListUseCase.
func NewListUseCase(repo interfaces.Repository) *ListUseCase {
	return &ListUseCase{Repo: repo}
}

// Execute retorna uma lista paginada de roles.
func (uc *ListUseCase) Execute(ctx context.Context, input dto.ListInput) (*dto.ListOutput, error) {
	span := trace.SpanFromContext(ctx)

	// Converter input para filtro de dominio
	filter := roledomain.ListFilter{
		Page:  input.Page,
		Limit: input.Limit,
		Name:  input.Name,
	}

	// Buscar no repositorio
	result, listErr := uc.Repo.List(ctx, filter)
	if listErr != nil {
		wrappedErr := fmt.Errorf("listing roles: %w", listErr)
		ucshared.ClassifyError(span, wrappedErr, nil, "listing roles")
		return nil, roleToAppError(wrappedErr)
	}

	// Converter para DTOs de saida
	items := make([]dto.RoleOutput, 0, len(result.Roles))
	for _, r := range result.Roles {
		items = append(items, dto.RoleOutput{
			ID:          r.ID.String(),
			Name:        r.Name,
			Description: r.Description,
			CreatedAt:   r.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
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
