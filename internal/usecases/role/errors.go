package role

import (
	"errors"

	roledomain "github.com/jrmarcello/gopherplate/internal/domain/role"
	"github.com/jrmarcello/gopherplate/pkg/apperror"
)

// Expected errors per use case — used by ClassifyError to distinguish
// domain errors (expected) from infra errors (unexpected).
var (
	createExpectedErrors = []error{roledomain.ErrDuplicateRoleName}
	deleteExpectedErrors = []error{roledomain.ErrRoleNotFound}
	// listExpectedErrors is intentionally nil — list only produces infra errors.
)

// roleToAppError maps domain errors to structured AppError codes.
// Unknown/infra errors are wrapped with CodeInternalError.
func roleToAppError(err error) *apperror.AppError {
	switch {
	case errors.Is(err, roledomain.ErrRoleNotFound):
		return apperror.Wrap(err, apperror.CodeNotFound, "role not found")
	case errors.Is(err, roledomain.ErrDuplicateRoleName):
		return apperror.Wrap(err, apperror.CodeConflict, "role name already exists")
	default:
		return apperror.Wrap(err, apperror.CodeInternalError, "internal server error")
	}
}
