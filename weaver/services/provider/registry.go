package provider

import (
	"fmt"
)

// Registry manages provider instances
type Registry struct {
	providers map[string]Provider
	factories map[ProviderType]ProviderFactory
}

// ProviderFactory creates provider instances
type ProviderFactory func(name string, config map[string]string) (Provider, error)

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		factories: make(map[ProviderType]ProviderFactory),
	}
}

// RegisterFactory registers a provider factory
func (r *Registry) RegisterFactory(providerType ProviderType, factory ProviderFactory) {
	r.factories[providerType] = factory
}

// RegisterProvider registers a provider with the registry
func (r *Registry) RegisterProvider(provider Provider) {
	r.providers[provider.Name()] = provider
}

// GetProvider retrieves a provider by name
func (r *Registry) GetProvider(name string) (Provider, error) {
	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return provider, nil
}

// ListProviders returns all registered providers
func (r *Registry) ListProviders() []Provider {
	providers := make([]Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		providers = append(providers, provider)
	}
	return providers
}

// CreateProvider creates a new provider instance based on configuration
func (r *Registry) CreateProvider(config ProviderConfig) (Provider, error) {
	factory, exists := r.factories[config.Type]
	if !exists {
		return nil, fmt.Errorf("unsupported provider type: %s", config.Type)
	}

	return factory(config.Name, config.Config)
}

// RegisterFromConfigs registers providers from a list of configurations
func (r *Registry) RegisterFromConfigs(configs []ProviderConfig) error {
	for _, config := range configs {
		if !config.Enabled {
			continue
		}

		provider, err := r.CreateProvider(config)
		if err != nil {
			return fmt.Errorf("failed to create provider %s: %w", config.Name, err)
		}

		r.RegisterProvider(provider)
	}
	return nil
}
