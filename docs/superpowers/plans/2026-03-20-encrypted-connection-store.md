# Encrypted Connection Store Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the read-only ConfigMap-based connection config with a K8s Secret-backed encrypted store, so connections can be managed via UI in production.

**Architecture:** On K8s, a new `K8sStore` (same `Store` interface as `FileStore`) encrypts connection data with AES-256-GCM and persists it in a K8s Secret. Crypto functions are extracted to a shared `internal/crypto` package. Helm chart is updated to remove connection ConfigMap and add RBAC for the connection Secret.

**Tech Stack:** Go 1.25, k8s.io/client-go, AES-256-GCM, Helm 3

**Spec:** `docs/superpowers/specs/2026-03-20-encrypted-connection-store-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/crypto/crypto.go` | Create | Shared AES-256-GCM Encrypt/Decrypt (moved from migration) |
| `internal/crypto/crypto_test.go` | Create | Encrypt/Decrypt tests (moved from migration) |
| `internal/config/k8s_store.go` | Create | K8sStore implementing Store interface via encrypted K8s Secret |
| `internal/config/k8s_store_test.go` | Create | K8sStore tests using fake.NewSimpleClientset() |
| `internal/migration/crypto.go` | Delete | Replaced by internal/crypto/crypto.go |
| `internal/migration/crypto_test.go` | Delete | Replaced by internal/crypto/crypto_test.go |
| `internal/migration/secret_store.go` | Modify | Update import from migration crypto to internal/crypto |
| `cmd/aap-server/main.go` | Modify | Use K8sStore on K8s, require AAP_ENCRYPTION_KEY |
| `charts/argocd-addons-platform/values.yaml` | Modify | Add connectionSecretName, AAP_ENCRYPTION_KEY |
| `charts/argocd-addons-platform/templates/deployment.yaml` | Modify | Remove config volume mount, add CONNECTION_SECRET_NAME env |
| `charts/argocd-addons-platform/templates/configmap.yaml` | Modify | Remove connection config entirely |
| `charts/argocd-addons-platform/templates/rbac.yaml` | Modify | Add connection Secret RBAC rules |
| `charts/argocd-addons-platform/templates/secret.yaml` | Modify | Auto-generate AAP_ENCRYPTION_KEY |
| `charts/argocd-addons-platform/values-production.yaml` | Modify | Remove config.connections, add connectionSecretName |

---

### Task 1: Extract crypto to shared package

**Files:**
- Create: `internal/crypto/crypto.go`
- Create: `internal/crypto/crypto_test.go`
- Modify: `internal/migration/secret_store.go`
- Delete: `internal/migration/crypto.go`
- Delete: `internal/migration/crypto_test.go`

- [ ] **Step 1: Create the shared crypto package**

Create `internal/crypto/crypto.go` — copy from `internal/migration/crypto.go` with package name changed:

```go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM with the given key.
// The key is hashed with SHA-256 to ensure it's always 32 bytes.
// Returns base64-encoded ciphertext (nonce prepended).
func Encrypt(plaintext []byte, key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("encryption key is empty")
	}

	keyHash := sha256.Sum256([]byte(key))

	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM.
func Decrypt(encoded string, key string) ([]byte, error) {
	if key == "" {
		return nil, fmt.Errorf("encryption key is empty")
	}

	keyHash := sha256.Sum256([]byte(key))

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decoding base64: %w", err)
	}

	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong key?): %w", err)
	}

	return plaintext, nil
}
```

- [ ] **Step 2: Create crypto tests**

Create `internal/crypto/crypto_test.go` — copy from `internal/migration/crypto_test.go` with package name changed to `crypto` and imports updated:

```go
package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := "my-secret-encryption-key-2026"
	plaintext := []byte(`{"token":"super-secret","server":"https://argocd.example.com"}`)

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if encrypted == string(plaintext) {
		t.Fatal("encrypted text should differ from plaintext")
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("expected %s, got %s", plaintext, decrypted)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key := "correct-key"
	plaintext := []byte("secret data")

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, "wrong-key")
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestEncryptEmptyKey(t *testing.T) {
	_, err := Encrypt([]byte("data"), "")
	if err == nil {
		t.Fatal("expected error with empty key")
	}
}

func TestDecryptEmptyKey(t *testing.T) {
	_, err := Decrypt("somedata", "")
	if err == nil {
		t.Fatal("expected error with empty key")
	}
}

func TestEncryptProducesDifferentCiphertext(t *testing.T) {
	key := "my-key"
	plaintext := []byte("same data")

	enc1, _ := Encrypt(plaintext, key)
	enc2, _ := Encrypt(plaintext, key)

	if enc1 == enc2 {
		t.Fatal("two encryptions of same data should differ (random nonce)")
	}

	dec1, _ := Decrypt(enc1, key)
	dec2, _ := Decrypt(enc2, key)
	if string(dec1) != string(dec2) {
		t.Fatal("both should decrypt to same value")
	}
}
```

- [ ] **Step 3: Run crypto tests**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/crypto/ -v`
Expected: All 5 tests PASS

- [ ] **Step 4: Update migration/secret_store.go imports**

In `internal/migration/secret_store.go`, change the `Encrypt`/`Decrypt` calls to use the shared package:

Add import: `"github.com/moran/argocd-addons-platform/internal/crypto"`

Change line 70: `encrypted, err := Encrypt(plaintext, ss.encryptionKey)` → `encrypted, err := crypto.Encrypt(plaintext, ss.encryptionKey)`
Change line 115: `plaintext, err := Decrypt(string(data), ss.encryptionKey)` → `plaintext, err := crypto.Decrypt(string(data), ss.encryptionKey)`

- [ ] **Step 5: Delete old migration crypto files**

Delete: `internal/migration/crypto.go`
Delete: `internal/migration/crypto_test.go`

- [ ] **Step 6: Verify all tests pass**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/crypto/ ./internal/migration/ -v`
Expected: All tests pass. Build succeeds.

- [ ] **Step 7: Commit**

```bash
git add internal/crypto/ internal/migration/crypto.go internal/migration/crypto_test.go internal/migration/secret_store.go
git commit -m "refactor: extract crypto to shared internal/crypto package"
```

---

### Task 2: Implement K8sStore

**Files:**
- Create: `internal/config/k8s_store.go`
- Create: `internal/config/k8s_store_test.go`

- [ ] **Step 1: Write K8sStore tests**

Create `internal/config/k8s_store_test.go`:

```go
package config

import (
	"testing"

	"github.com/moran/argocd-addons-platform/internal/models"
	"k8s.io/client-go/kubernetes/fake"
)

const testKey = "test-encryption-key-32chars-ok"
const testNS = "test-ns"
const testSecretName = "aap-connections"

func newTestK8sStore(t *testing.T) *K8sStore {
	t.Helper()
	clientset := fake.NewSimpleClientset()
	store, err := newK8sStoreWithClient(clientset, testNS, testSecretName, testKey)
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}
	return store
}

func TestK8sStore_SaveAndList(t *testing.T) {
	store := newTestK8sStore(t)

	conn := models.Connection{
		Name: "prod",
		Git: models.GitRepoConfig{
			Provider: models.GitProviderGitHub,
			Owner:    "my-org",
			Repo:     "my-repo",
			Token:    "ghp_secret123",
		},
		Argocd: models.ArgocdConfig{
			Namespace: "argocd",
			Insecure:  true,
		},
	}

	if err := store.SaveConnection(conn); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	conns, err := store.ListConnections()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].Name != "prod" {
		t.Errorf("expected name 'prod', got %q", conns[0].Name)
	}
	if conns[0].Git.Token != "ghp_secret123" {
		t.Errorf("token should be preserved, got %q", conns[0].Git.Token)
	}
}

func TestK8sStore_GetConnection(t *testing.T) {
	store := newTestK8sStore(t)

	conn := models.Connection{Name: "test", Git: models.GitRepoConfig{Provider: "github"}}
	_ = store.SaveConnection(conn)

	got, err := store.GetConnection("test")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected connection, got nil")
	}
	if got.Name != "test" {
		t.Errorf("expected 'test', got %q", got.Name)
	}

	// Non-existent
	got, err = store.GetConnection("nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent connection")
	}
}

func TestK8sStore_DeleteConnection(t *testing.T) {
	store := newTestK8sStore(t)

	_ = store.SaveConnection(models.Connection{Name: "a"})
	_ = store.SaveConnection(models.Connection{Name: "b"})

	if err := store.DeleteConnection("a"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	conns, _ := store.ListConnections()
	if len(conns) != 1 {
		t.Fatalf("expected 1, got %d", len(conns))
	}
	if conns[0].Name != "b" {
		t.Errorf("expected 'b', got %q", conns[0].Name)
	}
}

func TestK8sStore_DeleteNonExistent(t *testing.T) {
	store := newTestK8sStore(t)
	if err := store.DeleteConnection("nope"); err == nil {
		t.Fatal("expected error deleting non-existent connection")
	}
}

func TestK8sStore_ActiveConnection(t *testing.T) {
	store := newTestK8sStore(t)

	_ = store.SaveConnection(models.Connection{Name: "conn1"})
	_ = store.SaveConnection(models.Connection{Name: "conn2"})

	// First connection should be auto-active
	active, err := store.GetActiveConnection()
	if err != nil {
		t.Fatalf("get active failed: %v", err)
	}
	if active != "conn1" {
		t.Errorf("expected 'conn1', got %q", active)
	}

	// Set different active
	if err := store.SetActiveConnection("conn2"); err != nil {
		t.Fatalf("set active failed: %v", err)
	}
	active, _ = store.GetActiveConnection()
	if active != "conn2" {
		t.Errorf("expected 'conn2', got %q", active)
	}
}

func TestK8sStore_SetActiveNonExistent(t *testing.T) {
	store := newTestK8sStore(t)
	if err := store.SetActiveConnection("nope"); err == nil {
		t.Fatal("expected error setting non-existent connection as active")
	}
}

func TestK8sStore_FirstConnectionBecomesDefaultAndActive(t *testing.T) {
	store := newTestK8sStore(t)

	_ = store.SaveConnection(models.Connection{Name: "first"})

	conns, _ := store.ListConnections()
	if !conns[0].IsDefault {
		t.Error("first connection should be default")
	}

	active, _ := store.GetActiveConnection()
	if active != "first" {
		t.Errorf("first connection should be active, got %q", active)
	}
}

func TestK8sStore_UpdateExisting(t *testing.T) {
	store := newTestK8sStore(t)

	_ = store.SaveConnection(models.Connection{
		Name: "prod",
		Git:  models.GitRepoConfig{Token: "old-token"},
	})
	_ = store.SaveConnection(models.Connection{
		Name: "prod",
		Git:  models.GitRepoConfig{Token: "new-token"},
	})

	conns, _ := store.ListConnections()
	if len(conns) != 1 {
		t.Fatalf("expected 1, got %d", len(conns))
	}
	if conns[0].Git.Token != "new-token" {
		t.Errorf("expected 'new-token', got %q", conns[0].Git.Token)
	}
}

func TestK8sStore_PersistsAcrossInstances(t *testing.T) {
	// Simulate pod restart: create store, save, create new store with same client
	clientset := fake.NewSimpleClientset()
	store1, _ := newK8sStoreWithClient(clientset, testNS, testSecretName, testKey)

	_ = store1.SaveConnection(models.Connection{Name: "persisted", Git: models.GitRepoConfig{Token: "secret"}})

	// New store instance (simulates new pod reading existing Secret)
	store2, _ := newK8sStoreWithClient(clientset, testNS, testSecretName, testKey)
	conns, err := store2.ListConnections()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(conns) != 1 || conns[0].Name != "persisted" {
		t.Fatal("connection should persist across store instances")
	}
	if conns[0].Git.Token != "secret" {
		t.Error("token should be preserved")
	}
}

func TestK8sStore_WrongKeyReturnsError(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	store1, _ := newK8sStoreWithClient(clientset, testNS, testSecretName, "key-one")
	_ = store1.SaveConnection(models.Connection{Name: "test"})

	store2, _ := newK8sStoreWithClient(clientset, testNS, testSecretName, "key-two")
	_, err := store2.ListConnections()
	if err == nil {
		t.Fatal("expected error reading with wrong key")
	}
}

func TestK8sStore_EmptySecretReturnsEmpty(t *testing.T) {
	store := newTestK8sStore(t)

	conns, err := store.ListConnections()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 0 {
		t.Errorf("expected 0 connections, got %d", len(conns))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/config/ -run TestK8sStore -v`
Expected: FAIL — `K8sStore` not defined

- [ ] **Step 3: Implement K8sStore**

Create `internal/config/k8s_store.go`:

```go
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moran/argocd-addons-platform/internal/crypto"
	"github.com/moran/argocd-addons-platform/internal/models"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	connectionsDataKey = "connections"
	activeDataKey      = "active"
	k8sTimeout         = 10 * time.Second
)

// K8sStore implements Store using an encrypted K8s Secret.
// Used in production Kubernetes deployments.
type K8sStore struct {
	client        kubernetes.Interface
	namespace     string
	secretName    string
	encryptionKey string
	mu            sync.RWMutex
}

// NewK8sStore creates a K8s Secret-backed connection store.
// Creates its own in-cluster K8s client.
func NewK8sStore(namespace, secretName, encryptionKey string) (*K8sStore, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("creating in-cluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating k8s client: %w", err)
	}
	return newK8sStoreWithClient(clientset, namespace, secretName, encryptionKey)
}

// newK8sStoreWithClient creates a K8sStore with an injected client (for testing).
func newK8sStoreWithClient(client kubernetes.Interface, namespace, secretName, encryptionKey string) (*K8sStore, error) {
	return &K8sStore{
		client:        client,
		namespace:     namespace,
		secretName:    secretName,
		encryptionKey: encryptionKey,
	}, nil
}

// connData is the internal structure stored in the Secret.
type connData struct {
	Connections      []models.Connection `json:"connections"`
	ActiveConnection string              `json:"active_connection,omitempty"`
}

func (s *K8sStore) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), k8sTimeout)
}

// load reads and decrypts connections from the K8s Secret.
// Caller must hold s.mu (read or write).
func (s *K8sStore) load() (*connData, string, error) {
	ctx, cancel := s.ctx()
	defer cancel()

	secret, err := s.client.CoreV1().Secrets(s.namespace).Get(ctx, s.secretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return &connData{}, "", nil
		}
		return nil, "", fmt.Errorf("reading connection secret: %w", err)
	}

	resourceVersion := secret.ResourceVersion

	// Read active connection (plaintext)
	data := &connData{}
	if v, ok := secret.Data[activeDataKey]; ok {
		data.ActiveConnection = string(v)
	}

	// Read and decrypt connections
	encData, ok := secret.Data[connectionsDataKey]
	if !ok || len(encData) == 0 {
		return data, resourceVersion, nil
	}

	plaintext, err := crypto.Decrypt(string(encData), s.encryptionKey)
	if err != nil {
		return nil, "", fmt.Errorf("decrypting connections: %w", err)
	}

	if err := json.Unmarshal(plaintext, &data.Connections); err != nil {
		return nil, "", fmt.Errorf("decoding connections: %w", err)
	}

	return data, resourceVersion, nil
}

// save encrypts and writes connections to the K8s Secret.
// Caller must hold s.mu.
func (s *K8sStore) save(data *connData) error {
	ctx, cancel := s.ctx()
	defer cancel()

	connJSON, err := json.Marshal(data.Connections)
	if err != nil {
		return fmt.Errorf("marshaling connections: %w", err)
	}

	encrypted, err := crypto.Encrypt(connJSON, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("encrypting connections: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.secretName,
			Namespace: s.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "aap",
				"app.kubernetes.io/component":  "connection-config",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			connectionsDataKey: []byte(encrypted),
			activeDataKey:      []byte(data.ActiveConnection),
		},
	}

	// Try update first, then create if not found
	_, err = s.client.CoreV1().Secrets(s.namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			_, err = s.client.CoreV1().Secrets(s.namespace).Create(ctx, secret, metav1.CreateOptions{})
			if err != nil {
				if k8serrors.IsAlreadyExists(err) {
					// Race condition: another pod created it — retry update
					_, err = s.client.CoreV1().Secrets(s.namespace).Update(ctx, secret, metav1.UpdateOptions{})
				}
			}
		}
		if err != nil {
			return fmt.Errorf("saving connection secret: %w", err)
		}
	}

	slog.Info("connections saved to K8s Secret", "namespace", s.namespace, "secret", s.secretName)
	return nil
}

func (s *K8sStore) ListConnections() ([]models.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, _, err := s.load()
	if err != nil {
		return nil, err
	}
	return data.Connections, nil
}

func (s *K8sStore) GetConnection(name string) (*models.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, _, err := s.load()
	if err != nil {
		return nil, err
	}

	for i := range data.Connections {
		if data.Connections[i].Name == name {
			return &data.Connections[i], nil
		}
	}
	return nil, nil
}

func (s *K8sStore) SaveConnection(conn models.Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, _, err := s.load()
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Update existing or append new
	found := false
	for i := range data.Connections {
		if data.Connections[i].Name == conn.Name {
			conn.UpdatedAt = now
			if conn.CreatedAt == "" {
				conn.CreatedAt = data.Connections[i].CreatedAt
			}
			data.Connections[i] = conn
			found = true
			break
		}
	}

	if !found {
		conn.CreatedAt = now
		conn.UpdatedAt = now
		data.Connections = append(data.Connections, conn)
	}

	// If this is the default, unset others
	if conn.IsDefault {
		for i := range data.Connections {
			if data.Connections[i].Name != conn.Name {
				data.Connections[i].IsDefault = false
			}
		}
	}

	// If this is the first connection, make it default and active
	if len(data.Connections) == 1 {
		data.Connections[0].IsDefault = true
		data.ActiveConnection = data.Connections[0].Name
	}

	return s.save(data)
}

func (s *K8sStore) DeleteConnection(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, _, err := s.load()
	if err != nil {
		return err
	}

	connections := make([]models.Connection, 0, len(data.Connections))
	for _, c := range data.Connections {
		if c.Name != name {
			connections = append(connections, c)
		}
	}

	if len(connections) == len(data.Connections) {
		return fmt.Errorf("connection %q not found", name)
	}

	data.Connections = connections

	if data.ActiveConnection == name {
		data.ActiveConnection = ""
		if len(data.Connections) > 0 {
			data.ActiveConnection = data.Connections[0].Name
		}
	}

	return s.save(data)
}

func (s *K8sStore) GetActiveConnection() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, _, err := s.load()
	if err != nil {
		return "", err
	}

	if data.ActiveConnection != "" {
		return data.ActiveConnection, nil
	}

	// Fall back to default connection
	for _, c := range data.Connections {
		if c.IsDefault {
			return c.Name, nil
		}
	}

	// Fall back to first connection
	if len(data.Connections) > 0 {
		return data.Connections[0].Name, nil
	}

	return "", nil
}

func (s *K8sStore) SetActiveConnection(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, _, err := s.load()
	if err != nil {
		return err
	}

	// Verify connection exists
	found := false
	for _, c := range data.Connections {
		if c.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("connection %q not found", name)
	}

	data.ActiveConnection = name
	return s.save(data)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/config/ -run TestK8sStore -v`
Expected: All 11 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/k8s_store.go internal/config/k8s_store_test.go
git commit -m "feat: add K8sStore for encrypted connection storage"
```

---

### Task 3: Wire K8sStore into main.go

**Files:**
- Modify: `cmd/aap-server/main.go:86-103`

- [ ] **Step 1: Update main.go store initialization**

Replace the store initialization block (lines 90-103) with:

```go
	// Initialize config store
	var store config.Store
	switch mode {
	case platform.ModeKubernetes:
		encKey := os.Getenv("AAP_ENCRYPTION_KEY")
		if encKey == "" {
			log.Fatal("FATAL: AAP_ENCRYPTION_KEY is required when running on Kubernetes. " +
				"Set it in your Helm values (secrets.AAP_ENCRYPTION_KEY) or existingSecret.")
		}
		secretName := os.Getenv("CONNECTION_SECRET_NAME")
		if secretName == "" {
			secretName = "aap-connections"
		}
		namespace := os.Getenv("AAP_NAMESPACE")
		if namespace == "" {
			namespace = "argocd-addons-platform"
		}
		var err error
		store, err = config.NewK8sStore(namespace, secretName, encKey)
		if err != nil {
			log.Fatalf("Failed to create K8s connection store: %v", err)
		}
		log.Printf("Connection config stored in encrypted K8s Secret: %s/%s", namespace, secretName)
	default:
		store = config.NewFileStore(*configPath)
	}
```

Also remove the now-unused AAP_CONFIG override (lines 76-78) since K8s mode no longer uses a config file path:

Keep the `AAP_CONFIG` env handling for local mode only — it's still valid there. The `configPath` variable is still used in the `default` case. No change needed to that section.

- [ ] **Step 2: Verify build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./cmd/aap-server/`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add cmd/aap-server/main.go
git commit -m "feat: use K8sStore for connections in Kubernetes mode"
```

---

### Task 4: Update Helm chart — RBAC & Secret

**Files:**
- Modify: `charts/argocd-addons-platform/templates/rbac.yaml:61-79`
- Modify: `charts/argocd-addons-platform/templates/secret.yaml`
- Modify: `charts/argocd-addons-platform/values.yaml`

- [ ] **Step 1: Add connectionSecretName to values.yaml**

Add under the `config:` section (after `config.connections`, around line 50):

```yaml
  # K8s Secret name for runtime connection config (managed by app, not Helm)
  connectionSecretName: "aap-connections"
```

Add `AAP_ENCRYPTION_KEY` to the `secrets:` map (around line 62):

```yaml
secrets:
  GITHUB_TOKEN: ""
  # ARGOCD_TOKEN: ""  # Not needed in in-cluster mode with RBAC
  # AAP_ENCRYPTION_KEY: ""  # Auto-generated on first install if empty
```

- [ ] **Step 2: Update RBAC**

In `charts/argocd-addons-platform/templates/rbac.yaml`, add a rule for the connection Secret in the Role (around line 67, after the migration-settings rule):

```yaml
  - apiGroups: [""]
    resources: ["secrets"]
    resourceNames: [{{ .Values.config.connectionSecretName | default "aap-connections" | quote }}]
    verbs: ["get", "update"]
```

The existing `create` rule on line 72 (`verbs: ["create"]` without `resourceNames`) already covers Secret creation.

- [ ] **Step 3: Auto-generate AAP_ENCRYPTION_KEY in secret.yaml**

In `charts/argocd-addons-platform/templates/secret.yaml`, add auto-generation logic. After the `admin.password`/`admin.initialPassword` block (around line 26), add:

```yaml
  {{- if not (index ($existingSecret.data | default dict) "AAP_ENCRYPTION_KEY") }}
  AAP_ENCRYPTION_KEY: {{ randAlphaNum 32 | quote }}
  {{- else }}
  AAP_ENCRYPTION_KEY: {{ index $existingSecret.data "AAP_ENCRYPTION_KEY" | b64dec | quote }}
  {{- end }}
```

For the first-install case (no existing secret, around line 23), also add:

```yaml
  AAP_ENCRYPTION_KEY: {{ randAlphaNum 32 | quote }}
```

- [ ] **Step 4: Verify Helm template renders**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && helm template test charts/argocd-addons-platform/ 2>&1 | head -200`
Expected: Template renders without errors. The Secret should contain `AAP_ENCRYPTION_KEY`. The Role should have the connection Secret rule.

- [ ] **Step 5: Commit**

```bash
git add charts/argocd-addons-platform/templates/rbac.yaml charts/argocd-addons-platform/templates/secret.yaml charts/argocd-addons-platform/values.yaml
git commit -m "feat(helm): add RBAC and encryption key for connection Secret store"
```

---

### Task 5: Update Helm chart — deployment & configmap

**Files:**
- Modify: `charts/argocd-addons-platform/templates/deployment.yaml:41-42,99-102,122-124`
- Modify: `charts/argocd-addons-platform/templates/configmap.yaml`

- [ ] **Step 1: Update deployment.yaml**

1. Replace the `AAP_CONFIG` env var (line 41-42) with `CONNECTION_SECRET_NAME`:

```yaml
            - name: CONNECTION_SECRET_NAME
              value: {{ .Values.config.connectionSecretName | default "aap-connections" | quote }}
```

2. Remove the config volume mount (lines 100-102):

Remove:
```yaml
            - name: config
              mountPath: /etc/aap
              readOnly: true
```

3. Remove the config volume (lines 122-124):

Remove:
```yaml
        - name: config
          configMap:
            name: {{ include "aap.fullname" . }}-config
```

4. Remove the configmap checksum annotation (line 16):

Remove:
```yaml
        checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
```

- [ ] **Step 2: Gut configmap.yaml**

Replace the entire `configmap.yaml` content with an empty/minimal file (keep it for potential future non-connection config, but remove all connection data):

```yaml
{{/* Connection config is now stored in an encrypted K8s Secret managed by the app. */}}
{{/* This ConfigMap is preserved for potential future non-sensitive configuration. */}}
```

- [ ] **Step 3: Verify Helm template**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && helm template test charts/argocd-addons-platform/ 2>&1 | grep -A5 "CONNECTION_SECRET_NAME"`
Expected: Shows the env var set correctly. No config volume mount or ConfigMap for connections.

- [ ] **Step 4: Commit**

```bash
git add charts/argocd-addons-platform/templates/deployment.yaml charts/argocd-addons-platform/templates/configmap.yaml
git commit -m "feat(helm): remove connection ConfigMap, use K8s Secret store"
```

---

### Task 6: Clean up values.yaml and values-production.yaml

**Files:**
- Modify: `charts/argocd-addons-platform/values.yaml:35-50`
- Modify: `charts/argocd-addons-platform/values-production.yaml:11-25`

- [ ] **Step 1: Remove config.connections from values.yaml**

Replace the `config.connections` block (lines 35-50) with a comment explaining the new approach:

```yaml
config:
  # Connections are managed via the Settings UI and stored in an encrypted K8s Secret.
  # The Secret name can be customized below. On first deploy, open the UI to configure.
  connectionSecretName: "aap-connections"
```

- [ ] **Step 2: Update values-production.yaml**

Replace the `config:` block (lines 11-25) with:

```yaml
config:
  connectionSecretName: "aap-connections"
```

This removes the old connection config (owner, repo, token, argocd server_url, etc.) — those are now entered via Settings UI and stored encrypted.

- [ ] **Step 3: Verify Helm template still renders**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && helm template test charts/argocd-addons-platform/ -f charts/argocd-addons-platform/values-production.yaml 2>&1 | tail -20`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add charts/argocd-addons-platform/values.yaml charts/argocd-addons-platform/values-production.yaml
git commit -m "feat(helm): simplify values — connections managed via UI"
```

---

### Task 7: Final integration verification

- [ ] **Step 1: Run all tests**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./... 2>&1`
Expected: All tests pass

- [ ] **Step 2: Verify build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./cmd/aap-server/`
Expected: Clean build

- [ ] **Step 3: Run gosec (if available)**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && which gosec && gosec ./internal/crypto/ ./internal/config/ 2>&1 || echo "gosec not installed, skip"`
Expected: No new findings (or not installed)

- [ ] **Step 4: Verify Helm lint**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && helm lint charts/argocd-addons-platform/`
Expected: No errors

- [ ] **Step 5: Commit if any fixups needed**

Only if earlier steps revealed issues that needed fixing.
