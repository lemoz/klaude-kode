package provider

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrorCodeInvalidModel          ErrorCode = "invalid_model"
	ErrorCodeCapabilityMismatch    ErrorCode = "capability_mismatch"
	ErrorCodeAuthUnavailable       ErrorCode = "auth_unavailable"
	ErrorCodeAuthFailed            ErrorCode = "auth_failed"
	ErrorCodeProviderRequestFailed ErrorCode = "provider_request_failed"
)

type Error struct {
	Code      ErrorCode
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("provider error: %s", e.Code)
}

func AsError(err error) *Error {
	var providerErr *Error
	if errors.As(err, &providerErr) {
		return providerErr
	}
	return nil
}
