package state

import (
	"weaver/internal/metering"
	"weaver/internal/network"
	"weaver/internal/provider"
	"weaver/internal/proxy"
	"weaver/internal/repository"
	"weaver/internal/scheduler"
	"weaver/internal/storage"
	"weaver/internal/stream"
)

// State represents the application state with all dependencies
type State struct {
	Repository *repository.Repository
	Stream     stream.Stream
	Meter      metering.Meter
	Storage    storage.Storage
	Network    network.Network
	Scheduler  scheduler.Scheduler
	Proxy      *proxy.Server
	Providers  map[string]provider.Provider
}

// New creates a new State instance
func New() *State {
	return &State{
		Providers: make(map[string]provider.Provider),
	}
}

// AddProvider adds a provider to the state
func (s *State) AddProvider(name string, provider provider.Provider) {
	s.Providers[name] = provider
}

// GetProvider retrieves a provider by name
func (s *State) GetProvider(name string) (provider.Provider, bool) {
	provider, exists := s.Providers[name]
	return provider, exists
}

// ListProviders returns all available providers
func (s *State) ListProviders() map[string]provider.Provider {
	return s.Providers
}

// Close closes all connections and cleans up resources
func (s *State) Close() error {
	var lastErr error

	// Close all providers
	for _, provider := range s.Providers {
		if err := provider.HealthCheck(nil); err != nil {
			lastErr = err
		}
	}

	// Close network
	if s.Network != nil {
		if err := s.Network.Close(); err != nil {
			lastErr = err
		}
	}

	// Close storage
	if s.Storage != nil {
		if err := s.Storage.Close(); err != nil {
			lastErr = err
		}
	}

	// Close meter
	if s.Meter != nil {
		if err := s.Meter.Close(); err != nil {
			lastErr = err
		}
	}

	// Close stream
	if s.Stream != nil {
		if err := s.Stream.Close(); err != nil {
			lastErr = err
		}
	}

	// Close repository
	if s.Repository != nil {
		if err := s.Repository.Close(); err != nil {
			lastErr = err
		}
	}

	// Close proxy
	if s.Proxy != nil {
		if err := s.Proxy.Stop(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}
