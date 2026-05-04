package notification

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/url"
	"testing"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/baofu/account/contracts"
	"github.com/stretchr/testify/require"
)

func TestParserParsesOfficialOpenAccountNotification(t *testing.T) {
	privatePEM, publicPEM := generateNotificationTestKeyPair(t)
	plaintext := []byte(`{"member_id":"102004465","terminal_id":"200005200","memberType":"2","state":"1","errorCode":"","errorMsg":"","transSerialNo":"OPEN123","loginNo":"merchant-login-001","customerName":"商户A","contractNo":"CM_SHARE_123","noticeType":"OPEN_ACC"}`)
	body := officialNotificationQueryForTest(t, privatePEM, plaintext)

	parser := NewParser(publicPEM)
	notification, err := parser.ParseOpenAccountNotification(body)

	require.NoError(t, err)
	require.Equal(t, "OPEN123", notification.OutRequestNo)
	require.Equal(t, "CM_SHARE_123", notification.ContractNo)
	require.Empty(t, notification.SharingMerID)
	require.Equal(t, "active", notification.OpenState)
	require.True(t, json.Valid(notification.Raw))
}

func TestParserParsesOfficialWithdrawNotification(t *testing.T) {
	privatePEM, publicPEM := generateNotificationTestKeyPair(t)
	plaintext := []byte(`{"contractNo":"CM202605040001","orderId":"WD_UP_001","transSerialNo":"WD202605040001","transMoney":"123.45","transFee":"1.00","transferTotalAmount":"124.45","state":"3","transRemark":"提现退回","reqReserved":"withdraw-001"}`)
	body := officialNotificationQueryForTest(t, privatePEM, plaintext)

	parser := NewParser(publicPEM)
	notification, err := parser.ParseWithdrawNotification(body)

	require.NoError(t, err)
	require.Equal(t, "WD202605040001", notification.TransSerialNo)
	require.Equal(t, "WD_UP_001", notification.BaofuWithdrawNo)
	require.Equal(t, "3", notification.UpstreamState)
	require.Equal(t, "returned", notification.Status)
	require.True(t, json.Valid(notification.Raw))
}

func TestParserDoesNotFallbackSharingMerIDFromContractNo(t *testing.T) {
	privatePEM, publicPEM := generateNotificationTestKeyPair(t)
	body := officialNotificationQueryForTest(t, privatePEM, []byte(`{"transSerialNo":"OPEN_CONTRACT_ONLY","contractNo":"CP_ONLY","state":"1"}`))

	parser := NewParser(publicPEM)
	notification, err := parser.ParseOpenAccountNotification(body)

	require.NoError(t, err)
	require.Equal(t, "CP_ONLY", notification.ContractNo)
	require.Empty(t, notification.SharingMerID)
}

func TestParserRejectsMissingPublicKey(t *testing.T) {
	parser := NewParser("")

	_, err := parser.ParseOpenAccountNotification([]byte(`{}`))

	require.EqualError(t, err, "baofu account notification parser is not configured")
}

func TestParserRejectsMissingDataContent(t *testing.T) {
	_, publicPEM := generateNotificationTestKeyPair(t)
	parser := NewParser(publicPEM)

	_, err := parser.ParseOpenAccountNotification([]byte(`member_id=102004465&terminal_id=200005200&data_type=JSON`))

	require.EqualError(t, err, "baofu account notification data_content is required")
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

func officialNotificationQueryForTest(t *testing.T, privatePEM string, plaintext []byte) []byte {
	t.Helper()
	content, err := baofu.EncodeUnionGWVerifyType1Content(privatePEM, plaintext)
	require.NoError(t, err)
	values := url.Values{}
	values.Set("member_id", "102004465")
	values.Set("terminal_id", "200005200")
	values.Set("data_type", "JSON")
	values.Set("data_content", content)
	return []byte(values.Encode())
}

func generateNotificationTestKeyPair(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})

	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})

	return string(privatePEM), string(publicPEM)
}
