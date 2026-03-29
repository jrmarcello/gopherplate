package dto

// =============================================================================
// Update User DTOs
// =============================================================================

// UpdateInput representa os dados de entrada para atualizar um user.
type UpdateInput struct {
	ID    string  `json:"-"`                                                 // ID vem da URL
	Name  *string `json:"name,omitempty" binding:"omitempty,max=255"`        // Nome (opcional)
	Email *string `json:"email,omitempty" binding:"omitempty,email,max=255"` // Email (opcional)
}

// UpdateOutput representa os dados de saída após atualização.
type UpdateOutput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Active    bool   `json:"active"`
	UpdatedAt string `json:"updated_at"`
}
