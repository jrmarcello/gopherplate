package user

import (
	"errors"

	userdomain "github.com/jrmarcello/go-boilerplate/internal/domain/user"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/jrmarcello/go-boilerplate/pkg/apperror"
)

// expectedErrors per use case — used by ClassifyError to distinguish
// domain/validation errors (expected) from infrastructure errors (unexpected).
var (
	createExpectedErrors = []error{vo.ErrInvalidEmail, userdomain.ErrDuplicateEmail}
	getExpectedErrors    = []error{vo.ErrInvalidID, userdomain.ErrUserNotFound}
	updateExpectedErrors = []error{vo.ErrInvalidID, vo.ErrInvalidEmail, userdomain.ErrUserNotFound}
	deleteExpectedErrors = []error{vo.ErrInvalidID, userdomain.ErrUserNotFound}
	// list has no expected errors — only infra errors are possible.
)

// userToAppError maps domain/validation errors to structured AppError.
// This is the single source of truth for user error translation in the use case layer.
func userToAppError(err error) *apperror.AppError {
	switch {
	case errors.Is(err, vo.ErrInvalidEmail):
		return apperror.Wrap(err, apperror.CodeInvalidRequest, "invalid email")
	case errors.Is(err, vo.ErrInvalidID):
		return apperror.Wrap(err, apperror.CodeInvalidRequest, "invalid ID")
	case errors.Is(err, userdomain.ErrUserNotFound):
		return apperror.Wrap(err, apperror.CodeNotFound, "user not found")
	case errors.Is(err, userdomain.ErrDuplicateEmail):
		return apperror.Wrap(err, apperror.CodeConflict, "email already exists")
	default:
		return apperror.Wrap(err, apperror.CodeInternalError, "internal server error")
	}
}
