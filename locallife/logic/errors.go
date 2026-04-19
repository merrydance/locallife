package logic

import "errors"

// RequestError carries an HTTP-style status code for user-facing validation errors.
type RequestError struct {
	Status int
	Err    error
	Cause  error
}

func (e *RequestError) Error() string {
	if e == nil || e.Err == nil {
		return "request error"
	}
	return e.Err.Error()
}

func (e *RequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	if e.Cause != nil {
		return e.Cause
	}
	return e.Err
}

// NewRequestError wraps an error with an HTTP status for API handlers to map.
func NewRequestError(status int, err error) error {
	return &RequestError{Status: status, Err: err}
}

// NewRequestErrorWithCause wraps a public error with an HTTP status while preserving the original cause for logging.
func NewRequestErrorWithCause(status int, publicErr error, cause error) error {
	return &RequestError{Status: status, Err: publicErr, Cause: cause}
}

// LoggableError returns the most useful error to write into logs while preserving the public message for API responses.
func LoggableError(err error) error {
	var reqErr *RequestError
	if errors.As(err, &reqErr) && reqErr != nil {
		if reqErr.Cause != nil {
			return reqErr.Cause
		}
		if reqErr.Err != nil {
			return reqErr.Err
		}
	}
	return err
}

// DeliveryConfirmValidationError carries a machine-readable reason for delivery confirmation validation failures.
type DeliveryConfirmValidationError struct {
	Reason         string
	DistanceMeters int
	RadiusMeters   int
	LocationAgeSec int
	MaxAgeSec      int
	Message        string
}

func (e *DeliveryConfirmValidationError) Error() string {
	if e == nil || e.Message == "" {
		return "delivery confirm validation failed"
	}
	return e.Message
}
