package wechat

import "fmt"

// APIError represents an error response returned by WeChat OpenAPI.
// It is typically caused by invalid/expired client parameters (e.g. js_code).
type APIError struct {
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("wechat api error: code=%d, msg=%s", e.Code, e.Msg)
}
