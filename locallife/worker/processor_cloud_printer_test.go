package worker

import (
	"testing"
	"time"

	"github.com/merrydance/locallife/cloudprint"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestBuildRuntimeCloudPrinterManagerWarnOnlyDisablesMalformedProvider(t *testing.T) {
	manager := buildRuntimeCloudPrinterManager(util.Config{
		FeieyunEnabled:       true,
		FeieyunUser:          "user",
		FeieyunUkey:          "ukey",
		ShangpengEnabled:     true,
		ShangpengAPIBaseURL:  "open.spyun.net",
		ShangpengAppID:       "appid",
		ShangpengAppSecret:   "secret",
		ShangpengHTTPTimeout: time.Second,
	})

	require.NotNil(t, manager)
	require.True(t, manager.Supported(string(cloudprint.ProviderFeieyun)))
	require.False(t, manager.Supported(string(cloudprint.ProviderShangpeng)))
}

func TestBuildRuntimeCloudPrinterManagerStrictModeDisablesMalformedProviderDefensively(t *testing.T) {
	manager := buildRuntimeCloudPrinterManager(util.Config{
		CloudPrinterFailOnProviderConfigError: true,
		FeieyunEnabled:                        true,
		FeieyunUser:                           "user",
		FeieyunUkey:                           "ukey",
		ShangpengEnabled:                      true,
		ShangpengAPIBaseURL:                   "open.spyun.net",
		ShangpengAppID:                        "appid",
		ShangpengAppSecret:                    "secret",
		ShangpengHTTPTimeout:                  time.Second,
	})

	require.NotNil(t, manager)
	require.True(t, manager.Supported(string(cloudprint.ProviderFeieyun)))
	require.False(t, manager.Supported(string(cloudprint.ProviderShangpeng)))
}
