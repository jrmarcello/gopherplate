package user

import "errors"

// Erros de domínio para User.
var (
	ErrUserNotFound   = errors.New("user not found")
	ErrDuplicateEmail = errors.New("email already exists")
)
