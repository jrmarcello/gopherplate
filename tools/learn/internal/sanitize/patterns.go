package sanitize

import "github.com/jrmarcello/gopherplate/tools/learn/internal/config"

// BuiltinPatterns returns the canonical default secret patterns compiled
// from config.DefaultSecretPatterns. It is a thin convenience wrapper that
// avoids duplicating regex strings in this package — the single source of
// truth lives in the config layer (REQ-4).
//
// An error is returned only if one of the default patterns fails to
// compile, which would be a programming error in the config package.
func BuiltinPatterns() ([]Pattern, error) {
	s, err := NewFromConfig(config.DefaultSecretPatterns)
	if err != nil {
		return nil, err
	}
	out := make([]Pattern, len(s.patterns))
	copy(out, s.patterns)
	return out, nil
}
