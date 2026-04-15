package dto

// =============================================================================
// Delete User DTOs
// =============================================================================

// DeleteInput representa os dados de entrada para deletar um user.
type DeleteInput struct {
	ID string `json:"id"` // UUID v7 do user
}

// DeleteOutput representa os dados de saída após deleção.
type DeleteOutput struct {
	ID string `json:"id"`
}
