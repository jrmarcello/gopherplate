// Semgrep test fixture for usecases.yml — consumed by `semgrep --test .semgrep/`.
// Marker comments:
//   ruleid: <rule-id>   → the next line MUST match the rule
//   ok:     <rule-id>   → the next line MUST NOT match the rule

//go:build semgrep_fixture

package semgrep_fixture_usecases

import (
	user "github.com/jrmarcello/gopherplate/internal/domain/user"
	"github.com/jrmarcello/gopherplate/pkg/apperror"
)

// userToAppError is the canonical mapping function — handlers resolve via
// errors.As on the returned *apperror.AppError.
func userToAppError(err error) *apperror.AppError {
	return apperror.Wrap(err, "internal", "unexpected")
}

// ExecuteOK wraps every error through userToAppError before returning.
// Rule gopherplate-usecase-no-direct-domain-error-bare-return must NOT fire.
func ExecuteOK(id string) error {
	if id == "" {
		// ok: gopherplate-usecase-no-direct-domain-error-bare-return
		return userToAppError(user.ErrDuplicateEmail)
	}
	// ok: gopherplate-usecase-no-direct-domain-error-bare-return
	return userToAppError(user.ErrUserNotFound)
}

// ExecuteBad returns bare domain sentinel errors without wrapping them.
// Rule gopherplate-usecase-no-direct-domain-error-bare-return MUST fire.
func ExecuteBad(id string) (any, error) {
	if id == "" {
		// ruleid: gopherplate-usecase-no-direct-domain-error-bare-return
		return nil, user.ErrDuplicateEmail
	}
	// ruleid: gopherplate-usecase-no-direct-domain-error-bare-return
	return nil, user.ErrUserNotFound
}
