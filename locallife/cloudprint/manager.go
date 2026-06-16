package cloudprint

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/merrydance/locallife/util"
)

// ProviderType is the LocalLife cloud-printer provider key.
type ProviderType string

const (
	// ProviderFeieyun is the current production cloud-printer provider.
	ProviderFeieyun ProviderType = "feieyun"
	// ProviderYilianyun is reserved for the planned Yilianyun integration.
	ProviderYilianyun ProviderType = "yilianyun"
	// ProviderShangpeng is reserved for the planned Shangpeng integration.
	ProviderShangpeng ProviderType = "shangpeng"
	// ProviderSelfCloud is LocalLife's own cloud-printer provider.
	ProviderSelfCloud ProviderType = "self_cloud"
)

// Manager exposes the cloud-printer providers that are configured at runtime.
type Manager interface {
	Provider(providerType string) (Client, bool)
	Supported(providerType string) bool
}

type providerManager struct {
	providers map[string]Client
}

// NewManagerFromConfig builds the configured provider registry.
//
// Yilianyun open-app printers require per-printer authorization tokens and are
// adapted at the caller runtime boundary instead of this provider-only manager.
func NewManagerFromConfig(config util.Config) Manager {
	manager := &providerManager{providers: make(map[string]Client)}

	if feieyun := NewFeieyunClientFromConfig(config); feieyun != nil {
		manager.providers[string(ProviderFeieyun)] = feieyun
	}
	if shangpeng := NewShangpengClientFromConfig(config); shangpeng != nil {
		manager.providers[string(ProviderShangpeng)] = shangpeng
	}
	if printServer := NewPrintServerClientFromConfig(config); printServer != nil {
		manager.providers[string(ProviderSelfCloud)] = printServer
	}

	return manager
}

// NewRuntimeManagerFromConfig validates provider settings before registering
// runtime clients. In warn-only mode it returns a runtime-safe manager plus
// the validation error for the caller's log boundary.
func NewRuntimeManagerFromConfig(config util.Config) (Manager, error) {
	configErr := config.ValidateCloudPrinterProviderConfig()
	if configErr != nil && config.CloudPrinterFailOnProviderConfigError {
		return nil, configErr
	}

	runtimeConfig := config
	if err := validateYilianyunRuntimeProviderConfig(config); err != nil {
		runtimeConfig.YilianyunEnabled = false
	}
	if err := validateShangpengRuntimeProviderConfig(config); err != nil {
		runtimeConfig.ShangpengEnabled = false
	}
	if err := validatePrintServerRuntimeProviderConfig(config); err != nil {
		runtimeConfig.PrintServerEnabled = false
	}

	return NewManagerFromConfig(runtimeConfig), configErr
}

func validateYilianyunRuntimeProviderConfig(config util.Config) error {
	if !config.YilianyunEnabled {
		return nil
	}
	if strings.TrimSpace(config.YilianyunAPIBaseURL) == "" ||
		strings.TrimSpace(config.YilianyunAppID) == "" ||
		strings.TrimSpace(config.YilianyunAppSecret) == "" {
		return fmt.Errorf("YILIANYUN_API_BASE_URL, YILIANYUN_APP_ID and YILIANYUN_APP_SECRET are required when YILIANYUN_ENABLED=true")
	}
	if err := validateRuntimeProviderAbsoluteURL("YILIANYUN_API_BASE_URL", config.YilianyunAPIBaseURL); err != nil {
		return err
	}
	if config.YilianyunHTTPTimeout <= 0 {
		return fmt.Errorf("YILIANYUN_HTTP_TIMEOUT must be > 0 when YILIANYUN_ENABLED=true")
	}
	return nil
}

func validateShangpengRuntimeProviderConfig(config util.Config) error {
	if !config.ShangpengEnabled {
		return nil
	}
	if strings.TrimSpace(config.ShangpengAPIBaseURL) == "" ||
		strings.TrimSpace(config.ShangpengAppID) == "" ||
		strings.TrimSpace(config.ShangpengAppSecret) == "" {
		return fmt.Errorf("SHANGPENG_API_BASE_URL, SHANGPENG_APPID and SHANGPENG_APPSECRET are required when SHANGPENG_ENABLED=true")
	}
	if err := validateRuntimeProviderAbsoluteURL("SHANGPENG_API_BASE_URL", config.ShangpengAPIBaseURL); err != nil {
		return err
	}
	if config.ShangpengHTTPTimeout <= 0 {
		return fmt.Errorf("SHANGPENG_HTTP_TIMEOUT must be > 0 when SHANGPENG_ENABLED=true")
	}
	return nil
}

func validatePrintServerRuntimeProviderConfig(config util.Config) error {
	if !config.PrintServerEnabled {
		return nil
	}
	if strings.TrimSpace(config.PrintServerAPIBaseURL) == "" ||
		strings.TrimSpace(config.PrintServerAppID) == "" ||
		strings.TrimSpace(config.PrintServerSecret) == "" {
		return fmt.Errorf("PRINT_SERVER_API_BASE_URL, PRINT_SERVER_APP_ID and PRINT_SERVER_SECRET are required when PRINT_SERVER_ENABLED=true")
	}
	if err := validateRuntimeProviderAbsoluteURL("PRINT_SERVER_API_BASE_URL", config.PrintServerAPIBaseURL); err != nil {
		return err
	}
	if config.PrintServerHTTPTimeout <= 0 {
		return fmt.Errorf("PRINT_SERVER_HTTP_TIMEOUT must be > 0 when PRINT_SERVER_ENABLED=true")
	}
	return nil
}

func validateRuntimeProviderAbsoluteURL(name, value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid absolute URL", name)
	}
	return nil
}

func (m *providerManager) Provider(providerType string) (Client, bool) {
	if m == nil {
		return nil, false
	}
	provider, ok := m.providers[providerType]
	return provider, ok
}

func (m *providerManager) Supported(providerType string) bool {
	_, ok := m.Provider(providerType)
	return ok
}

func (m *providerManager) Configured() bool {
	return m != nil && len(m.providers) > 0
}
