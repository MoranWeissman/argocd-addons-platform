# Security

This document describes the security architecture, implemented controls, and known considerations for the ArgoCD Addons Platform (AAP).

---

## Authentication

### K8s-Native Auth (Production)

AAP uses Kubernetes-native resources for user management, mirroring ArgoCD's approach:

- **ConfigMap `{release}-users`**: Defines user accounts (username, enabled, role)
- **Secret `{release}`**: Stores bcrypt password hashes as `{username}.password` keys

**Initial setup:**
1. Helm generates a random 16-character admin password on first install
2. The bcrypt hash is stored in the Secret as `admin.password`
3. The plaintext is stored as `admin.initialPassword` (removed after first password change)
4. Retrieve it with:
   ```bash
   kubectl get secret <release> -n <namespace> -o jsonpath='{.data.admin\.initialPassword}' | base64 -d
   ```

**Password changes:**
- Done via the User Info page in the UI
- New bcrypt hash is written directly to the K8s Secret via the K8s API
- Persists across pod restarts and Helm upgrades (Helm preserves existing password data)
- The `admin.initialPassword` key is deleted after the first password change

**RBAC:**
- ServiceAccount has a scoped Role: `get`/`update` on its own Secret, `get` on the users ConfigMap
- No cluster-wide secret access

### Local Dev Mode (Fallback)

When not running in K8s (local development), falls back to environment variables:
- `AAP_AUTH_USER` — username
- `AAP_AUTH_PASSWORD` — password (plaintext or bcrypt hash)

### Session Management

- Login returns a cryptographically random 64-character hex token
- Token valid for 24 hours
- Stored in browser `sessionStorage` (cleared on tab close)
- All API calls include `Authorization: Bearer <token>` header
- 401 responses auto-redirect to login page

### Rate Limiting

- Login endpoint (`POST /api/v1/auth/login`) is rate-limited to 10 attempts per IP per minute
- Returns HTTP 429 when exceeded
- Uses `X-Forwarded-For` header for IP detection (behind ALB)
- In-memory tracking with automatic cleanup of expired entries

### Auth Bypass Warning

If no users are configured (no ConfigMap, no env vars), authentication is disabled entirely. The server logs a loud warning on startup:
```
WARNING: Authentication is DISABLED — all API endpoints are publicly accessible
```

---

## API Security

### Token-Based Access Control

All `/api/v1/*` endpoints require a valid Bearer token, except:
- `GET /api/v1/health` — K8s liveness/readiness probes
- `POST /api/v1/auth/login` — authentication endpoint
- All non-API paths (static files served by the SPA)

### CORS Policy

- Default: same-origin only (no cross-origin requests allowed)
- Configurable via `AAP_CORS_ORIGIN` env var:
  - Empty (default): same-origin only
  - Specific URL: only that origin allowed
  - `*`: all origins (development only)

### Security Headers

All responses include:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY` (prevents clickjacking)
- `Referrer-Policy: strict-origin-when-cross-origin`

### Input Validation

- Datadog namespace and cluster name parameters validated against `^[a-zA-Z0-9._-]+$`
- Prevents query injection into Datadog metric queries
- All JSON request bodies validated and decoded safely

---

## Secrets Management

### What's in the K8s Secret

The Helm-managed Secret contains:
- `admin.password` — bcrypt hash of admin password
- `admin.initialPassword` — plaintext initial password (removed after first change)
- `GITHUB_TOKEN` — GitHub PAT for Git provider access
- `ARGOCD_TOKEN` — ArgoCD API bearer token
- `AI_API_KEY` — API key for the configured AI provider (Gemini/Claude/OpenAI)
- `DATADOG_API_KEY` / `DATADOG_APP_KEY` — Datadog metrics access

### Secrets in Transit

- **ArgoCD API calls**: Bearer token in `Authorization` header (never in URL)
- **Gemini API calls**: API key in `x-goog-api-key` header (never in URL query string)
- **Datadog API calls**: API key in `DD-API-KEY` header (never in URL query string)
- **GitHub API calls**: Token via OAuth2 transport (standard `Authorization` header)

### Secrets in API Responses

- All tokens are masked in API responses (e.g., `ghp_****...ABCD`)
- K8s Secret resources are explicitly blocked from the AI agent's resource tools
- The `GetManagedResource` ArgoCD client method refuses to return `Secret` kind resources

### Secrets in AI Context

When using cloud AI providers (Gemini, Claude, OpenAI), the following data is sent to the provider's API:
- Cluster names and addon configurations
- Health status and version information
- Tool call results (addon values, resource lists, events)
- **No secrets, tokens, or passwords are sent**

When using Ollama (local), all data stays within the cluster.

---

## Container Security

### Docker Image

- Multi-stage build: source code and build tools not in final image
- Base image: `alpine:3.21` (minimal attack surface)
- Runs as non-root user (UID 1001)
- Binary is statically compiled (`CGO_ENABLED=0`)
- Image size: ~20MB

### Pod Security

Helm chart enforces:
```yaml
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1001
  runAsGroup: 1001
  fsGroup: 1001

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  capabilities:
    drop: [ALL]
```

### RBAC (Least Privilege)

The ServiceAccount has:
- **ClusterRole**: Read-only access to ArgoCD applications and app projects
- **Role (argocd namespace)**: Read-only access to secrets (for cluster connection info)
- **Role (own namespace)**: Read/update own Secret, read own ConfigMap
- **No write access** to any ArgoCD resources

---

## Network Security

### Ingress

- ALB ingress with HTTPS (TLS terminated at ALB)
- ACM wildcard certificate for `*.mahi-techlabs.com`
- HTTP-to-HTTPS redirect via `alb.ingress.kubernetes.io/ssl-redirect: "443"`

### In-Cluster Communication

- ArgoCD: HTTP within cluster network (port 80, TLS terminated at ArgoCD's ingress)
- `InsecureSkipVerify: true` for ArgoCD TLS — acceptable for in-cluster traffic but noted as a known consideration

---

## AI Agent Security

### Tool Access

The AI agent has read-only access to:
- Git repository contents (configuration files)
- ArgoCD applications, clusters, resource trees, events, pod logs
- Helm chart versions and release notes
- Web search (DuckDuckGo)

The agent **cannot**:
- Modify any resources (read-only platform)
- Access K8s Secrets (explicitly blocked)
- Execute arbitrary commands
- Access resources outside ArgoCD's scope

### Agent Sessions

- Sessions expire after 1 hour
- Maximum 100 concurrent sessions (oldest evicted when exceeded)
- Pruning runs on each chat request
- Each session is bound to an authenticated user session

### Agent Memory

- Persistent memory file at `/tmp/aap-agent-memory.json` (permissions: 0600)
- Stores learned observations across conversations (max 100 entries)
- No secrets or tokens stored in memory
- Memory is pod-local (not shared across replicas)

---

## Known Considerations

| Item | Severity | Description | Mitigation |
|------|----------|-------------|------------|
| In-cluster TLS | MEDIUM | `InsecureSkipVerify: true` for ArgoCD | In-cluster traffic only; ArgoCD uses internal K8s CA |
| Token in sessionStorage | LOW | XSS could exfiltrate auth token | No known XSS vectors; YAML viewer properly escapes all HTML entities |
| Single-user model | LOW | Currently only `admin` account | ConfigMap supports multiple users; multi-user with session binding is a future enhancement |
| Agent memory on /tmp | LOW | Lost on pod restart | Non-critical data (learned observations); persistent volume could be added |

---

## Reporting Security Issues

If you discover a security vulnerability, please report it responsibly by contacting the maintainers directly rather than opening a public issue.
