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
	require.Equal(t, "102004465", notification.MemberID)
	require.Equal(t, "200005200", notification.TerminalID)
	require.Equal(t, "CM_SHARE_123", notification.ContractNo)
	require.Empty(t, notification.SharingMerID)
	require.Equal(t, "active", notification.OpenState)
	require.True(t, json.Valid(notification.Raw))
}

func TestParserParsesOpenAccountNotificationWithCamelCasePlaintextIdentity(t *testing.T) {
	privatePEM, publicPEM := generateNotificationTestKeyPair(t)
	plaintext := []byte(`{"memberId":"102004465","terminalId":"200005200","memberType":"1","state":"1","errorCode":"","errorMsg":"","transSerialNo":"OPEN123","loginNo":"person002","customerName":"张宝","contractNo":"CP690000000000001468","noticeType":"OPEN_ACC"}`)
	body := officialNotificationQueryForTest(t, privatePEM, plaintext)

	parser := NewParser(publicPEM)
	notification, err := parser.ParseOpenAccountNotification(body)

	require.NoError(t, err)
	require.Equal(t, "102004465", notification.MemberID)
	require.Equal(t, "200005200", notification.TerminalID)
	require.Equal(t, "OPEN123", notification.OutRequestNo)
	require.Equal(t, "CP690000000000001468", notification.ContractNo)
	require.Equal(t, "active", notification.OpenState)
}

func TestParserRejectsOpenAccountNotificationTransportIdentityMismatch(t *testing.T) {
	privatePEM, publicPEM := generateNotificationTestKeyPair(t)
	plaintext := []byte(`{"member_id":"102004465","terminal_id":"200005200","memberType":"2","state":"1","errorCode":"","errorMsg":"","transSerialNo":"OPEN123","loginNo":"merchant-login-001","customerName":"商户A","contractNo":"CM_SHARE_123","noticeType":"OPEN_ACC"}`)
	content, err := baofu.EncodeUnionGWVerifyType1Content(privatePEM, plaintext)
	require.NoError(t, err)
	values := url.Values{}
	values.Set("member_id", "102004466")
	values.Set("terminal_id", "200005200")
	values.Set("data_type", "JSON")
	values.Set("data_content", content)

	parser := NewParser(publicPEM)
	_, err = parser.ParseOpenAccountNotification([]byte(values.Encode()))

	require.EqualError(t, err, "baofu open account notification member_id does not match transport")
}

func TestParserParsesOfficialWithdrawNotification(t *testing.T) {
	privatePEM, publicPEM := generateNotificationTestKeyPair(t)
	plaintext := []byte(`{"contractNo":"CM202605040001","orderId":"WD_UP_001","transSerialNo":"WD202605040001","transMoney":"123.45","transFee":"1.00","transferTotalAmount":"124.45","state":"3","transRemark":"提现退回","reqReserved":"withdraw-001"}`)
	body := officialNotificationQueryForTest(t, privatePEM, plaintext)

	parser := NewParser(publicPEM)
	notification, err := parser.ParseWithdrawNotification(body)

	require.NoError(t, err)
	require.Equal(t, "102004465", notification.MemberID)
	require.Equal(t, "200005200", notification.TerminalID)
	require.Equal(t, "WD202605040001", notification.TransSerialNo)
	require.Equal(t, "WD_UP_001", notification.BaofuWithdrawNo)
	require.Equal(t, "3", notification.UpstreamState)
	require.Equal(t, "returned", notification.Status)
	require.True(t, json.Valid(notification.Raw))
}

func TestParserDoesNotFallbackSharingMerIDFromContractNo(t *testing.T) {
	privatePEM, publicPEM := generateNotificationTestKeyPair(t)
	body := officialNotificationQueryForTest(t, privatePEM, []byte(`{"member_id":"102004465","terminal_id":"200005200","memberType":"2","state":"1","transSerialNo":"OPEN_CONTRACT_ONLY","loginNo":"merchant-login-001","customerName":"商户A","contractNo":"CP_ONLY","noticeType":"OPEN_ACC"}`))

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

func TestParserRejectsMissingOfficialTransportFields(t *testing.T) {
	privatePEM, publicPEM := generateNotificationTestKeyPair(t)
	plaintext := []byte(`{"member_id":"102004465","terminal_id":"200005200","memberType":"2","state":"1","transSerialNo":"OPEN123","loginNo":"merchant-login-001","customerName":"商户A","contractNo":"CM_SHARE_123","noticeType":"OPEN_ACC"}`)
	content, err := baofu.EncodeUnionGWVerifyType1Content(privatePEM, plaintext)
	require.NoError(t, err)
	parser := NewParser(publicPEM)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"missing member", "terminal_id=200005200&data_type=JSON&data_content=" + url.QueryEscape(content), "baofu account notification member_id is required"},
		{"missing terminal", "member_id=102004465&data_type=JSON&data_content=" + url.QueryEscape(content), "baofu account notification terminal_id is required"},
		{"missing data type", "member_id=102004465&terminal_id=200005200&data_content=" + url.QueryEscape(content), "baofu account notification data_type is required"},
		{"unsupported data type", "member_id=102004465&terminal_id=200005200&data_type=XML&data_content=" + url.QueryEscape(content), "baofu account notification data_type must be JSON"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parser.ParseOpenAccountNotification([]byte(tc.body))
			require.EqualError(t, err, tc.want)
		})
	}
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

func TestParseOpenAccountPlaintextAllowsMissingNoticeTypeFromOfficialExamples(t *testing.T) {
	raw := []byte(`{"contractNo":"CP690000000000001468","customerName":"张宝","errorCode":"","errorMsg":"","loginNo":"person002","memberId":"100030218","memberType":"1","state":"1","terminalId":"200005478","transSerialNo":"TSN314778753119603185643720"}`)

	notification, err := ParseOpenAccountPlaintext(raw)

	require.NoError(t, err)
	require.Equal(t, "TSN314778753119603185643720", notification.OutRequestNo)
	require.Equal(t, "CP690000000000001468", notification.ContractNo)
	require.Equal(t, contracts.OpenStateActive, notification.OpenState)
}

func TestParseOpenAccountPlaintextRejectsMissingMandatoryFieldsAndUnsupportedState(t *testing.T) {
	base := map[string]string{
		"member_id":     "102004465",
		"terminal_id":   "200005200",
		"memberType":    "2",
		"state":         "1",
		"transSerialNo": "OPEN202605040001",
		"loginNo":       "merchant-login-001",
		"customerName":  "商户A",
		"contractNo":    "CM202605040001",
		"noticeType":    "OPEN_ACC",
	}
	cases := []struct {
		name   string
		mutate func(map[string]string)
		want   string
	}{
		{"missing member", func(p map[string]string) { delete(p, "member_id") }, "baofu open account notification member_id is required"},
		{"missing terminal", func(p map[string]string) { delete(p, "terminal_id") }, "baofu open account notification terminal_id is required"},
		{"missing member type", func(p map[string]string) { delete(p, "memberType") }, "baofu open account notification memberType is required"},
		{"missing state", func(p map[string]string) { delete(p, "state") }, "baofu open account notification state is required"},
		{"unsupported state", func(p map[string]string) { p["state"] = "9" }, "baofu open account notification state is unsupported"},
		{"missing trans serial", func(p map[string]string) { delete(p, "transSerialNo") }, "baofu open account notification transSerialNo is required"},
		{"missing login", func(p map[string]string) { delete(p, "loginNo") }, "baofu open account notification loginNo is required"},
		{"missing customer name", func(p map[string]string) { delete(p, "customerName") }, "baofu open account notification customerName is required"},
		{"missing contract", func(p map[string]string) { delete(p, "contractNo") }, "baofu open account notification contractNo is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := make(map[string]string, len(base))
			for k, v := range base {
				payload[k] = v
			}
			tc.mutate(payload)
			raw, err := json.Marshal(payload)
			require.NoError(t, err)

			_, err = ParseOpenAccountPlaintext(raw)
			require.EqualError(t, err, tc.want)
		})
	}
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

func TestParseWithdrawPlaintextRejectsMissingMandatoryFieldsAndUnsupportedState(t *testing.T) {
	base := map[string]string{
		"contractNo":          "CM202605040001",
		"orderId":             "WD_UP_001",
		"transSerialNo":       "WD202605040001",
		"transMoney":          "123.45",
		"transFee":            "1.00",
		"transferTotalAmount": "124.45",
		"state":               "3",
		"transRemark":         "提现退回",
		"reqReserved":         "withdraw-001",
	}
	cases := []struct {
		name   string
		mutate func(map[string]string)
		want   string
	}{
		{"missing contract", func(p map[string]string) { delete(p, "contractNo") }, "baofu withdraw notification contractNo is required"},
		{"missing order id", func(p map[string]string) { delete(p, "orderId") }, "baofu withdraw notification orderId is required"},
		{"missing trans serial", func(p map[string]string) { delete(p, "transSerialNo") }, "baofu withdraw notification transSerialNo is required"},
		{"missing trans money", func(p map[string]string) { delete(p, "transMoney") }, "baofu withdraw notification transMoney is required"},
		{"missing trans fee", func(p map[string]string) { delete(p, "transFee") }, "baofu withdraw notification transFee is required"},
		{"missing total amount", func(p map[string]string) { delete(p, "transferTotalAmount") }, "baofu withdraw notification transferTotalAmount is required"},
		{"missing state", func(p map[string]string) { delete(p, "state") }, "baofu withdraw notification state is required"},
		{"unsupported state", func(p map[string]string) { p["state"] = "9" }, "baofu withdraw notification state is unsupported"},
		{"missing remark", func(p map[string]string) { delete(p, "transRemark") }, "baofu withdraw notification transRemark is required"},
		{"missing reserved", func(p map[string]string) { delete(p, "reqReserved") }, "baofu withdraw notification reqReserved is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := make(map[string]string, len(base))
			for k, v := range base {
				payload[k] = v
			}
			tc.mutate(payload)
			raw, err := json.Marshal(payload)
			require.NoError(t, err)

			_, err = ParseWithdrawPlaintext(raw)
			require.EqualError(t, err, tc.want)
		})
	}
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
