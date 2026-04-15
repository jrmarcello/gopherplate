package dto

// =============================================================================
// Delete Role DTOs
// =============================================================================

// DeleteInput representa os dados de entrada para deletar uma role.
type DeleteInput struct {
	ID string `json:"id"` // UUID v7 da role
}

// DeleteOutput representa os dados de saida apos delecao.
type DeleteOutput struct {
	ID string `json:"id"`
}
