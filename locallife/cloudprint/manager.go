package cloudprint

import "github.com/merrydance/locallife/util"

// ProviderType is the LocalLife cloud-printer provider key.
type ProviderType string

const (
	// ProviderFeieyun is the current production cloud-printer provider.
	ProviderFeieyun ProviderType = "feieyun"
	// ProviderYilianyun is reserved for the planned Yilianyun integration.
	ProviderYilianyun ProviderType = "yilianyun"
	// ProviderShangpeng is reserved for the planned Shangpeng integration.
	ProviderShangpeng ProviderType = "shangpeng"
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
// This rollout intentionally registers only Feieyun so existing runtime
// behavior stays unchanged while later providers are added behind the same
// dispatch boundary.
func NewManagerFromConfig(config util.Config) Manager {
	manager := &providerManager{providers: make(map[string]Client)}

	if feieyun := NewFeieyunClientFromConfig(config); feieyun != nil {
		manager.providers[string(ProviderFeieyun)] = feieyun
	}
	if shangpeng := NewShangpengClientFromConfig(config); shangpeng != nil {
		manager.providers[string(ProviderShangpeng)] = shangpeng
	}

	return manager
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
