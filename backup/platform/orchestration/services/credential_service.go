package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// CredentialService handles credential storage and retrieval
type CredentialService struct {
	encryptionKey []byte
}

// NewCredentialService creates a new credential service
func NewCredentialService(encryptionKey string) (*CredentialService, error) {
	if encryptionKey == "" {
		encryptionKey = os.Getenv("ENCRYPTION_KEY")
	}
	if encryptionKey == "" {
		return nil, fmt.Errorf("encryption key not provided")
	}

	// Ensure key is 32 bytes for AES-256
	key := []byte(encryptionKey)
	if len(key) < 32 {
		// Pad key to 32 bytes
		paddedKey := make([]byte, 32)
		copy(paddedKey, key)
		key = paddedKey
	} else if len(key) > 32 {
		key = key[:32]
	}

	return &CredentialService{
		encryptionKey: key,
	}, nil
}

// Credential represents stored credentials
type Credential struct {
	CredentialType string                 `json:"credential_type"`
	Payload        map[string]interface{} `json:"payload"`
}

// GetCredential retrieves and optionally decrypts credentials
func (cs *CredentialService) GetCredential(key string, decrypt bool) (*Credential, error) {
	// TODO: Implement credential retrieval from secure store (e.g., HashiCorp Vault, AWS Secrets Manager)
	// For now, return mock credential for development
	return &Credential{
		CredentialType: "password_authentication",
		Payload: map[string]interface{}{
			"user":            "admin",
			"pass":            "password",
			"enable_password": "enable",
		},
	}, nil
}

// StoreCredential encrypts and stores credentials
func (cs *CredentialService) StoreCredential(key string, cred *Credential) error {
	data, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("failed to marshal credential: %w", err)
	}

	encrypted, err := cs.encrypt(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt credential: %w", err)
	}

	// TODO: Store encrypted credential in secure storage
	_ = encrypted

	return nil
}

// encrypt encrypts data using AES-GCM
func (cs *CredentialService) encrypt(data []byte) (string, error) {
	block, err := aes.NewCipher(cs.encryptionKey)
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

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts data using AES-GCM
func (cs *CredentialService) decrypt(encrypted string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(cs.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

