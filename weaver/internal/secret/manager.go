package secret

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ExternalProvider defines the interface for external secret providers
type ExternalProvider interface {
	GetSecret(ctx context.Context, ref *ExternalSecretRef) (map[string][]byte, error)
	Name() string
}

// Manager handles secret operations with encryption
type Manager struct {
	repo      Repository
	encryptor *Encryptor
	providers map[string]ExternalProvider
}

// NewManager creates a new secret manager
func NewManager(repo Repository, encryptor *Encryptor) *Manager {
	return &Manager{
		repo:      repo,
		encryptor: encryptor,
		providers: make(map[string]ExternalProvider),
	}
}

// RegisterProvider registers an external secret provider
func (m *Manager) RegisterProvider(provider ExternalProvider) {
	m.providers[provider.Name()] = provider
}

// Create creates a new secret
func (m *Manager) Create(ctx context.Context, namespace, name string, spec *Spec) (*Secret, error) {
	// Generate ID
	id := uuid.New().String()

	// Encrypt data if provided
	encryptedData := make(map[string][]byte)
	if spec.Data != nil {
		for key, value := range spec.Data {
			encrypted, err := m.encryptor.Encrypt(value)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt secret data for key %s: %w", key, err)
			}
			encryptedData[key] = encrypted
		}
	}

	// Create secret
	secret := &Secret{
		ID:        id,
		Name:      name,
		Namespace: namespace,
		Spec: Spec{
			Type:        spec.Type,
			Data:        encryptedData,
			ExternalRef: spec.ExternalRef,
		},
		Status: Status{
			Phase: PhasePending,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// If external reference, sync from provider
	if spec.ExternalRef != nil {
		if err := m.syncFromExternal(ctx, secret); err != nil {
			secret.Status.Phase = PhaseFailed
			secret.Status.Message = err.Error()
		} else {
			secret.Status.Phase = PhaseActive
		}
	} else {
		secret.Status.Phase = PhaseActive
	}

	// Save to repository
	if err := m.repo.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	return secret, nil
}

// Get retrieves a secret by namespace and name
func (m *Manager) Get(ctx context.Context, namespace, name string) (*Secret, error) {
	secret, err := m.repo.Get(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	// Decrypt data for response
	if err := m.decryptSecretData(secret); err != nil {
		return nil, fmt.Errorf("failed to decrypt secret data: %w", err)
	}

	return secret, nil
}

// GetByID retrieves a secret by ID
func (m *Manager) GetByID(ctx context.Context, id string) (*Secret, error) {
	secret, err := m.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Decrypt data for response
	if err := m.decryptSecretData(secret); err != nil {
		return nil, fmt.Errorf("failed to decrypt secret data: %w", err)
	}

	return secret, nil
}

// Update updates an existing secret
func (m *Manager) Update(ctx context.Context, namespace, name string, spec *Spec) (*Secret, error) {
	// Get existing secret
	secret, err := m.repo.Get(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	// Encrypt new data if provided
	if spec.Data != nil {
		encryptedData := make(map[string][]byte)
		for key, value := range spec.Data {
			encrypted, err := m.encryptor.Encrypt(value)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt secret data for key %s: %w", key, err)
			}
			encryptedData[key] = encrypted
		}
		secret.Spec.Data = encryptedData
	}

	// Update other fields
	secret.Spec.Type = spec.Type
	secret.Spec.ExternalRef = spec.ExternalRef
	secret.UpdatedAt = time.Now()

	// If external reference, sync from provider
	if spec.ExternalRef != nil {
		if err := m.syncFromExternal(ctx, secret); err != nil {
			secret.Status.Phase = PhaseFailed
			secret.Status.Message = err.Error()
		} else {
			secret.Status.Phase = PhaseActive
		}
	} else {
		secret.Status.Phase = PhaseActive
	}

	// Save to repository
	if err := m.repo.Update(ctx, secret); err != nil {
		return nil, fmt.Errorf("failed to update secret: %w", err)
	}

	// Decrypt data for response
	if err := m.decryptSecretData(secret); err != nil {
		return nil, fmt.Errorf("failed to decrypt secret data: %w", err)
	}

	return secret, nil
}

// Delete deletes a secret
func (m *Manager) Delete(ctx context.Context, namespace, name string) error {
	return m.repo.Delete(ctx, namespace, name)
}

// List lists secrets with optional filtering
func (m *Manager) List(ctx context.Context, filter Filter) (*List, error) {
	list, err := m.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Decrypt data for all secrets in response
	for i := range list.Items {
		if err := m.decryptSecretData(&list.Items[i]); err != nil {
			return nil, fmt.Errorf("failed to decrypt secret data for %s/%s: %w",
				list.Items[i].Namespace, list.Items[i].Name, err)
		}
	}

	return list, nil
}

// SyncExternal syncs secrets from external providers
func (m *Manager) SyncExternal(ctx context.Context, namespace, name string) error {
	secret, err := m.repo.Get(ctx, namespace, name)
	if err != nil {
		return err
	}

	if secret.Spec.ExternalRef == nil {
		return fmt.Errorf("secret %s/%s has no external reference", namespace, name)
	}

	if err := m.syncFromExternal(ctx, secret); err != nil {
		secret.Status.Phase = PhaseFailed
		secret.Status.Message = err.Error()
		secret.Status.SyncError = err.Error()
	} else {
		secret.Status.Phase = PhaseActive
		secret.Status.Message = ""
		secret.Status.SyncError = ""
		now := time.Now()
		secret.Status.LastSync = &now
	}

	secret.UpdatedAt = time.Now()
	return m.repo.Update(ctx, secret)
}

// syncFromExternal syncs data from external provider
func (m *Manager) syncFromExternal(ctx context.Context, secret *Secret) error {
	if secret.Spec.ExternalRef == nil {
		return nil
	}

	provider, exists := m.providers[secret.Spec.ExternalRef.Provider]
	if !exists {
		return fmt.Errorf("external provider %s not found", secret.Spec.ExternalRef.Provider)
	}

	// Get data from external provider
	data, err := provider.GetSecret(ctx, secret.Spec.ExternalRef)
	if err != nil {
		return fmt.Errorf("failed to get secret from external provider: %w", err)
	}

	// Encrypt the data
	encryptedData := make(map[string][]byte)
	for key, value := range data {
		encrypted, err := m.encryptor.Encrypt(value)
		if err != nil {
			return fmt.Errorf("failed to encrypt external secret data for key %s: %w", key, err)
		}
		encryptedData[key] = encrypted
	}

	secret.Spec.Data = encryptedData
	return nil
}

// decryptSecretData decrypts the data in a secret for response
func (m *Manager) decryptSecretData(secret *Secret) error {
	if secret.Spec.Data == nil {
		return nil
	}

	decryptedData := make(map[string][]byte)
	for key, encryptedValue := range secret.Spec.Data {
		decrypted, err := m.encryptor.Decrypt(encryptedValue)
		if err != nil {
			return fmt.Errorf("failed to decrypt data for key %s: %w", key, err)
		}
		decryptedData[key] = decrypted
	}

	secret.Spec.Data = decryptedData
	return nil
}

// GetDecryptedValue gets a specific decrypted value from a secret
func (m *Manager) GetDecryptedValue(ctx context.Context, namespace, name, key string) ([]byte, error) {
	secret, err := m.repo.Get(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	if secret.Spec.Data == nil {
		return nil, fmt.Errorf("secret %s/%s has no data", namespace, name)
	}

	encryptedValue, exists := secret.Spec.Data[key]
	if !exists {
		return nil, fmt.Errorf("key %s not found in secret %s/%s", key, namespace, name)
	}

	return m.encryptor.Decrypt(encryptedValue)
}
