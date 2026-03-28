package logic

import "errors"

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

// ErrRiderRegionUnassigned 表示骑手尚未分配服务区域。
// 使用 errors.Is 匹配，避免字符串比较导致的逻辑脱节。
var ErrRiderRegionUnassigned = errors.New("您尚未分配服务区域，请联系管理员")

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
