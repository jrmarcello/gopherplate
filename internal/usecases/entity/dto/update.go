package dto

// =============================================================================
// Update Entity DTOs
// =============================================================================

// UpdateInput representa os dados de entrada para atualizar uma entity.
type UpdateInput struct {
	ID    string  `json:"-"`               // ID vem da URL
	Name  *string `json:"name,omitempty"`  // Nome (opcional)
	Email *string `json:"email,omitempty"` // Email (opcional)
}

// UpdateOutput representa os dados de saída após atualização.
type UpdateOutput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Active    bool   `json:"active"`
	UpdatedAt string `json:"updated_at"`
}
