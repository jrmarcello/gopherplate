package dto

// =============================================================================
// List Entity DTOs
// =============================================================================

// ListInput representa os dados de entrada para listar entities.
type ListInput struct {
	Page       int    `form:"page"`        // Página atual (1-indexed)
	Limit      int    `form:"limit"`       // Itens por página
	Name       string `form:"name"`        // Filtro por nome
	Email      string `form:"email"`       // Filtro por email
	ActiveOnly bool   `form:"active_only"` // Apenas ativos
}

// ListOutput representa os dados de saída da listagem.
type ListOutput struct {
	Data       []GetOutput      `json:"data"`
	Pagination PaginationOutput `json:"pagination"`
}

// PaginationOutput representa os dados de paginação.
type PaginationOutput struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}
