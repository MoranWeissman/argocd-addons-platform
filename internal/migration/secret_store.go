package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/moran/argocd-addons-platform/internal/crypto"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	secretName = "aap-migration-settings"
	secretKey  = "settings.enc"
)

// SecretStore stores migration settings in a K8s Secret, encrypted with
// AES-256-GCM at the application level. Even if the Secret is read directly,
// credentials are ciphertext.
type SecretStore struct {
	client        kubernetes.Interface
	namespace     string
	encryptionKey string
}

// NewSecretStore creates a K8s Secret-backed settings store.
// Returns nil if not running in K8s (caller should fall back to file store).
func NewSecretStore() *SecretStore {
	encKey := os.Getenv("AAP_ENCRYPTION_KEY")
	if encKey == "" {
		slog.Warn("AAP_ENCRYPTION_KEY not set, migration settings will use file store (unencrypted)")
		return nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		slog.Info("not in k8s cluster, migration settings will use file store")
		return nil
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		slog.Error("failed to create k8s client for migration settings", "error", err)
		return nil
	}

	namespace := os.Getenv("AAP_NAMESPACE")
	if namespace == "" {
		namespace = "argocd-addons-platform"
	}

	slog.Info("migration settings will be stored encrypted in K8s Secret", "namespace", namespace, "secret", secretName)
	return &SecretStore{
		client:        clientset,
		namespace:     namespace,
		encryptionKey: encKey,
	}
}

// SaveSettings encrypts and stores migration settings in a K8s Secret.
func (ss *SecretStore) SaveSettings(ctx context.Context, settings *MigrationSettings) error {
	plaintext, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	encrypted, err := crypto.Encrypt(plaintext, ss.encryptionKey)
	if err != nil {
		return fmt.Errorf("encrypting settings: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ss.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "aap",
				"app.kubernetes.io/component":  "migration-settings",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			secretKey: []byte(encrypted),
		},
	}

	// Try update first, create if not exists
	_, err = ss.client.CoreV1().Secrets(ss.namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		_, err = ss.client.CoreV1().Secrets(ss.namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("creating/updating migration settings secret: %w", err)
		}
	}

	slog.Info("migration settings saved to K8s Secret", "namespace", ss.namespace)
	return nil
}

// GetSettings reads and decrypts migration settings from the K8s Secret.
func (ss *SecretStore) GetSettings(ctx context.Context) (*MigrationSettings, error) {
	secret, err := ss.client.CoreV1().Secrets(ss.namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return &MigrationSettings{}, nil // not found = not configured
	}

	data, ok := secret.Data[secretKey]
	if !ok {
		return &MigrationSettings{}, nil
	}

	plaintext, err := crypto.Decrypt(string(data), ss.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decrypting migration settings: %w", err)
	}

	var settings MigrationSettings
	if err := json.Unmarshal(plaintext, &settings); err != nil {
		return nil, fmt.Errorf("decoding migration settings: %w", err)
	}
	return &settings, nil
}

// DeleteSettings removes the migration settings K8s Secret.
func (ss *SecretStore) DeleteSettings(ctx context.Context) error {
	err := ss.client.CoreV1().Secrets(ss.namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("deleting migration settings secret: %w", err)
	}
	slog.Info("migration settings secret deleted", "namespace", ss.namespace)
	return nil
}
