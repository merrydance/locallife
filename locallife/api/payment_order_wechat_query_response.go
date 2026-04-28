package api

import (
	"github.com/merrydance/locallife/logic"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

type paymentOrderWechatQueryResult struct {
	AppID           string                              `json:"appid,omitempty"`
	MchID           string                              `json:"mchid,omitempty"`
	SpAppID         string                              `json:"sp_appid"`
	SpMchID         string                              `json:"sp_mchid"`
	SubAppID        string                              `json:"sub_appid,omitempty"`
	SubMchID        string                              `json:"sub_mchid"`
	OutTradeNo      string                              `json:"out_trade_no"`
	TransactionID   string                              `json:"transaction_id,omitempty"`
	TradeType       string                              `json:"trade_type,omitempty"`
	TradeState      string                              `json:"trade_state"`
	TradeStateDesc  string                              `json:"trade_state_desc"`
	BankType        string                              `json:"bank_type,omitempty"`
	Attach          string                              `json:"attach,omitempty"`
	SuccessTime     string                              `json:"success_time,omitempty"`
	Payer           *paymentOrderWechatPayerResult      `json:"payer,omitempty"`
	Amount          *paymentOrderWechatAmountResult     `json:"amount,omitempty"`
	SceneInfo       *paymentOrderWechatSceneInfo        `json:"scene_info,omitempty"`
	PromotionDetail []paymentOrderWechatPromotionDetail `json:"promotion_detail,omitempty"`
}

type paymentOrderWechatPayerResult struct {
	OpenID    string `json:"openid,omitempty"`
	SpOpenID  string `json:"sp_openid,omitempty"`
	SubOpenID string `json:"sub_openid,omitempty"`
}

type paymentOrderWechatAmountResult struct {
	Total         int64  `json:"total"`
	PayerTotal    int64  `json:"payer_total"`
	Currency      string `json:"currency,omitempty"`
	PayerCurrency string `json:"payer_currency,omitempty"`
}

type paymentOrderWechatSceneInfo struct {
	DeviceID string `json:"device_id,omitempty"`
}

type paymentOrderWechatPromotionDetail struct {
	CouponID            string                                   `json:"coupon_id"`
	Name                string                                   `json:"name,omitempty"`
	Scope               string                                   `json:"scope,omitempty"`
	Type                string                                   `json:"type,omitempty"`
	Amount              int64                                    `json:"amount"`
	StockID             string                                   `json:"stock_id,omitempty"`
	WechatpayContribute int64                                    `json:"wechatpay_contribute,omitempty"`
	MerchantContribute  int64                                    `json:"merchant_contribute,omitempty"`
	OtherContribute     int64                                    `json:"other_contribute,omitempty"`
	Currency            string                                   `json:"currency,omitempty"`
	GoodsDetail         []paymentOrderWechatPromotionGoodsDetail `json:"goods_detail,omitempty"`
}

type paymentOrderWechatPromotionGoodsDetail struct {
	GoodsID        string `json:"goods_id"`
	Quantity       int64  `json:"quantity"`
	UnitPrice      int64  `json:"unit_price"`
	DiscountAmount int64  `json:"discount_amount"`
	GoodsRemark    string `json:"goods_remark,omitempty"`
}

func newPaymentOrderWechatQueryResult(query *logic.QueryPaymentOrderWechatOrder) *paymentOrderWechatQueryResult {
	if query == nil {
		return nil
	}

	resp := &paymentOrderWechatQueryResult{
		AppID:          query.AppID,
		MchID:          query.MchID,
		SpAppID:        query.SpAppID,
		SpMchID:        query.SpMchID,
		SubAppID:       query.SubAppID,
		SubMchID:       query.SubMchID,
		OutTradeNo:     query.OutTradeNo,
		TransactionID:  query.TransactionID,
		TradeType:      query.TradeType,
		TradeState:     query.TradeState,
		TradeStateDesc: query.TradeStateDesc,
		BankType:       query.BankType,
		Attach:         query.Attach,
		SuccessTime:    query.SuccessTime,
	}

	if query.Payer.OpenID != "" || query.Payer.SpOpenID != "" || query.Payer.SubOpenID != "" {
		resp.Payer = &paymentOrderWechatPayerResult{
			OpenID:    query.Payer.OpenID,
			SpOpenID:  query.Payer.SpOpenID,
			SubOpenID: query.Payer.SubOpenID,
		}
	}
	if query.Amount.Total != 0 || query.Amount.PayerTotal != 0 || query.Amount.Currency != "" || query.Amount.PayerCurrency != "" {
		resp.Amount = &paymentOrderWechatAmountResult{
			Total:         query.Amount.Total,
			PayerTotal:    query.Amount.PayerTotal,
			Currency:      query.Amount.Currency,
			PayerCurrency: query.Amount.PayerCurrency,
		}
	}
	if query.SceneDeviceID != "" {
		resp.SceneInfo = &paymentOrderWechatSceneInfo{DeviceID: query.SceneDeviceID}
	}
	resp.PromotionDetail = newWechatPromotionDetails(query.PromotionDetail)

	return resp
}

func newWechatPromotionDetails(details []wechatcontracts.PartnerPromotionDetail) []paymentOrderWechatPromotionDetail {
	if len(details) == 0 {
		return nil
	}

	result := make([]paymentOrderWechatPromotionDetail, 0, len(details))
	for _, promotion := range details {
		item := paymentOrderWechatPromotionDetail{
			CouponID:            promotion.CouponID,
			Name:                promotion.Name,
			Scope:               promotion.Scope,
			Type:                promotion.Type,
			Amount:              promotion.Amount,
			StockID:             promotion.StockID,
			WechatpayContribute: promotion.WechatpayContribute,
			MerchantContribute:  promotion.MerchantContribute,
			OtherContribute:     promotion.OtherContribute,
			Currency:            promotion.Currency,
		}
		if len(promotion.GoodsDetail) > 0 {
			item.GoodsDetail = make([]paymentOrderWechatPromotionGoodsDetail, 0, len(promotion.GoodsDetail))
			for _, goods := range promotion.GoodsDetail {
				item.GoodsDetail = append(item.GoodsDetail, paymentOrderWechatPromotionGoodsDetail{
					GoodsID:        goods.GoodsID,
					Quantity:       goods.Quantity,
					UnitPrice:      goods.UnitPrice,
					DiscountAmount: goods.DiscountAmount,
					GoodsRemark:    goods.GoodsRemark,
				})
			}
		}
		result = append(result, item)
	}

	return result
}
