package cloudprint

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestShangpengPrintSignsFormRequest(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/printer/print", r.URL.Path)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		received = r.PostForm
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"errorcode":   0,
			"id":          "sp-order-123",
			"create_time": "2026-06-05 08:00:00",
		}))
	}))
	defer server.Close()

	client := newShangpengClientFromConfig(util.Config{
		ShangpengEnabled:     true,
		ShangpengAPIBaseURL:  server.URL,
		ShangpengAppID:       "appid-001",
		ShangpengAppSecret:   "secret-001",
		ShangpengHTTPTimeout: time.Second,
	}, fixedClock("1717555200"))

	orderID, err := client.Print(context.Background(), PrintInput{
		SN:      "SP-SN-001",
		Content: "<C>hello</C>",
		Copies:  2,
	})

	require.NoError(t, err)
	require.Equal(t, "sp-order-123", orderID)
	require.Equal(t, "appid-001", received.Get("appid"))
	require.Equal(t, "1717555200", received.Get("timestamp"))
	require.Equal(t, "SP-SN-001", received.Get("sn"))
	require.Equal(t, "<C>hello</C>", received.Get("content"))
	require.Equal(t, "2", received.Get("times"))
	require.Equal(t, BuildShangpengSign(received, "secret-001"), received.Get("sign"))
	require.Equal(t, strings.ToUpper(received.Get("sign")), received.Get("sign"))
	require.False(t, client.PrintResultCallbackEnabled())
}

func TestNewShangpengClientFromConfigRequiresEnabledCredentials(t *testing.T) {
	require.Nil(t, NewShangpengClientFromConfig(util.Config{
		ShangpengEnabled:   false,
		ShangpengAppID:     "appid-001",
		ShangpengAppSecret: "secret-001",
	}))
	require.Nil(t, NewShangpengClientFromConfig(util.Config{
		ShangpengEnabled:   true,
		ShangpengAppSecret: "secret-001",
	}))
	require.Nil(t, NewShangpengClientFromConfig(util.Config{
		ShangpengEnabled: true,
		ShangpengAppID:   "appid-001",
	}))
	require.NotNil(t, NewShangpengClientFromConfig(util.Config{
		ShangpengEnabled:   true,
		ShangpengAppID:     "appid-001",
		ShangpengAppSecret: "secret-001",
	}))
}

func TestBuildShangpengSignSkipsEmptyValuesAndExistingSign(t *testing.T) {
	values := url.Values{
		"appid":     []string{"appid-001"},
		"empty":     []string{""},
		"signature": []string{"kept"},
		"sign":      []string{"ignored"},
		"sn":        []string{"SP-SN-001"},
		"timestamp": []string{"1717555200"},
	}

	require.Equal(t,
		"EF10EBF842994880FE2E558F2BB8556C",
		BuildShangpengSign(values, "secret-001"),
	)
}

func TestShangpengAddAndRemovePrinterUseOfficialFields(t *testing.T) {
	seen := make([]url.Values, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		seen = append(seen, r.Form)
		switch len(seen) {
		case 1:
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v1/printer/add", r.URL.Path)
		case 2:
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/printer/delete", r.URL.Path)
		default:
			t.Fatalf("unexpected request %d", len(seen))
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"errorcode": 0}))
	}))
	defer server.Close()

	client := newShangpengClientFromConfig(util.Config{
		ShangpengEnabled:     true,
		ShangpengAPIBaseURL:  server.URL,
		ShangpengAppID:       "appid-001",
		ShangpengAppSecret:   "secret-001",
		ShangpengHTTPTimeout: time.Second,
	}, fixedClock("1717555200"))

	require.NoError(t, client.AddPrinter(context.Background(), AddPrinterInput{
		SN:       "SP-SN-001",
		Key:      "printer-key",
		Name:     "front desk",
		Business: "merchant-100",
	}))
	require.NoError(t, client.RemovePrinter(context.Background(), RemovePrinterInput{
		SN:       "SP-SN-001",
		Business: "merchant-100",
	}))

	require.Len(t, seen, 2)
	require.Equal(t, "merchant-100", seen[0].Get("business"))
	require.Equal(t, "SP-SN-001", seen[0].Get("sn"))
	require.Equal(t, "printer-key", seen[0].Get("pkey"))
	require.Equal(t, "front desk", seen[0].Get("name"))
	require.Equal(t, "merchant-100", seen[1].Get("business"))
	require.Equal(t, "SP-SN-001", seen[1].Get("sn"))
}

func TestShangpengQueryOrderStateAndPrinterInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		switch r.URL.Path {
		case "/v1/printer/order/status":
			require.Equal(t, "sp-order-123", r.URL.Query().Get("id"))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"errorcode":  0,
				"status":     true,
				"print_time": "2026-06-05 08:01:00",
			}))
		case "/v1/printer/info":
			require.Equal(t, "SP-SN-001", r.URL.Query().Get("sn"))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"errorcode": 0,
				"online":    1,
				"status":    0,
				"sqsnum":    3,
				"model":     "SP-P1",
			}))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newShangpengClientFromConfig(util.Config{
		ShangpengEnabled:     true,
		ShangpengAPIBaseURL:  server.URL,
		ShangpengAppID:       "appid-001",
		ShangpengAppSecret:   "secret-001",
		ShangpengHTTPTimeout: time.Second,
	}, fixedClock("1717555200"))

	printed, err := client.QueryOrderState(context.Background(), "sp-order-123")
	require.NoError(t, err)
	require.True(t, printed)

	status, err := client.QueryPrinterStatus(context.Background(), "SP-SN-001")
	require.NoError(t, err)
	require.Equal(t, "online", status)

	info, err := client.GetPrinterInfo(context.Background(), "SP-SN-001")
	require.NoError(t, err)
	require.Equal(t, "SP-P1", info.Model)
	require.Equal(t, "online", info.Status)
	require.Nil(t, info.PrintLogo)
	require.Nil(t, info.ScanSwitch)
}

func TestShangpengProviderErrorsAreMapped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"errorcode": -4,
			"errormsg":  "signature error",
		}))
	}))
	defer server.Close()

	client := newShangpengClientFromConfig(util.Config{
		ShangpengEnabled:     true,
		ShangpengAPIBaseURL:  server.URL,
		ShangpengAppID:       "appid-001",
		ShangpengAppSecret:   "secret-001",
		ShangpengHTTPTimeout: time.Second,
	}, fixedClock("1717555200"))

	_, err := client.Print(context.Background(), PrintInput{SN: "SP-SN-001", Content: "hello"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "shangpeng api error -4")
	require.Contains(t, err.Error(), "signature error")
}

func TestShangpengMalformedRequiredFieldsFailClosed(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		want     string
	}{
		{name: "missing errorcode", response: map[string]any{"id": "1"}, want: "missing errorcode"},
		{name: "missing print order id", response: map[string]any{"errorcode": 0}, want: "missing print order id"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewEncoder(w).Encode(tc.response))
			}))
			defer server.Close()

			client := newShangpengClientFromConfig(util.Config{
				ShangpengEnabled:     true,
				ShangpengAPIBaseURL:  server.URL,
				ShangpengAppID:       "appid-001",
				ShangpengAppSecret:   "secret-001",
				ShangpengHTTPTimeout: time.Second,
			}, fixedClock("1717555200"))

			_, err := client.Print(context.Background(), PrintInput{SN: "SP-SN-001", Content: "hello"})

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestShangpengMalformedStatusFieldsFailClosed(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		response map[string]any
		call     func(Client) error
		want     string
	}{
		{
			name:     "missing order status",
			path:     "/v1/printer/order/status",
			response: map[string]any{"errorcode": 0},
			call: func(client Client) error {
				_, err := client.QueryOrderState(context.Background(), "sp-order-123")
				return err
			},
			want: "missing order status",
		},
		{
			name:     "missing printer status",
			path:     "/v1/printer/info",
			response: map[string]any{"errorcode": 0, "online": 1},
			call: func(client Client) error {
				_, err := client.GetPrinterInfo(context.Background(), "SP-SN-001")
				return err
			},
			want: "missing printer status",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, tc.path, r.URL.Path)
				require.NoError(t, json.NewEncoder(w).Encode(tc.response))
			}))
			defer server.Close()

			client := newShangpengClientFromConfig(util.Config{
				ShangpengEnabled:     true,
				ShangpengAPIBaseURL:  server.URL,
				ShangpengAppID:       "appid-001",
				ShangpengAppSecret:   "secret-001",
				ShangpengHTTPTimeout: time.Second,
			}, fixedClock("1717555200"))

			err := tc.call(client)

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func fixedClock(value string) func() string {
	return func() string { return value }
}
