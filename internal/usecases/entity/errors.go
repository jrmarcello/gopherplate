package entity

import "errors"

// Erros específicos da camada de use cases.
var (
	// ErrInvalidInput indica que os dados de entrada são inválidos.
	ErrInvalidInput = errors.New("invalid input")
)
