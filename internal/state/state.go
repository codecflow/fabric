package state

import (
	"fabric/internal/metering"
	"fabric/internal/network"
	"fabric/internal/repository"
	"fabric/internal/scheduler"
	"fabric/internal/storage"
	"fabric/internal/stream"
	"fabric/internal/types"
)

// State represents the application state with all dependencies
type State struct {
	Repository repository.Repository
	Stream     stream.Stream
	Meter      metering.Meter
	Storage    storage.Storage
	Network    network.Network
	Scheduler  scheduler.Scheduler
	Providers  map[string]types.Provider
}

// New creates a new State instance
func New() *State {
	return &State{
		Providers: make(map[string]types.Provider),
	}
}

// AddProvider adds a provider to the state
func (s *State) AddProvider(name string, provider types.Provider) {
	s.Providers[name] = provider
}

// GetProvider retrieves a provider by name
func (s *State) GetProvider(name string) (types.Provider, bool) {
	provider, exists := s.Providers[name]
	return provider, exists
}

// ListProviders returns all available providers
func (s *State) ListProviders() map[string]types.Provider {
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

	return lastErr
}
