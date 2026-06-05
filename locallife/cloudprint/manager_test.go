package cloudprint

import (
	"testing"
	"time"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestNewManagerFromConfigRegistersFeieyunWhenEnabled(t *testing.T) {
	manager := NewManagerFromConfig(util.Config{
		FeieyunEnabled: true,
		FeieyunUser:    "user",
		FeieyunUkey:    "ukey",
	})

	require.True(t, manager.Supported(string(ProviderFeieyun)))
	require.False(t, manager.Supported(string(ProviderYilianyun)))
	require.False(t, manager.Supported(string(ProviderShangpeng)))

	provider, ok := manager.Provider(string(ProviderFeieyun))
	require.True(t, ok)
	require.NotNil(t, provider)
	require.False(t, provider.PrintResultCallbackEnabled())
}

func TestNewManagerFromConfigRegistersShangpengWhenEnabled(t *testing.T) {
	manager := NewManagerFromConfig(util.Config{
		ShangpengEnabled:   true,
		ShangpengAppID:     "appid",
		ShangpengAppSecret: "secret",
	})

	require.True(t, manager.Supported(string(ProviderShangpeng)))
	require.False(t, manager.Supported(string(ProviderYilianyun)))

	provider, ok := manager.Provider(string(ProviderShangpeng))
	require.True(t, ok)
	require.NotNil(t, provider)
	require.False(t, provider.PrintResultCallbackEnabled())
}

func TestNewManagerFromConfigOmitsFeieyunWhenDisabledOrIncomplete(t *testing.T) {
	testCases := []struct {
		name   string
		config util.Config
	}{
		{
			name: "disabled",
			config: util.Config{
				FeieyunEnabled: false,
				FeieyunUser:    "user",
				FeieyunUkey:    "ukey",
			},
		},
		{
			name: "missing user",
			config: util.Config{
				FeieyunEnabled: true,
				FeieyunUkey:    "ukey",
			},
		},
		{
			name: "missing ukey",
			config: util.Config{
				FeieyunEnabled: true,
				FeieyunUser:    "user",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := NewManagerFromConfig(tc.config)

			require.False(t, manager.Supported(string(ProviderFeieyun)))
			provider, ok := manager.Provider(string(ProviderFeieyun))
			require.False(t, ok)
			require.Nil(t, provider)
		})
	}
}

func TestManagerRejectsUnknownProvider(t *testing.T) {
	manager := NewManagerFromConfig(util.Config{
		FeieyunEnabled: true,
		FeieyunUser:    "user",
		FeieyunUkey:    "ukey",
	})

	require.False(t, manager.Supported("unknown"))
	provider, ok := manager.Provider("unknown")
	require.False(t, ok)
	require.Nil(t, provider)
}

func TestManagerDoesNotRegisterYilianyunBecauseRuntimeNeedsPrinterAuthorization(t *testing.T) {
	manager := NewManagerFromConfig(util.Config{
		YilianyunEnabled:         true,
		YilianyunAPIBaseURL:      "https://open-api.10ss.net/v2",
		YilianyunAppID:           "app",
		YilianyunAppSecret:       "secret",
		YilianyunAuthCallbackURL: "https://api.example.com/v1/merchant/devices/yilianyun/auth/callback",
	})

	require.False(t, manager.Supported(string(ProviderYilianyun)))
}

func TestNewRuntimeManagerFromConfigDisablesMalformedProviderInWarnOnlyMode(t *testing.T) {
	manager, err := NewRuntimeManagerFromConfig(util.Config{
		FeieyunEnabled:       true,
		FeieyunUser:          "user",
		FeieyunUkey:          "ukey",
		ShangpengEnabled:     true,
		ShangpengAPIBaseURL:  "open.spyun.net",
		ShangpengAppID:       "appid",
		ShangpengAppSecret:   "secret",
		ShangpengHTTPTimeout: time.Second,
	})

	require.Error(t, err)
	require.NotNil(t, manager)
	require.True(t, manager.Supported(string(ProviderFeieyun)))
	require.False(t, manager.Supported(string(ProviderShangpeng)))
}

func TestNewRuntimeManagerFromConfigFailsOnMalformedProviderInStrictMode(t *testing.T) {
	manager, err := NewRuntimeManagerFromConfig(util.Config{
		CloudPrinterFailOnProviderConfigError: true,
		ShangpengEnabled:                      true,
		ShangpengAPIBaseURL:                   "open.spyun.net",
		ShangpengAppID:                        "appid",
		ShangpengAppSecret:                    "secret",
		ShangpengHTTPTimeout:                  time.Second,
	})

	require.Error(t, err)
	require.Nil(t, manager)
}

func TestNewRuntimeManagerFromConfigKeepsValidProviderWhenOnlyPollConfigIsInvalid(t *testing.T) {
	manager, err := NewRuntimeManagerFromConfig(util.Config{
		ShangpengEnabled:               true,
		ShangpengAPIBaseURL:            "https://open.spyun.net",
		ShangpengAppID:                 "appid",
		ShangpengAppSecret:             "secret",
		ShangpengHTTPTimeout:           time.Second,
		CloudPrinterStatusPollInterval: -time.Second,
	})

	require.Error(t, err)
	require.NotNil(t, manager)
	require.True(t, manager.Supported(string(ProviderShangpeng)))
}
