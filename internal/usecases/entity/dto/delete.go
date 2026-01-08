package dto

// =============================================================================
// Delete Entity DTOs
// =============================================================================

// DeleteInput representa os dados de entrada para deletar uma entity.
type DeleteInput struct {
	ID string `json:"id"` // ULID da entity
}

// DeleteOutput representa os dados de saída após deleção.
type DeleteOutput struct {
	ID        string `json:"id"`
	DeletedAt string `json:"deleted_at"` // Timestamp da deleção
}
