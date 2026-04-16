package errorcodes

import "strings"

type ComplaintCodeSet map[string]struct{}

func newComplaintCodeSet(codes ...string) ComplaintCodeSet {
	set := make(ComplaintCodeSet, len(codes))
	for _, code := range codes {
		set[CanonicalComplaintCode(code)] = struct{}{}
	}
	return set
}

func (s ComplaintCodeSet) Has(code string) bool {
	_, ok := s[CanonicalComplaintCode(code)]
	return ok
}

const (
	ComplaintCodeParamError     = "PARAM_ERROR"
	ComplaintCodeInvalidRequest = "INVALID_REQUEST"
	ComplaintCodeSignError      = "SIGN_ERROR"
	ComplaintCodeSystemError    = "SYSTEM_ERROR"
)

func CanonicalComplaintCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

var ComplaintCommonCodes = newComplaintCodeSet(
	ComplaintCodeParamError,
	ComplaintCodeInvalidRequest,
	ComplaintCodeSignError,
	ComplaintCodeSystemError,
)

// 投诉单列表、详情、协商历史接口当前官方页面仅列出公共错误码。
var ComplaintQueryDocumentedCodes = newComplaintCodeSet(
	ComplaintCodeParamError,
	ComplaintCodeInvalidRequest,
	ComplaintCodeSignError,
	ComplaintCodeSystemError,
)

// 投诉通知回调地址 CRUD 当前官方页面仅列出公共错误码。
var ComplaintNotificationConfigDocumentedCodes = newComplaintCodeSet(
	ComplaintCodeParamError,
	ComplaintCodeInvalidRequest,
	ComplaintCodeSignError,
	ComplaintCodeSystemError,
)

// 回复用户、反馈处理完成、更新退款审批结果、即时服务回复当前官方页面仅列出公共错误码。
var ComplaintHandlingDocumentedCodes = newComplaintCodeSet(
	ComplaintCodeParamError,
	ComplaintCodeInvalidRequest,
	ComplaintCodeSignError,
	ComplaintCodeSystemError,
)

// 图片上传、图片请求接口当前官方页面仅列出公共错误码。
var ComplaintImageDocumentedCodes = newComplaintCodeSet(
	ComplaintCodeParamError,
	ComplaintCodeInvalidRequest,
	ComplaintCodeSignError,
	ComplaintCodeSystemError,
)
