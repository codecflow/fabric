package secret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"
)

// Secret represents a stored secret
type Secret struct {
	ID          string
	WorkspaceID string
	Name        string
	Type        SecretType
	Value       string // Encrypted
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SecretType defines the type of secret
type SecretType string

const (
	SecretTypeGeneric  SecretType = "generic"
	SecretTypeRegistry SecretType = "registry"
	SecretTypeSSH      SecretType = "ssh"
	SecretTypeTLS      SecretType = "tls"
	SecretTypeToken    SecretType = "token"
)

// Manager handles secret storage and retrieval
type Manager struct {
	mu      sync.RWMutex
	secrets map[string]*Secret
	encKey  []byte
}

// NewManager creates a new secret manager
func NewManager(encryptionKey []byte) (*Manager, error) {
	// Validate key length (must be 16, 24, or 32 bytes for AES)
	if len(encryptionKey) != 16 && len(encryptionKey) != 24 && len(encryptionKey) != 32 {
		return nil, fmt.Errorf("encryption key must be 16, 24, or 32 bytes")
	}

	return &Manager{
		secrets: make(map[string]*Secret),
		encKey:  encryptionKey,
	}, nil
}

// Create creates a new secret
func (m *Manager) Create(ctx context.Context, workspaceID, name string, secretType SecretType, value string) (*Secret, error) {
	// Validate inputs
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if name == "" {
		return nil, fmt.Errorf("secret name is required")
	}
	if value == "" {
		return nil, fmt.Errorf("secret value is required")
	}

	// Encrypt the value
	encrypted, err := m.encrypt(value)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %w", err)
	}

	// Create secret
	secretID := fmt.Sprintf("secret-%d", time.Now().UnixNano())
	secret := &Secret{
		ID:          secretID,
		WorkspaceID: workspaceID,
		Name:        name,
		Type:        secretType,
		Value:       encrypted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Store
	m.mu.Lock()
	m.secrets[secretID] = secret
	m.mu.Unlock()

	return secret, nil
}

// Get retrieves a secret by ID
func (m *Manager) Get(ctx context.Context, secretID string) (*Secret, error) {
	m.mu.RLock()
	secret, ok := m.secrets[secretID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("secret not found: %s", secretID)
	}

	return secret, nil
}

// GetByName retrieves a secret by workspace and name
func (m *Manager) GetByName(ctx context.Context, workspaceID, name string) (*Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, secret := range m.secrets {
		if secret.WorkspaceID == workspaceID && secret.Name == name {
			return secret, nil
		}
	}

	return nil, fmt.Errorf("secret not found: %s/%s", workspaceID, name)
}

// GetDecrypted retrieves and decrypts a secret value
func (m *Manager) GetDecrypted(ctx context.Context, secretID string) (string, error) {
	secret, err := m.Get(ctx, secretID)
	if err != nil {
		return "", err
	}

	// Decrypt the value
	decrypted, err := m.decrypt(secret.Value)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret: %w", err)
	}

	return decrypted, nil
}

// List returns all secrets for a workspace
func (m *Manager) List(ctx context.Context, workspaceID string) ([]*Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	secrets := make([]*Secret, 0)
	for _, secret := range m.secrets {
		if secret.WorkspaceID == workspaceID {
			secrets = append(secrets, secret)
		}
	}

	return secrets, nil
}

// Update updates a secret value
func (m *Manager) Update(ctx context.Context, secretID, newValue string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	secret, ok := m.secrets[secretID]
	if !ok {
		return fmt.Errorf("secret not found: %s", secretID)
	}

	// Encrypt new value
	encrypted, err := m.encrypt(newValue)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}

	secret.Value = encrypted
	secret.UpdatedAt = time.Now()

	return nil
}

// Delete removes a secret
func (m *Manager) Delete(ctx context.Context, secretID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.secrets[secretID]; !ok {
		return fmt.Errorf("secret not found: %s", secretID)
	}

	delete(m.secrets, secretID)
	return nil
}

// encrypt encrypts a value using AES-GCM
func (m *Manager) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(m.encKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a value using AES-GCM
func (m *Manager) decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(m.encKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// InjectIntoEnv injects secrets into an environment map
func (m *Manager) InjectIntoEnv(ctx context.Context, env map[string]string, secretNames []string, workspaceID string) error {
	for _, name := range secretNames {
		secret, err := m.GetByName(ctx, workspaceID, name)
		if err != nil {
			return fmt.Errorf("failed to get secret %s: %w", name, err)
		}

		value, err := m.decrypt(secret.Value)
		if err != nil {
			return fmt.Errorf("failed to decrypt secret %s: %w", name, err)
		}

		// Add to environment
		env[name] = value
	}

	return nil
}
