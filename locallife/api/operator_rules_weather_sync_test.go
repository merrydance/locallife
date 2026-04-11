package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWeatherRuleKeyToPlatformConfigKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		want    string
		wantErr bool
	}{
		{name: "extreme", key: "WEATHER_COEFF_EXTREME", want: "WEATHER_COEFF_EXTREME"},
		{name: "heavy", key: "WEATHER_COEFF_HEAVY", want: "WEATHER_COEFF_HEAVY"},
		{name: "moderate", key: "WEATHER_COEFF_MODERATE", want: "WEATHER_COEFF_MODERATE"},
		{name: "light", key: "WEATHER_COEFF_LIGHT", want: "WEATHER_COEFF_LIGHT"},
		{name: "unknown", key: "OTHER", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := weatherRuleKeyToPlatformConfigKey(tc.key)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
