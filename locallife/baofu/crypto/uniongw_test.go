package crypto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnionGWEncryptDecryptRoundTrip(t *testing.T) {
	codec, err := NewUnionGWCodec("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)

	payload := map[string]string{"contractNo": "CM123", "status": "1"}
	ciphertext, err := codec.EncryptJSON(payload)
	require.NoError(t, err)
	require.NotContains(t, ciphertext, "CM123")

	plaintext, err := codec.DecryptJSON(ciphertext)
	require.NoError(t, err)

	var decoded map[string]string
	require.NoError(t, json.Unmarshal(plaintext, &decoded))
	require.Equal(t, payload, decoded)
}

func TestUnionGWEnvelopeRejectsTamperedSignature(t *testing.T) {
	codec, err := NewUnionGWCodec("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)

	envelope, err := codec.SealEnvelope("102004465", "200005200", map[string]string{"contractNo": "CM123"})
	require.NoError(t, err)
	envelope.Sign = envelope.Sign[:len(envelope.Sign)-2] + "xx"

	_, err = codec.OpenEnvelope(envelope)
	require.ErrorIs(t, err, ErrInvalidEnvelopeSignature)
}
