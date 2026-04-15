package contracts

const JSAPIPaySignTypeRSA = "RSA"

// JSAPIPayParams is the canonical contract for wx.requestPayment.
//
// Official fields:
// - timeStamp
// - nonceStr
// - package
// - signType
// - paySign
type JSAPIPayParams struct {
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}
