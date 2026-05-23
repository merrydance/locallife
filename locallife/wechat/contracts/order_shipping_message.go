package contracts

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
)

// OrderShippingEventType is the Mini Program message-push event emitted by
// WeChat's order shipping management service when an order is shipped or
// settled.
const OrderShippingEventType = "trade_manage_order_settlement"

// OrderShippingSettlementMessage is the Mini Program message-push payload for
// trade_manage_order_settlement. Numeric time fields are Unix timestamps in
// seconds as defined by the Mini Program message-push contract.
type OrderShippingSettlementMessage struct {
	XMLName                 xml.Name `xml:"xml" json:"-"`
	ToUserName              string   `xml:"ToUserName" json:"ToUserName"`
	FromUserName            string   `xml:"FromUserName" json:"FromUserName"`
	CreateTime              int64    `xml:"CreateTime" json:"CreateTime"`
	MsgType                 string   `xml:"MsgType" json:"MsgType"`
	Event                   string   `xml:"Event" json:"Event"`
	TransactionID           string   `xml:"transaction_id" json:"transaction_id"`
	MerchantID              string   `xml:"merchant_id" json:"merchant_id"`
	SubMerchantID           string   `xml:"sub_merchant_id" json:"sub_merchant_id"`
	MerchantTradeNo         string   `xml:"merchant_trade_no" json:"merchant_trade_no"`
	PayTime                 int64    `xml:"pay_time" json:"pay_time"`
	ShippedTime             int64    `xml:"shipped_time" json:"shipped_time"`
	EstimatedSettlementTime int64    `xml:"estimated_settlement_time" json:"estimated_settlement_time"`
	ConfirmReceiveMethod    int      `xml:"confirm_receive_method" json:"confirm_receive_method"`
	ConfirmReceiveTime      int64    `xml:"confirm_receive_time" json:"confirm_receive_time"`
	SettlementTime          int64    `xml:"settlement_time" json:"settlement_time"`
}

func (m OrderShippingSettlementMessage) IsSettlementEvent() bool {
	return m.Event == OrderShippingEventType
}

func (m OrderShippingSettlementMessage) IsSettled() bool {
	return m.SettlementTime > 0 || m.ConfirmReceiveTime > 0
}

func ParseOrderShippingSettlementMessage(body []byte) (OrderShippingSettlementMessage, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return OrderShippingSettlementMessage{}, fmt.Errorf("empty order shipping settlement message")
	}

	switch trimmed[0] {
	case '{':
		return parseOrderShippingSettlementJSON(trimmed)
	case '<':
		return parseOrderShippingSettlementXML(trimmed)
	default:
		message, jsonErr := parseOrderShippingSettlementJSON(trimmed)
		if jsonErr == nil {
			return message, nil
		}
		message, xmlErr := parseOrderShippingSettlementXML(trimmed)
		if xmlErr == nil {
			return message, nil
		}
		return OrderShippingSettlementMessage{}, fmt.Errorf("parse order shipping settlement message as JSON: %w; as XML: %w", jsonErr, xmlErr)
	}
}

func parseOrderShippingSettlementJSON(body []byte) (OrderShippingSettlementMessage, error) {
	var message OrderShippingSettlementMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return OrderShippingSettlementMessage{}, err
	}
	return message, nil
}

func parseOrderShippingSettlementXML(body []byte) (OrderShippingSettlementMessage, error) {
	var message OrderShippingSettlementMessage
	if err := xml.Unmarshal(body, &message); err != nil {
		return OrderShippingSettlementMessage{}, err
	}
	return message, nil
}
