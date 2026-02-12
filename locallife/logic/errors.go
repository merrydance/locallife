package logic

// RequestError carries an HTTP-style status code for user-facing validation errors.
type RequestError struct {
	Status int
	Err    error
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
	return e.Err
}

// NewRequestError wraps an error with an HTTP status for API handlers to map.
func NewRequestError(status int, err error) error {
	return &RequestError{Status: status, Err: err}
}
