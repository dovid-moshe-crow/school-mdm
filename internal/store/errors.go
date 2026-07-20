package store

import "errors"

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("not found")

// ErrInsufficientCredits is returned when a spend would make balance negative.
var ErrInsufficientCredits = errors.New("insufficient credits")
