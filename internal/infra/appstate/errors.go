package appstate

import "errors"

var (
	// ErrInvalidStateTransition is returned when attempting an invalid state transition
	ErrInvalidStateTransition = errors.New("invalid state transition")

	// ErrAlreadyTerminated is returned when attempting to change state after termination
	ErrAlreadyTerminated = errors.New("application already terminated")
)
