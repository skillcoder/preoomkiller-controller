package pinger

import "errors"

var (
	// ErrPingerNotFound is returned when a pinger is not found
	ErrPingerNotFound = errors.New("pinger not found")

	// ErrPingerAlreadyRegistered is returned when attempting to register a pinger that already exists
	ErrPingerAlreadyRegistered = errors.New("pinger already registered")
)
