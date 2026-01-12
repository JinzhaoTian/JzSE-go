// Package errors defines common error types for the JzSE system.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common error conditions.
var (
	// Storage errors
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrStorageFull   = errors.New("storage capacity exceeded")

	// Sync errors
	ErrSyncFailed  = errors.New("sync operation failed")
	ErrSyncTimeout = errors.New("sync timeout")
	ErrConflict    = errors.New("conflict detected")
	ErrQueueFull   = errors.New("sync queue full")

	// Metadata errors
	ErrInvalidMetadata = errors.New("invalid metadata")
	ErrVersionMismatch = errors.New("version mismatch")

	// Region errors
	ErrRegionOffline     = errors.New("region is offline")
	ErrRegionUnavailable = errors.New("region unavailable")

	// Coordinator errors
	ErrCoordinatorUnavailable = errors.New("coordinator unavailable")
	ErrNoHealthyRegion        = errors.New("no healthy region available")

	// Permission errors
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")

	// Validation errors
	ErrInvalidInput = errors.New("invalid input")
)

// JzSEError is a custom error type with additional context.
type JzSEError struct {
	Op      string // Operation that failed
	Kind    error  // Category of error
	Err     error  // Underlying error
	Details string // Additional details
}

// Error implements the error interface.
func (e *JzSEError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s: %s (%s)", e.Op, e.Kind, e.Err, e.Details)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Kind, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Kind)
}

// Unwrap returns the underlying error.
func (e *JzSEError) Unwrap() error {
	return e.Err
}

// Is reports whether target matches this error.
func (e *JzSEError) Is(target error) bool {
	return errors.Is(e.Kind, target) || errors.Is(e.Err, target)
}

// E creates a new JzSEError.
func E(op string, kind error, err error, details ...string) error {
	e := &JzSEError{
		Op:   op,
		Kind: kind,
		Err:  err,
	}
	if len(details) > 0 {
		e.Details = details[0]
	}
	return e
}

// Wrap wraps an error with operation context.
func Wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return &JzSEError{
		Op:  op,
		Err: err,
	}
}

// IsNotFound checks if the error is a not found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsConflict checks if the error is a conflict error.
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}

// IsUnauthorized checks if the error is an unauthorized error.
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}
