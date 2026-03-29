package dto

// =============================================================================
// Get User DTOs
// =============================================================================

// GetInput representa os dados de entrada para buscar um user.
type GetInput struct {
	ID string `json:"id"` // UUID v7 do user
}

// GetOutput representa os dados de saída do user encontrado.
type GetOutput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
