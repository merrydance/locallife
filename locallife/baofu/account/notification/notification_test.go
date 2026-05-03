package notification

import (
	"encoding/json"
	"testing"

	baofucrypto "github.com/merrydance/locallife/baofu/crypto"
	"github.com/stretchr/testify/require"
)

func TestParserParsesOpenAccountNotification(t *testing.T) {
	codec, err := baofucrypto.NewUnionGWCodec("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	envelope, err := codec.SealEnvelope("102004465", "200005200", map[string]any{
		"outRequestNo": "OPEN123",
		"contractNo":   "CP123",
		"status":       "1",
	})
	require.NoError(t, err)
	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	parser := NewParser(codec)
	notification, err := parser.ParseOpenAccountNotification(body)

	require.NoError(t, err)
	require.Equal(t, "OPEN123", notification.OutRequestNo)
	require.Equal(t, "CP123", notification.ContractNo)
	require.Equal(t, "active", notification.OpenState)
	require.True(t, json.Valid(notification.Raw))
}

func TestParserRejectsMissingCodec(t *testing.T) {
	parser := NewParser(nil)

	_, err := parser.ParseOpenAccountNotification([]byte(`{}`))

	require.EqualError(t, err, "baofu account notification parser is not configured")
}
