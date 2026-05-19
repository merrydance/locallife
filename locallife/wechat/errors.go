package wechat

import "fmt"

var ErrCode2SessionMissingOpenID = fmt.Errorf("wechat code2session missing openid")

// APIError represents an error response returned by WeChat OpenAPI.
// It is typically caused by invalid/expired client parameters (e.g. js_code).
type APIError struct {
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("wechat api error: code=%d, msg=%s", e.Code, e.Msg)
}
