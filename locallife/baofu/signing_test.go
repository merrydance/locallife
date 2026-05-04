package baofu

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalJSONSortsObjectKeys(t *testing.T) {
	input := map[string]any{
		"z": "last",
		"a": map[string]any{"b": 2, "a": 1},
	}

	canonical, err := CanonicalJSON(input)

	require.NoError(t, err)
	require.Equal(t, `{"a":{"a":1,"b":2},"z":"last"}`, string(canonical))
}

func TestSignAndVerifySHA256WithRSA(t *testing.T) {
	privatePEM, publicPEM := generateBaofuTestKeyPair(t)
	message := []byte(`{"merchantNo":"102004465","outTradeNo":"BF123"}`)

	signature, err := SignSHA256WithRSA(privatePEM, message)
	require.NoError(t, err)
	require.NotEmpty(t, signature)
	_, err = hex.DecodeString(signature)
	require.NoError(t, err)

	require.NoError(t, VerifySHA256WithRSA(publicPEM, message, signature))
	require.ErrorIs(t, VerifySHA256WithRSA(publicPEM, []byte(`{"tampered":true}`), signature), ErrInvalidSignature)
}

func generateBaofuTestKeyPair(t *testing.T) (string, string) {
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
