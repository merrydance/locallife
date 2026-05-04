package notification

import (
	"encoding/json"
	"testing"

	"github.com/merrydance/locallife/baofu/account/contracts"
	baofucrypto "github.com/merrydance/locallife/baofu/crypto"
	"github.com/stretchr/testify/require"
)

func TestParserParsesOpenAccountNotification(t *testing.T) {
	codec, err := baofucrypto.NewUnionGWCodec("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	envelope, err := codec.SealEnvelope("102004465", "200005200", map[string]any{
		"member_id":     "102004465",
		"terminal_id":   "200005200",
		"memberType":    "2",
		"state":         "1",
		"errorCode":     "",
		"errorMsg":      "",
		"transSerialNo": "OPEN123",
		"loginNo":       "merchant-login-001",
		"customerName":  "商户A",
		"contractNo":    "CM_SHARE_123",
		"noticeType":    "OPEN_ACC",
	})
	require.NoError(t, err)
	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	parser := NewParser(codec)
	notification, err := parser.ParseOpenAccountNotification(body)

	require.NoError(t, err)
	require.Equal(t, "OPEN123", notification.OutRequestNo)
	require.Equal(t, "CM_SHARE_123", notification.ContractNo)
	require.Empty(t, notification.SharingMerID)
	require.Equal(t, "active", notification.OpenState)
	require.True(t, json.Valid(notification.Raw))
}

func TestParserParsesWithdrawNotification(t *testing.T) {
	codec, err := baofucrypto.NewUnionGWCodec("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	envelope, err := codec.SealEnvelope("102004466", "200005201", map[string]any{
		"contractNo":          "CM202605040001",
		"orderId":             "WD_UP_001",
		"transSerialNo":       "WD202605040001",
		"transMoney":          "123.45",
		"transFee":            "1.00",
		"transferTotalAmount": "124.45",
		"state":               "3",
		"transRemark":         "提现退回",
		"reqReserved":         "withdraw-001",
	})
	require.NoError(t, err)
	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	parser := NewParser(codec)
	notification, err := parser.ParseWithdrawNotification(body)

	require.NoError(t, err)
	require.Equal(t, "WD202605040001", notification.TransSerialNo)
	require.Equal(t, "WD_UP_001", notification.BaofuWithdrawNo)
	require.Equal(t, "3", notification.UpstreamState)
	require.Equal(t, "returned", notification.Status)
	require.True(t, json.Valid(notification.Raw))
}

func TestParserDoesNotFallbackSharingMerIDFromContractNo(t *testing.T) {
	codec, err := baofucrypto.NewUnionGWCodec("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	envelope, err := codec.SealEnvelope("102004465", "200005200", map[string]any{
		"transSerialNo": "OPEN_CONTRACT_ONLY",
		"contractNo":    "CP_ONLY",
		"state":         "1",
	})
	require.NoError(t, err)
	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	parser := NewParser(codec)
	notification, err := parser.ParseOpenAccountNotification(body)

	require.NoError(t, err)
	require.Equal(t, "CP_ONLY", notification.ContractNo)
	require.Empty(t, notification.SharingMerID)
}

func TestParserRejectsMissingCodec(t *testing.T) {
	parser := NewParser(nil)

	_, err := parser.ParseOpenAccountNotification([]byte(`{}`))

	require.EqualError(t, err, "baofu account notification parser is not configured")
}

func TestParseOpenAccountPlaintextUsesOfficialFields(t *testing.T) {
	raw := []byte(`{"member_id":"100000","terminal_id":"200000","memberType":"2","state":"1","errorCode":"","errorMsg":"","transSerialNo":"OPEN202605040001","loginNo":"merchant-login-001","customerName":"商户A","contractNo":"CM202605040001","noticeType":"OPEN_ACC"}`)

	notification, err := ParseOpenAccountPlaintext(raw)

	require.NoError(t, err)
	require.Equal(t, "OPEN202605040001", notification.OutRequestNo)
	require.Equal(t, "CM202605040001", notification.ContractNo)
	require.Equal(t, "1", notification.UpstreamState)
	require.Equal(t, contracts.OpenStateActive, notification.OpenState)
	require.Empty(t, notification.SharingMerID)
	require.JSONEq(t, string(raw), string(notification.Raw))
}

func TestParseWithdrawPlaintextUsesOfficialFields(t *testing.T) {
	raw := []byte(`{"contractNo":"CM202605040001","orderId":"WD_UP_001","transSerialNo":"WD202605040001","transMoney":"123.45","transFee":"1.00","transferTotalAmount":"124.45","state":"3","transRemark":"提现退回","reqReserved":"withdraw-001"}`)

	notification, err := ParseWithdrawPlaintext(raw)

	require.NoError(t, err)
	require.Equal(t, "WD202605040001", notification.TransSerialNo)
	require.Equal(t, "WD_UP_001", notification.BaofuWithdrawNo)
	require.Equal(t, "CM202605040001", notification.ContractNo)
	require.Equal(t, "3", notification.UpstreamState)
	require.Equal(t, "returned", notification.Status)
	require.Equal(t, int64(12345), notification.AmountFen)
	require.Equal(t, int64(100), notification.FeeFen)
	require.Equal(t, int64(12445), notification.TotalAmountFen)
}

func TestAccountNotificationACKIsPlainOK(t *testing.T) {
	require.Equal(t, "OK", AccountNotificationACK())
}
