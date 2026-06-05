package main

import (
	"testing"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestBuildCommandInputUsesConfiguredCallbackURL(t *testing.T) {
	input, err := buildCommandInput(util.Config{
		DBSource:                  "postgres://db",
		YilianyunPrintCallbackURL: "https://api.example.com/v1/webhooks/yilianyun/print-result",
	}, "", 42, " YL-SN-001 ", "", " OPEN ")

	require.NoError(t, err)
	require.Equal(t, "postgres://db", input.dbURL)
	require.Equal(t, int64(42), input.merchantID)
	require.Equal(t, "YL-SN-001", input.machineCode)
	require.Equal(t, "https://api.example.com/v1/webhooks/yilianyun/print-result", input.callbackURL)
	require.Equal(t, "open", input.status)
}

func TestBuildCommandInputRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name        string
		cfg         util.Config
		dbURL       string
		merchantID  int64
		machineCode string
		callbackURL string
		status      string
		want        string
	}{
		{
			name:        "missing db",
			merchantID:  42,
			machineCode: "YL-SN-001",
			callbackURL: "https://api.example.com/callback",
			status:      "open",
			want:        "db connection string is empty",
		},
		{
			name:        "missing merchant",
			cfg:         util.Config{DBSource: "postgres://db"},
			machineCode: "YL-SN-001",
			callbackURL: "https://api.example.com/callback",
			status:      "open",
			want:        "-merchant-id must be a positive integer",
		},
		{
			name:        "invalid callback",
			cfg:         util.Config{DBSource: "postgres://db"},
			merchantID:  42,
			machineCode: "YL-SN-001",
			callbackURL: "ftp://api.example.com/callback",
			status:      "open",
			want:        "scheme must be http or https",
		},
		{
			name:        "invalid status",
			cfg:         util.Config{DBSource: "postgres://db"},
			merchantID:  42,
			machineCode: "YL-SN-001",
			callbackURL: "https://api.example.com/callback",
			status:      "pause",
			want:        "-status must be open or close",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildCommandInput(
				tc.cfg,
				tc.dbURL,
				tc.merchantID,
				tc.machineCode,
				tc.callbackURL,
				tc.status,
			)

			require.ErrorContains(t, err, tc.want)
		})
	}
}
