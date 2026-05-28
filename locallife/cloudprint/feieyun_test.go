package cloudprint

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestFeieyunPrintIncludesConfiguredCallbackURL(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/Api/Open/printMsg", r.URL.Path)
		require.NoError(t, r.ParseForm())
		received = r.PostForm
		require.NoError(t, json.NewEncoder(w).Encode(feieyunResponse{
			Ret:  0,
			Msg:  "ok",
			Data: json.RawMessage(`"vendor-order-123"`),
		}))
	}))
	defer server.Close()

	client := NewFeieyunClientFromConfig(util.Config{
		FeieyunEnabled:              true,
		FeieyunAPIBaseURL:           server.URL,
		FeieyunUser:                 "user",
		FeieyunUkey:                 "ukey",
		FeieyunPrintCallbackURL:     "https://api.example.com/v1/webhooks/feieyun/print-result",
		FeieyunCallbackPublicKeyPEM: "configured-public-key",
	})

	orderID, err := client.Print(context.Background(), PrintInput{
		SN:      "sn-001",
		Content: "hello",
		Copies:  2,
	})

	require.NoError(t, err)
	require.Equal(t, "vendor-order-123", orderID)
	require.Equal(t, "https://api.example.com/v1/webhooks/feieyun/print-result", received.Get("backurl"))
	require.Equal(t, "sn-001", received.Get("sn"))
	require.Equal(t, "hello", received.Get("content"))
	require.Equal(t, "2", received.Get("times"))
	require.Equal(t, "Open_printMsg", received.Get("apiname"))
	require.True(t, client.PrintResultCallbackEnabled())
}

func TestFeieyunPrintOmitsCallbackURLWhenPublicKeyMissing(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		received = r.PostForm
		require.NoError(t, json.NewEncoder(w).Encode(feieyunResponse{
			Ret:  0,
			Msg:  "ok",
			Data: json.RawMessage(`"vendor-order-456"`),
		}))
	}))
	defer server.Close()

	client := NewFeieyunClientFromConfig(util.Config{
		FeieyunEnabled:          true,
		FeieyunAPIBaseURL:       server.URL,
		FeieyunUser:             "user",
		FeieyunUkey:             "ukey",
		FeieyunPrintCallbackURL: "https://api.example.com/v1/webhooks/feieyun/print-result",
	})

	_, err := client.Print(context.Background(), PrintInput{SN: "sn-001", Content: "hello"})

	require.NoError(t, err)
	require.Empty(t, received.Get("backurl"))
	require.False(t, client.PrintResultCallbackEnabled())
}

func TestFeieyunPrintOmitsEmptyCallbackURL(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		received = r.PostForm
		require.NoError(t, json.NewEncoder(w).Encode(feieyunResponse{
			Ret:  0,
			Msg:  "ok",
			Data: json.RawMessage(`"vendor-order-456"`),
		}))
	}))
	defer server.Close()

	client := NewFeieyunClientFromConfig(util.Config{
		FeieyunEnabled:    true,
		FeieyunAPIBaseURL: server.URL,
		FeieyunUser:       "user",
		FeieyunUkey:       "ukey",
	})

	_, err := client.Print(context.Background(), PrintInput{SN: "sn-001", Content: "hello"})

	require.NoError(t, err)
	require.Empty(t, received.Get("backurl"))
	require.False(t, client.PrintResultCallbackEnabled())
}

func TestBuildFeieyunCallbackCanonicalString(t *testing.T) {
	values := url.Values{
		"status":  []string{"1"},
		"sign":    []string{"signature"},
		"stime":   []string{"1625194910"},
		"empty":   []string{""},
		"orderId": []string{"816501678_20160919184316_1419533539"},
	}

	canonical := BuildFeieyunCallbackCanonicalString(values)

	require.Equal(t, "orderId=816501678_20160919184316_1419533539&status=1&stime=1625194910", canonical)
}

func TestVerifyFeieyunCallbackSignature(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))

	values := url.Values{
		"orderId": []string{"vendor-order-123"},
		"status":  []string{"1"},
		"stime":   []string{"1625194910"},
	}
	digest := sha256.Sum256([]byte(BuildFeieyunCallbackCanonicalString(values)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	require.NoError(t, err)
	values.Set("sign", base64.StdEncoding.EncodeToString(signature))

	require.NoError(t, VerifyFeieyunCallbackSignature(values, publicPEM))

	values.Set("status", "2")
	require.Error(t, VerifyFeieyunCallbackSignature(values, publicPEM))
}
