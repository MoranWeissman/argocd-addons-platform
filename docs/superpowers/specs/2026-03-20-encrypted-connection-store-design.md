# Encrypted Connection Store — Design Spec

## Goal

Replace the read-only ConfigMap-based connection config with a K8s Secret-backed encrypted store. Connections are configured exclusively via the Settings UI. Helm provides only the Secret name and encryption key. This follows K8s security best practices: secrets in Secrets, least-privilege RBAC, encryption at rest.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Settings UI                         │
│  Configure connections (Git provider, ArgoCD, tokens)   │
└─────────────────┬───────────────────────────────────────┘
                  │ PUT /api/v1/connections/{name}
                  ▼
┌─────────────────────────────────────────────────────────┐
│              ConnectionService                          │
│  Business logic, validation, token masking              │
└─────────────────┬───────────────────────────────────────┘
                  │ Store interface (unchanged)
                  ▼
┌─────────────────────────────────────────────────────────┐
│  K8s: K8sStore              │  Local: FileStore         │
│  ─────────────              │  ──────────────           │
│  AES-256-GCM encrypt        │  Plain YAML file          │
│  K8s Secret read/write      │  (dev only, not prod)     │
│  RBAC-scoped                │                           │
└─────────────────────────────┴───────────────────────────┘
```

### Mode Detection

`platform.Detect()` already distinguishes K8s vs local. On K8s, `main.go` creates `K8sStore`. On local dev, `FileStore` (unchanged).

## Security Design

### Encryption

- **Algorithm:** AES-256-GCM (authenticated encryption — integrity + confidentiality)
- **Key derivation:** SHA-256 hash of `AAP_ENCRYPTION_KEY` env var (matches existing `internal/migration/crypto.go`). Note: SHA-256 is a fast hash, not a KDF like PBKDF2/Argon2. Acceptable here because the key is operator-provisioned (not a user password) and behind K8s RBAC. Future improvement: switch to Argon2id.
- **What's encrypted:** The entire connections JSON blob, stored as a single opaque value in the K8s Secret
- **Key rotation:** Change `AAP_ENCRYPTION_KEY`, then delete the Secret via `kubectl delete secret aap-connections` (requires cluster admin — the app's RBAC intentionally excludes `delete`). Re-enter connections via UI. Connections are unavailable between deletion and re-entry. Future: re-encryption migration tool.

### K8s Secret Security

- **Secret type:** `Opaque` (standard for application-managed secrets)
- **Namespace-scoped:** Secret lives in the release namespace only
- **RBAC:** ServiceAccount gets read/write only on the specific Secret by `resourceNames` restriction
- **etcd encryption at rest:** Relies on cluster-level `EncryptionConfiguration` (standard EKS/GKE practice). Our application-level AES-256-GCM provides defense-in-depth even if etcd encryption is not enabled.
- **No plaintext tokens in ConfigMaps:** Tokens never appear in ConfigMaps, env vars, or logs
- **Helm never overwrites:** Helm does not manage this Secret. It's created by the app on first connection save.

### RBAC (Principle of Least Privilege)

```yaml
# Added to existing Role in rbac.yaml
# Rule 1: read/write the specific named Secret
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["{{ connectionSecretName }}"]
  verbs: ["get", "update"]
# Rule 2: create permission (resourceNames cannot restrict create in K8s)
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["create"]
```

No `list`, no `delete`, no wildcard. `create` needs a separate rule without `resourceNames` because K8s RBAC cannot restrict `create` by name (the resource doesn't exist yet at admission time). This matches the existing pattern in `rbac.yaml` line 72.

### Startup Validation

- **No `AAP_ENCRYPTION_KEY`:** Fatal error on K8s. App refuses to start with clear message:
  `"FATAL: AAP_ENCRYPTION_KEY is required when running on Kubernetes"`
- **Secret doesn't exist:** Not an error. Created on first connection save.
- **Secret exists but undecryptable (wrong key):** Error returned to UI. Does not silently return empty.

## Components

### New: `internal/config/k8s_store.go`

Implements the existing `Store` interface (from `internal/config/store.go`):

```go
type Store interface {
    ListConnections() ([]models.Connection, error)
    GetConnection(name string) (*models.Connection, error)
    SaveConnection(conn models.Connection) error
    DeleteConnection(name string) error
    GetActiveConnection() (string, error)
    SetActiveConnection(name string) error
}
```

**Context strategy:** The `Store` interface does not accept `context.Context` (changing it would break all consumers). The `K8sStore` uses `context.Background()` internally for K8s API calls, with a 10-second timeout per operation. This matches the pragmatic approach — connection CRUD is always user-initiated, not part of request pipelines that need cancellation propagation.

Implementation details:
- Uses `k8s.io/client-go` (already a dependency for migration stores)
- Creates its own K8s client internally via `rest.InClusterConfig()` (same pattern as `migration/secret_store.go`)
- Thread-safe with `sync.RWMutex` for in-memory cache
- Uses K8s `resourceVersion` for optimistic concurrency on writes (not just mutex)
- On `Create` conflict (409 AlreadyExists): retry as `Update` to handle pod rolling update race
- Secret data key: `"connections"` (encrypted JSON of all connections)
- Secret data key: `"active"` (plain text — connection name is not sensitive, keeping it plaintext avoids decrypt overhead for read-only active-connection lookups)
- Reuses `internal/migration/crypto.go` Encrypt/Decrypt functions (move to shared `internal/crypto/` package)

### Modified: `cmd/aap-server/main.go`

```go
var store config.Store
if runtime == platform.Kubernetes {
    encKey := os.Getenv("AAP_ENCRYPTION_KEY")
    if encKey == "" {
        log.Fatal("AAP_ENCRYPTION_KEY is required when running on Kubernetes")
    }
    secretName := os.Getenv("CONNECTION_SECRET_NAME")
    if secretName == "" {
        secretName = "aap-connections"
    }
    namespace := os.Getenv("AAP_NAMESPACE") // already set by Helm deployment
    // K8sStore creates its own in-cluster K8s client internally
    // (same pattern as migration/secret_store.go)
    var err error
    store, err = config.NewK8sStore(namespace, secretName, encKey)
    if err != nil {
        log.Fatalf("Failed to create K8s connection store: %v", err)
    }
} else {
    store = config.NewFileStore(configPath)
}
```

### Modified: Helm Chart

**`values.yaml`:**
```yaml
config:
  connectionSecretName: "aap-connections"  # K8s Secret name for runtime connection config
```

**`deployment.yaml`:**
- Remove: `config` volume mount (`/etc/aap/config.yaml`)
- Remove: `AAP_CONFIG` env var
- Add: `CONNECTION_SECRET_NAME` env var from values

**`configmap.yaml`:**
- Remove connection config block entirely
- Keep ConfigMap if other non-connection config exists, otherwise delete template

**`rbac.yaml`:**
- Add `resourceNames` rule for the connection Secret name

**`secret.yaml`:**
- Ensure `AAP_ENCRYPTION_KEY` is included in the app Secret
- Add to `values.secrets` map with empty default. In production, users supply via `existingSecret` or sealed-secrets. Helm auto-generates a random key on first install if not provided (same pattern as `admin.initialPassword`).

### Shared: `internal/crypto/crypto.go`

Move `Encrypt`/`Decrypt` from `internal/migration/crypto.go` to `internal/crypto/crypto.go`. Update imports in both migration and config packages. Single source of truth for encryption.

## Data Format

K8s Secret contents (after base64 decode by K8s):

```json
{
  "connections": "<AES-256-GCM encrypted base64 string>",
  "active": "my-connection-name"
}
```

Decrypted `connections` value:

```json
[
  {
    "name": "production",
    "description": "Production cluster",
    "is_default": true,
    "created_at": "2026-03-20T10:00:00Z",
    "updated_at": "2026-03-20T10:00:00Z",
    "git": {
      "provider": "github",
      "owner": "my-org",
      "repo": "argocd-cluster-addons",
      "token": "ghp_actual_token_here"
    },
    "argocd": {
      "server_url": "",
      "token": "",
      "namespace": "argocd",
      "insecure": true
    }
  }
]
```

## Error Handling

| Scenario | Behavior |
|---|---|
| No encryption key on K8s | Fatal startup error |
| Secret doesn't exist | Created on first save |
| Decryption fails (wrong key) | Return error to caller, log warning (no token data in log) |
| K8s API unreachable | Return error to caller, retry on next request |
| Concurrent writes | Mutex serializes in-process; K8s resourceVersion prevents conflicts |
| Empty/corrupt Secret data | Treat as empty connections list |

## What Gets Removed

1. **`config.yaml` ConfigMap** for connection data — no longer needed
2. **`${GITHUB_TOKEN}` env var expansion** — tokens stored directly in encrypted Secret
3. **`FileStore` usage on K8s** — replaced by `K8sStore`
4. **`AAP_CONFIG` env var** — no config file path needed on K8s

## What Stays the Same

1. **`Store` interface** — unchanged, all consumers (ConnectionService, API handlers) unaffected
2. **`FileStore` for local dev** — still reads/writes `config.yaml` file
3. **UI API contracts** — same REST endpoints, same request/response shapes
4. **Token masking in API responses** — ConnectionService still masks tokens before returning

## Testing Strategy

- Unit tests for `K8sStore` using `fake.NewSimpleClientset()` (standard K8s testing pattern)
- Unit tests for encrypt/decrypt round-trip
- Integration test: save → restart (new store instance) → read back
- Error cases: missing key, corrupt data, missing Secret
- Existing `FileStore` tests remain unchanged

## Migration Path

Existing deployments with ConfigMap-based connections:
1. Deploy new version — app starts with empty K8s Secret (no connections)
2. User re-enters connections via Settings UI
3. Old ConfigMap can be cleaned up manually or by Helm

No automatic migration. Connections are few (typically 1-2) and re-entering via UI takes 30 seconds. We intentionally do not auto-migrate tokens from the less-secure ConfigMap store — this ensures the new encrypted path is the only way secrets enter the system.
