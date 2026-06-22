package witherrors

import "errors"

// ErrFixtureNotFound is returned when a fixture is not found.
var ErrFixtureNotFound = errors.New("fixture not found")

// ErrFixtureConflict is returned when a fixture conflicts.
var ErrFixtureConflict = errors.New("fixture conflict")
