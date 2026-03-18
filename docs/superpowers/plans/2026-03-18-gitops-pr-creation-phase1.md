# GitOps PR Creation — Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable the AI agent to create GitHub branches, modify files, open PRs, and trigger ArgoCD sync/refresh — enough for the migration wizard POC.

**Architecture:** Extend GitProvider interface with write methods, extend ArgoCD client with sync/refresh, add custom OpenAI-compatible provider support, add write tools to AI agent. All write operations go through Git PRs, never direct deployment.

**Tech Stack:** Go 1.25, go-github/v68, ArgoCD REST API, OpenAI-compatible chat/completions API

**Spec:** `docs/superpowers/specs/2026-03-18-gitops-pr-creation-design.md`

---

## File Structure

### New Files
| File | Responsibility |
|------|---------------|
| `internal/gitprovider/github_write.go` | GitHub write methods (CreateBranch, CreateOrUpdateFile, DeleteFile, CreatePullRequest) |
| `internal/gitprovider/github_write_test.go` | Tests for GitHub write methods using httptest mock server |
| `internal/argocd/client_write.go` | ArgoCD write methods (SyncApplication, RefreshApplication, doPost) |
| `internal/argocd/client_write_test.go` | Tests for ArgoCD write methods |
| `internal/ai/tools_write.go` | Write tool definitions and executors (enable_addon, disable_addon, etc.) |
| `internal/ai/tools_write_test.go` | Tests for write tools |
| `internal/gitops/yaml_mutator.go` | YAML-preserving mutation for cluster-addons.yaml and addons-catalog.yaml |
| `internal/gitops/yaml_mutator_test.go` | Tests for YAML mutation |

### Modified Files
| File | Change |
|------|--------|
| `internal/gitprovider/provider.go` | Add 4 write methods to GitProvider interface |
| `internal/gitprovider/azuredevops.go` | Add stub implementations for write methods |
| `internal/ai/client.go` | Add `ProviderCustomOpenAI`, `BaseURL`, `AuthHeader`, `MaxIterations`, TLS fields to Config |
| `internal/ai/agent.go` | Add `callCustomOpenAIChat`, `ProviderCustomOpenAI` dispatch, configurable loop limit |
| `internal/ai/tools.go` | Register write tools in `GetToolDefinitions()` (when enabled) |

---

## Task 1: Extend GitProvider Interface with Write Methods

**Files:**
- Modify: `internal/gitprovider/provider.go`
- Modify: `internal/gitprovider/azuredevops.go`

- [ ] **Step 1: Add write methods to GitProvider interface**

In `internal/gitprovider/provider.go`, add after the existing 4 methods:

```go
// Write operations — create branches, modify files, open PRs.
CreateBranch(ctx context.Context, branchName, fromRef string) error
CreateOrUpdateFile(ctx context.Context, path string, content []byte, branch, commitMessage string) error
DeleteFile(ctx context.Context, path, branch, commitMessage string) error
CreatePullRequest(ctx context.Context, title, body, head, base string) (*PullRequest, error)
```

- [ ] **Step 2: Add stub implementations in Azure DevOps provider**

In `internal/gitprovider/azuredevops.go`, add stub methods that return `fmt.Errorf("azure devops: write operations not implemented")`.

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./...`
Expected: Compilation fails because `GitHubProvider` doesn't implement the new methods yet. That's correct — we'll fix it in Task 2.

- [ ] **Step 4: Commit**

```bash
git add internal/gitprovider/provider.go internal/gitprovider/azuredevops.go
git commit -m "feat: add write methods to GitProvider interface"
```

---

## Task 2: Implement GitHub Write Methods

**Files:**
- Create: `internal/gitprovider/github_write.go`
- Create: `internal/gitprovider/github_write_test.go`

- [ ] **Step 1: Write tests for CreateBranch**

Create `internal/gitprovider/github_write_test.go`. Use `net/http/httptest` to mock the GitHub API. Test:
- CreateBranch success: GET ref returns SHA, POST ref creates branch
- CreateBranch failure: ref not found returns error

```go
package gitprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v68/github"
)

func newTestGitHubProvider(handler http.Handler) *GitHubProvider {
	server := httptest.NewServer(handler)
	client := github.NewClient(nil)
	url := server.URL + "/"
	client, _ = client.WithEnterpriseURLs(url, url)
	return &GitHubProvider{
		client: client,
		owner:  "test-owner",
		repo:   "test-repo",
	}
}

func TestCreateBranch(t *testing.T) {
	mux := http.NewServeMux()
	// GET /repos/test-owner/test-repo/git/ref/heads/main
	mux.HandleFunc("/repos/test-owner/test-repo/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ref": "refs/heads/main",
			"object": map[string]string{
				"sha": "abc123",
			},
		})
	})
	// POST /repos/test-owner/test-repo/git/refs
	mux.HandleFunc("/repos/test-owner/test-repo/git/refs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["ref"] != "refs/heads/aap/test-branch" {
			t.Errorf("expected refs/heads/aap/test-branch, got %s", body["ref"])
		}
		if body["sha"] != "abc123" {
			t.Errorf("expected sha abc123, got %s", body["sha"])
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ref": "refs/heads/aap/test-branch",
			"object": map[string]string{"sha": "abc123"},
		})
	})

	provider := newTestGitHubProvider(mux)
	err := provider.CreateBranch(context.Background(), "aap/test-branch", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/gitprovider/ -run TestCreateBranch -v`
Expected: FAIL — `CreateBranch` method doesn't exist yet.

- [ ] **Step 3: Implement CreateBranch**

Create `internal/gitprovider/github_write.go`:

```go
package gitprovider

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v68/github"
)

// CreateBranch creates a new branch from the given ref (e.g., "main").
func (g *GitHubProvider) CreateBranch(ctx context.Context, branchName, fromRef string) error {
	ref, _, err := g.client.Git.GetRef(ctx, g.owner, g.repo, "refs/heads/"+fromRef)
	if err != nil {
		return fmt.Errorf("get ref %q: %w", fromRef, err)
	}

	newRef := &github.Reference{
		Ref:    github.Ptr("refs/heads/" + branchName),
		Object: &github.GitObject{SHA: ref.Object.SHA},
	}
	_, _, err = g.client.Git.CreateRef(ctx, g.owner, g.repo, newRef)
	if err != nil {
		return fmt.Errorf("create branch %q: %w", branchName, err)
	}

	slog.Info("github branch created", "branch", branchName, "from", fromRef)
	return nil
}

// getContentsRaw fetches a file and returns the full RepositoryContent (including SHA).
func (g *GitHubProvider) getContentsRaw(ctx context.Context, path, ref string) (*github.RepositoryContent, error) {
	opts := &github.RepositoryContentGetOptions{Ref: ref}
	fileContent, _, _, err := g.client.Repositories.GetContents(ctx, g.owner, g.repo, path, opts)
	if err != nil {
		return nil, err
	}
	if fileContent == nil {
		return nil, fmt.Errorf("path %q is not a file", path)
	}
	return fileContent, nil
}

// CreateOrUpdateFile creates or updates a file on the given branch.
// If the file exists, it fetches the current SHA and updates. On SHA mismatch (422), retries once.
func (g *GitHubProvider) CreateOrUpdateFile(ctx context.Context, path string, content []byte, branch, commitMessage string) error {
	opts := &github.RepositoryContentFileOptions{
		Message: github.Ptr(commitMessage),
		Content: content,
		Branch:  github.Ptr(branch),
		Author: &github.CommitAuthor{
			Name:  github.Ptr("AAP Bot"),
			Email: github.Ptr("aap-bot@users.noreply.github.com"),
		},
	}

	// Try to get existing file SHA
	existing, err := g.getContentsRaw(ctx, path, branch)
	if err == nil && existing != nil {
		opts.SHA = existing.SHA
	}

	var resp *github.Response
	if opts.SHA != nil {
		_, resp, err = g.client.Repositories.UpdateFile(ctx, g.owner, g.repo, path, opts)
	} else {
		_, resp, err = g.client.Repositories.CreateFile(ctx, g.owner, g.repo, path, opts)
	}

	// Retry once on 422 SHA mismatch
	if err != nil && resp != nil && resp.StatusCode == 422 {
		slog.Warn("github SHA mismatch, retrying", "path", path, "branch", branch)
		existing, retryErr := g.getContentsRaw(ctx, path, branch)
		if retryErr != nil {
			return fmt.Errorf("retry get SHA for %q: %w", path, retryErr)
		}
		opts.SHA = existing.SHA
		_, _, err = g.client.Repositories.UpdateFile(ctx, g.owner, g.repo, path, opts)
	}

	if err != nil {
		return fmt.Errorf("create/update file %q: %w", path, err)
	}

	slog.Info("github file updated", "path", path, "branch", branch)
	return nil
}

// DeleteFile removes a file on the given branch.
func (g *GitHubProvider) DeleteFile(ctx context.Context, path, branch, commitMessage string) error {
	existing, err := g.getContentsRaw(ctx, path, branch)
	if err != nil {
		return fmt.Errorf("get file for delete %q: %w", path, err)
	}

	opts := &github.RepositoryContentFileOptions{
		Message: github.Ptr(commitMessage),
		Branch:  github.Ptr(branch),
		SHA:     existing.SHA,
		Author: &github.CommitAuthor{
			Name:  github.Ptr("AAP Bot"),
			Email: github.Ptr("aap-bot@users.noreply.github.com"),
		},
	}

	_, _, err = g.client.Repositories.DeleteFile(ctx, g.owner, g.repo, path, opts)
	if err != nil {
		return fmt.Errorf("delete file %q: %w", path, err)
	}

	slog.Info("github file deleted", "path", path, "branch", branch)
	return nil
}

// CreatePullRequest opens a new pull request.
func (g *GitHubProvider) CreatePullRequest(ctx context.Context, title, body, head, base string) (*PullRequest, error) {
	newPR := &github.NewPullRequest{
		Title:               github.Ptr(title),
		Head:                github.Ptr(head),
		Base:                github.Ptr(base),
		Body:                github.Ptr(body),
		MaintainerCanModify: github.Ptr(true),
	}

	pr, _, err := g.client.PullRequests.Create(ctx, g.owner, g.repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("create pull request: %w", err)
	}

	result := &PullRequest{
		ID:           pr.GetNumber(),
		Title:        pr.GetTitle(),
		Description:  pr.GetBody(),
		Author:       pr.GetUser().GetLogin(),
		Status:       "open",
		SourceBranch: pr.GetHead().GetRef(),
		TargetBranch: pr.GetBase().GetRef(),
		URL:          pr.GetHTMLURL(),
	}
	if t := pr.GetCreatedAt(); !t.IsZero() {
		result.CreatedAt = t.Format("2006-01-02T15:04:05Z")
	}

	slog.Info("github pull request created", "number", result.ID, "url", result.URL)
	return result, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/gitprovider/ -run TestCreateBranch -v`
Expected: PASS

- [ ] **Step 5: Add tests for CreateOrUpdateFile and CreatePullRequest**

Add to `github_write_test.go`:

```go
func TestCreateOrUpdateFile_NewFile(t *testing.T) {
	mux := http.NewServeMux()
	// GET contents returns 404 (file doesn't exist)
	mux.HandleFunc("/repos/test-owner/test-repo/contents/configuration/new-file.yaml", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(404)
			return
		}
		// PUT — create file
		if r.Method == "PUT" {
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"content": map[string]string{"sha": "newsha"},
				"commit":  map[string]string{"sha": "commitsha"},
			})
			return
		}
	})

	provider := newTestGitHubProvider(mux)
	err := provider.CreateOrUpdateFile(context.Background(), "configuration/new-file.yaml", []byte("test: true"), "aap/test", "add file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreatePullRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test-owner/test-repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"number":   42,
			"title":    "Enable keda on feedlot-dev",
			"html_url": "https://github.com/test-owner/test-repo/pull/42",
			"head":     map[string]string{"ref": "aap/enable-addon/keda/feedlot-dev/123"},
			"base":     map[string]string{"ref": "main"},
			"user":     map[string]string{"login": "aap-bot"},
		})
	})

	provider := newTestGitHubProvider(mux)
	pr, err := provider.CreatePullRequest(context.Background(), "Enable keda on feedlot-dev", "Automated by AAP", "aap/enable-addon/keda/feedlot-dev/123", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.ID != 42 {
		t.Errorf("expected PR #42, got #%d", pr.ID)
	}
	if pr.URL != "https://github.com/test-owner/test-repo/pull/42" {
		t.Errorf("expected PR URL, got %s", pr.URL)
	}
}
```

- [ ] **Step 6: Run all gitprovider tests**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/gitprovider/ -v`
Expected: All PASS

- [ ] **Step 7: Verify full build compiles**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./...`
Expected: Success (GitHubProvider now implements the full interface)

- [ ] **Step 8: Commit**

```bash
git add internal/gitprovider/github_write.go internal/gitprovider/github_write_test.go
git commit -m "feat: implement GitHub write methods (branch, file, PR creation)"
```

---

## Task 3: YAML Mutator for cluster-addons.yaml

**Files:**
- Create: `internal/gitops/yaml_mutator.go`
- Create: `internal/gitops/yaml_mutator_test.go`

- [ ] **Step 1: Write tests for EnableAddonLabel and DisableAddonLabel**

Create `internal/gitops/yaml_mutator_test.go`:

```go
package gitops

import "testing"

func TestEnableAddonLabel(t *testing.T) {
	input := `clusters:
  - name: feedlot-dev
    labels:
      datadog: enabled
      keda: disabled
  - name: ark-dev-eks
    labels:
      datadog: enabled
`
	result, err := EnableAddonLabel([]byte(input), "feedlot-dev", "keda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `clusters:
  - name: feedlot-dev
    labels:
      datadog: enabled
      keda: enabled
  - name: ark-dev-eks
    labels:
      datadog: enabled
`
	if string(result) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(result))
	}
}

func TestEnableAddonLabel_AddNew(t *testing.T) {
	input := `clusters:
  - name: feedlot-dev
    labels:
      datadog: enabled
`
	result, err := EnableAddonLabel([]byte(input), "feedlot-dev", "keda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should add the new label under the existing labels
	if !containsLine(string(result), "      keda: enabled") {
		t.Errorf("expected keda: enabled to be added, got:\n%s", string(result))
	}
}

func TestDisableAddonLabel(t *testing.T) {
	input := `clusters:
  - name: feedlot-dev
    labels:
      datadog: enabled
      keda: enabled
`
	result, err := DisableAddonLabel([]byte(input), "feedlot-dev", "keda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsLine(string(result), "      keda: disabled") {
		t.Errorf("expected keda: disabled, got:\n%s", string(result))
	}
}

func TestClusterNotFound(t *testing.T) {
	input := `clusters:
  - name: feedlot-dev
    labels:
      datadog: enabled
`
	_, err := EnableAddonLabel([]byte(input), "nonexistent-cluster", "keda")
	if err == nil {
		t.Fatal("expected error for nonexistent cluster")
	}
}

func containsLine(s, line string) bool {
	for _, l := range splitLines(s) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/gitops/ -v`
Expected: FAIL — package doesn't exist yet

- [ ] **Step 3: Implement YAML mutator**

Create `internal/gitops/yaml_mutator.go`:

```go
package gitops

import (
	"fmt"
	"regexp"
	"strings"
)

// EnableAddonLabel sets addon: enabled in cluster-addons.yaml for the given cluster.
// Uses line-level manipulation to preserve comments, anchors, and formatting.
func EnableAddonLabel(data []byte, clusterName, addonName string) ([]byte, error) {
	return setAddonLabel(data, clusterName, addonName, "enabled")
}

// DisableAddonLabel sets addon: disabled in cluster-addons.yaml for the given cluster.
func DisableAddonLabel(data []byte, clusterName, addonName string) ([]byte, error) {
	return setAddonLabel(data, clusterName, addonName, "disabled")
}

func setAddonLabel(data []byte, clusterName, addonName, value string) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	clusterIdx := findClusterBlock(lines, clusterName)
	if clusterIdx < 0 {
		return nil, fmt.Errorf("cluster %q not found", clusterName)
	}

	labelsIdx := findLabelsLine(lines, clusterIdx)
	if labelsIdx < 0 {
		return nil, fmt.Errorf("no labels section found for cluster %q", clusterName)
	}

	// Find the addon label line within this cluster's labels block
	labelEnd := findLabelBlockEnd(lines, labelsIdx)
	addonLineIdx := -1
	addonPattern := regexp.MustCompile(`^\s+` + regexp.QuoteMeta(addonName) + `:\s+\S+`)
	for i := labelsIdx + 1; i < labelEnd; i++ {
		if addonPattern.MatchString(lines[i]) {
			addonLineIdx = i
			break
		}
	}

	if addonLineIdx >= 0 {
		// Replace existing label value
		re := regexp.MustCompile(`(^\s+` + regexp.QuoteMeta(addonName) + `:\s+)\S+`)
		lines[addonLineIdx] = re.ReplaceAllString(lines[addonLineIdx], "${1}"+value)
	} else {
		// Add new label — detect indentation from existing labels
		indent := "      " // default 6 spaces
		if labelsIdx+1 < labelEnd {
			match := regexp.MustCompile(`^(\s+)`).FindString(lines[labelsIdx+1])
			if match != "" {
				indent = match
			}
		}
		newLine := indent + addonName + ": " + value
		// Insert after the last label line
		insertAt := labelEnd
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:insertAt]...)
		newLines = append(newLines, newLine)
		newLines = append(newLines, lines[insertAt:]...)
		lines = newLines
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// findClusterBlock returns the line index of `- name: <clusterName>`.
func findClusterBlock(lines []string, clusterName string) int {
	pattern := regexp.MustCompile(`^\s+-\s+name:\s+` + regexp.QuoteMeta(clusterName) + `\s*$`)
	for i, line := range lines {
		if pattern.MatchString(line) {
			return i
		}
	}
	return -1
}

// findLabelsLine returns the line index of `labels:` within a cluster block.
func findLabelsLine(lines []string, clusterIdx int) int {
	for i := clusterIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		// If we hit another cluster entry, stop
		if strings.HasPrefix(trimmed, "- name:") {
			break
		}
		if strings.TrimSpace(lines[i]) == "labels:" || strings.HasSuffix(strings.TrimSpace(lines[i]), "labels:") {
			return i
		}
	}
	return -1
}

// findLabelBlockEnd returns the line index where the labels block ends.
func findLabelBlockEnd(lines []string, labelsIdx int) int {
	if labelsIdx+1 >= len(lines) {
		return labelsIdx + 1
	}
	// Get indentation of labels line
	labelsIndent := countLeadingSpaces(lines[labelsIdx])
	for i := labelsIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		lineIndent := countLeadingSpaces(lines[i])
		// If indentation is <= labels line, we've left the labels block
		if lineIndent <= labelsIndent {
			return i
		}
	}
	return len(lines)
}

func countLeadingSpaces(s string) int {
	count := 0
	for _, c := range s {
		if c == ' ' {
			count++
		} else {
			break
		}
	}
	return count
}

// UpdateCatalogVersion updates the version field for an addon in addons-catalog.yaml.
func UpdateCatalogVersion(data []byte, addonName, newVersion string) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	appNamePattern := regexp.MustCompile(`^\s+-?\s*appName:\s+` + regexp.QuoteMeta(addonName) + `\s*$`)
	versionPattern := regexp.MustCompile(`^(\s+version:\s+)(.+)$`)

	foundAddon := false
	for i, line := range lines {
		if appNamePattern.MatchString(line) {
			foundAddon = true
			continue
		}
		if foundAddon && versionPattern.MatchString(line) {
			lines[i] = versionPattern.ReplaceAllString(line, "${1}"+newVersion)
			return []byte(strings.Join(lines, "\n")), nil
		}
		// If we hit another appName entry, the addon had no version field
		if foundAddon && strings.Contains(line, "appName:") {
			break
		}
	}

	if !foundAddon {
		return nil, fmt.Errorf("addon %q not found in catalog", addonName)
	}
	return nil, fmt.Errorf("version field not found for addon %q", addonName)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/gitops/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gitops/yaml_mutator.go internal/gitops/yaml_mutator_test.go
git commit -m "feat: add YAML mutator for cluster-addons and addons-catalog"
```

---

## Task 4: ArgoCD Write Methods (Sync + Refresh)

**Files:**
- Create: `internal/argocd/client_write.go`
- Create: `internal/argocd/client_write_test.go`

- [ ] **Step 1: Write tests**

Create `internal/argocd/client_write_test.go`:

```go
package argocd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncApplication(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/applications/datadog-feedlot-dev/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL, "test-token", true)
	err := client.SyncApplication(context.Background(), "datadog-feedlot-dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRefreshApplication(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/applications/datadog-feedlot-dev", func(w http.ResponseWriter, r *http.Request) {
		refresh := r.URL.Query().Get("refresh")
		if refresh != "hard" {
			t.Errorf("expected refresh=hard, got %s", refresh)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"metadata": map[string]string{"name": "datadog-feedlot-dev", "namespace": "argocd"},
			"status": map[string]interface{}{
				"sync":   map[string]string{"status": "Synced"},
				"health": map[string]string{"status": "Healthy"},
			},
			"spec": map[string]interface{}{
				"source":      map[string]string{},
				"destination": map[string]string{},
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL, "test-token", true)
	app, err := client.RefreshApplication(context.Background(), "datadog-feedlot-dev", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.HealthStatus != "Healthy" {
		t.Errorf("expected Healthy, got %s", app.HealthStatus)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/argocd/ -run "TestSync|TestRefresh" -v`
Expected: FAIL

- [ ] **Step 3: Implement SyncApplication, RefreshApplication, and doPost**

Create `internal/argocd/client_write.go`:

```go
package argocd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/moran/argocd-addons-platform/internal/models"
)

// SyncApplication triggers a sync for the named application.
func (c *Client) SyncApplication(ctx context.Context, appName string) error {
	_, err := c.doPost(ctx, "/api/v1/applications/"+appName+"/sync", []byte("{}"))
	if err != nil {
		return fmt.Errorf("sync application %q: %w", appName, err)
	}
	slog.Info("argocd application synced", "app", appName)
	return nil
}

// RefreshApplication triggers a refresh (optionally hard) and returns the refreshed application.
func (c *Client) RefreshApplication(ctx context.Context, appName string, hard bool) (*models.ArgocdApplication, error) {
	refreshParam := "true"
	if hard {
		refreshParam = "hard"
	}
	path := fmt.Sprintf("/api/v1/applications/%s?refresh=%s", appName, refreshParam)

	body, err := c.doGet(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("refresh application %q: %w", appName, err)
	}

	var raw argocdApplicationItem
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding refreshed application: %w", err)
	}

	app := raw.toModel()
	slog.Info("argocd application refreshed", "app", appName, "hard", hard, "health", app.HealthStatus)
	return &app, nil
}

// doPost performs an authenticated POST request and returns the response body.
func (c *Client) doPost(ctx context.Context, path string, payload []byte) ([]byte, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("argocd POST failed", "error", err, "endpoint", path)
		return nil, fmt.Errorf("executing POST to %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("argocd POST failed", "endpoint", path, "status", resp.StatusCode)
		return nil, fmt.Errorf("unexpected status %d from POST %s: %s", resp.StatusCode, path, string(body))
	}

	return body, nil
}
```

Note: `client_write.go` needs `encoding/json` import — add it to the import block.

- [ ] **Step 4: Run tests**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go test ./internal/argocd/ -run "TestSync|TestRefresh" -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/argocd/client_write.go internal/argocd/client_write_test.go
git commit -m "feat: add ArgoCD sync and refresh application methods"
```

---

## Task 5: Custom OpenAI-Compatible Provider Support

**Files:**
- Modify: `internal/ai/client.go`
- Modify: `internal/ai/agent.go`

- [ ] **Step 1: Add ProviderCustomOpenAI and new config fields to client.go**

In `internal/ai/client.go`, add constant and fields:

```go
// After ProviderGemini:
ProviderCustomOpenAI Provider = "custom-openai"
```

Add fields to Config struct:

```go
type Config struct {
	Provider      Provider `yaml:"provider"`
	OllamaURL     string   `yaml:"ollama_url"`
	OllamaModel   string   `yaml:"ollama_model"`
	AgentModel    string   `yaml:"agent_model"`
	APIKey        string   `yaml:"api_key"`
	CloudModel    string   `yaml:"cloud_model"`
	BaseURL       string   `yaml:"base_url"`        // Custom endpoint base URL
	AuthHeader    string   `yaml:"auth_header"`      // Custom auth header name (default: "Authorization")
	AuthPrefix    string   `yaml:"auth_prefix"`      // Auth value prefix (default: "Bearer ")
	MaxIterations int      `yaml:"max_iterations"`   // Agent loop limit (default: 8)
}
```

Update `GetAgentModel` to include `ProviderCustomOpenAI`:

```go
func (c Config) GetAgentModel() string {
	if c.Provider == ProviderClaude || c.Provider == ProviderOpenAI || c.Provider == ProviderGemini || c.Provider == ProviderCustomOpenAI {
		if c.CloudModel != "" {
			return c.CloudModel
		}
		return "gemini-2.5-flash"
	}
	// ...existing
}
```

Add `ProviderCustomOpenAI` case to `Summarize`:

```go
case ProviderCustomOpenAI:
	return c.customOpenAISummarize(ctx, prompt)
```

Add `customOpenAISummarize` method — same as `openaiSummarize` but uses `BaseURL` and custom auth header:

```go
func (c *Client) customOpenAISummarize(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(map[string]interface{}{
		"model":      c.config.CloudModel,
		"max_tokens": 4096,
		"messages": []map[string]interface{}{
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	baseURL := strings.TrimRight(c.config.BaseURL, "/")
	url := baseURL + "/v2/" + c.config.CloudModel + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	authHeader := c.config.AuthHeader
	if authHeader == "" {
		authHeader = "Authorization"
	}
	authPrefix := c.config.AuthPrefix
	if authPrefix == "" && authHeader == "Authorization" {
		authPrefix = "Bearer "
	}
	req.Header.Set(authHeader, authPrefix+c.config.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("custom-openai request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("custom-openai returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("empty response")
}
```

- [ ] **Step 2: Add callCustomOpenAIChat to agent.go**

In `internal/ai/agent.go`, add dispatch case in `callLLM`:

```go
case ProviderCustomOpenAI:
	return a.callCustomOpenAIChat(ctx)
```

Add `callCustomOpenAIChat` method — same as `callOpenAIChat` but with configurable URL and auth:

```go
func (a *Agent) callCustomOpenAIChat(ctx context.Context) (*ChatResponse, error) {
	openaiTools := convertToolsToOpenAI(GetToolDefinitions())
	openaiMessages := convertMessagesToOpenAI(a.messages)

	reqBody := map[string]interface{}{
		"model":    a.client.config.GetAgentModel(),
		"messages": openaiMessages,
		"tools":    openaiTools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	baseURL := strings.TrimRight(a.client.config.BaseURL, "/")
	model := a.client.config.GetAgentModel()
	url := baseURL + "/v2/" + model + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	authHeader := a.client.config.AuthHeader
	if authHeader == "" {
		authHeader = "Authorization"
	}
	authPrefix := a.client.config.AuthPrefix
	if authPrefix == "" && authHeader == "Authorization" {
		authPrefix = "Bearer "
	}
	req.Header.Set(authHeader, authPrefix+a.client.config.APIKey)

	resp, err := a.client.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("custom-openai chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("custom-openai returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Same response parsing as OpenAI
	var result struct {
		Choices []struct {
			Message struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls,omitempty"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	choice := result.Choices[0]
	chatResp := &ChatResponse{Content: choice.Message.Content}
	for _, tc := range choice.Message.ToolCalls {
		chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: ToolCallFunc{
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			},
		})
	}

	return chatResp, nil
}
```

- [ ] **Step 3: Make agent loop limit configurable in agent.go**

Replace the hardcoded `for i := 0; i < 8; i++` in `Chat()` with:

```go
maxIter := a.client.config.MaxIterations
if maxIter <= 0 {
	maxIter = 8
}
for i := 0; i < maxIter; i++ {
```

- [ ] **Step 4: Verify build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./...`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add internal/ai/client.go internal/ai/agent.go
git commit -m "feat: add custom OpenAI-compatible provider and configurable agent loop limit"
```

---

## Task 6: AI Agent Write Tools

**Files:**
- Create: `internal/ai/tools_write.go`
- Modify: `internal/ai/tools.go` (register new tools)

- [ ] **Step 1: Create write tool definitions and executors**

Create `internal/ai/tools_write.go`:

```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/moran/argocd-addons-platform/internal/gitops"
)

// GetWriteToolDefinitions returns tool definitions for GitOps write operations.
func GetWriteToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "enable_addon",
				Description: "Enable an addon on a cluster by creating a Git PR that sets the addon label to 'enabled' in cluster-addons.yaml. Returns the PR URL.",
				Parameters: json.RawMessage(`{"type":"object","properties":{
					"cluster_name":{"type":"string","description":"Name of the cluster"},
					"addon_name":{"type":"string","description":"Name of the addon to enable"}
				},"required":["cluster_name","addon_name"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "disable_addon",
				Description: "Disable an addon on a cluster by creating a Git PR that sets the addon label to 'disabled' in cluster-addons.yaml. Returns the PR URL.",
				Parameters: json.RawMessage(`{"type":"object","properties":{
					"cluster_name":{"type":"string","description":"Name of the cluster"},
					"addon_name":{"type":"string","description":"Name of the addon to disable"}
				},"required":["cluster_name","addon_name"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "update_addon_version",
				Description: "Update the version of an addon in the addons-catalog.yaml by creating a Git PR. Returns the PR URL.",
				Parameters: json.RawMessage(`{"type":"object","properties":{
					"addon_name":{"type":"string","description":"Name of the addon"},
					"version":{"type":"string","description":"New version string"}
				},"required":["addon_name","version"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "sync_argocd_app",
				Description: "Trigger a sync for an ArgoCD application. The app name is usually {addon}-{cluster}.",
				Parameters: json.RawMessage(`{"type":"object","properties":{
					"app_name":{"type":"string","description":"ArgoCD application name"}
				},"required":["app_name"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "refresh_argocd_app",
				Description: "Trigger a refresh (optionally hard) for an ArgoCD application to re-read its source and reconcile.",
				Parameters: json.RawMessage(`{"type":"object","properties":{
					"app_name":{"type":"string","description":"ArgoCD application name"},
					"hard":{"type":"string","description":"Set to 'true' for hard refresh (default: false)"}
				},"required":["app_name"]}`),
			},
		},
	}
}

// sanitizeBranchName replaces non-alphanumeric/hyphen chars with hyphens.
func sanitizeBranchName(s string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	return re.ReplaceAllString(s, "-")
}

func (e *ToolExecutor) enableAddon(ctx context.Context, clusterName, addonName string) (string, error) {
	if clusterName == "" || addonName == "" {
		return "Please specify both cluster_name and addon_name.", nil
	}

	// Read current file
	data, err := e.gp.GetFileContent(ctx, "configuration/cluster-addons.yaml", "main")
	if err != nil {
		return "", fmt.Errorf("reading cluster-addons.yaml: %w", err)
	}

	// Mutate
	modified, err := gitops.EnableAddonLabel(data, clusterName, addonName)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}

	// Create branch, update file, open PR
	ts := time.Now().Unix()
	branch := fmt.Sprintf("aap/enable-addon/%s/%s/%d", sanitizeBranchName(addonName), sanitizeBranchName(clusterName), ts)

	if err := e.gp.CreateBranch(ctx, branch, "main"); err != nil {
		return "", fmt.Errorf("creating branch: %w", err)
	}

	commitMsg := fmt.Sprintf("Enable %s on %s", addonName, clusterName)
	if err := e.gp.CreateOrUpdateFile(ctx, "configuration/cluster-addons.yaml", modified, branch, commitMsg); err != nil {
		return "", fmt.Errorf("updating file: %w", err)
	}

	title := fmt.Sprintf("[AAP] Enable %s on %s", addonName, clusterName)
	body := fmt.Sprintf("Automated by ArgoCD Addons Platform.\n\n**Change:** Enable addon `%s` on cluster `%s`.", addonName, clusterName)
	pr, err := e.gp.CreatePullRequest(ctx, title, body, branch, "main")
	if err != nil {
		return "", fmt.Errorf("creating PR: %w", err)
	}

	return fmt.Sprintf("PR created: %s\nBranch: %s", pr.URL, branch), nil
}

func (e *ToolExecutor) disableAddon(ctx context.Context, clusterName, addonName string) (string, error) {
	if clusterName == "" || addonName == "" {
		return "Please specify both cluster_name and addon_name.", nil
	}

	data, err := e.gp.GetFileContent(ctx, "configuration/cluster-addons.yaml", "main")
	if err != nil {
		return "", fmt.Errorf("reading cluster-addons.yaml: %w", err)
	}

	modified, err := gitops.DisableAddonLabel(data, clusterName, addonName)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}

	ts := time.Now().Unix()
	branch := fmt.Sprintf("aap/disable-addon/%s/%s/%d", sanitizeBranchName(addonName), sanitizeBranchName(clusterName), ts)

	if err := e.gp.CreateBranch(ctx, branch, "main"); err != nil {
		return "", fmt.Errorf("creating branch: %w", err)
	}

	commitMsg := fmt.Sprintf("Disable %s on %s", addonName, clusterName)
	if err := e.gp.CreateOrUpdateFile(ctx, "configuration/cluster-addons.yaml", modified, branch, commitMsg); err != nil {
		return "", fmt.Errorf("updating file: %w", err)
	}

	title := fmt.Sprintf("[AAP] Disable %s on %s", addonName, clusterName)
	body := fmt.Sprintf("Automated by ArgoCD Addons Platform.\n\n**Change:** Disable addon `%s` on cluster `%s`.", addonName, clusterName)
	pr, err := e.gp.CreatePullRequest(ctx, title, body, branch, "main")
	if err != nil {
		return "", fmt.Errorf("creating PR: %w", err)
	}

	return fmt.Sprintf("PR created: %s\nBranch: %s", pr.URL, branch), nil
}

func (e *ToolExecutor) updateAddonVersion(ctx context.Context, addonName, version string) (string, error) {
	if addonName == "" || version == "" {
		return "Please specify both addon_name and version.", nil
	}

	data, err := e.gp.GetFileContent(ctx, "configuration/addons-catalog.yaml", "main")
	if err != nil {
		return "", fmt.Errorf("reading addons-catalog.yaml: %w", err)
	}

	modified, err := gitops.UpdateCatalogVersion(data, addonName, version)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}

	ts := time.Now().Unix()
	branch := fmt.Sprintf("aap/update-version/%s/%d", sanitizeBranchName(addonName), ts)

	if err := e.gp.CreateBranch(ctx, branch, "main"); err != nil {
		return "", fmt.Errorf("creating branch: %w", err)
	}

	commitMsg := fmt.Sprintf("Update %s to version %s", addonName, version)
	if err := e.gp.CreateOrUpdateFile(ctx, "configuration/addons-catalog.yaml", modified, branch, commitMsg); err != nil {
		return "", fmt.Errorf("updating file: %w", err)
	}

	title := fmt.Sprintf("[AAP] Update %s to %s", addonName, version)
	body := fmt.Sprintf("Automated by ArgoCD Addons Platform.\n\n**Change:** Update addon `%s` version to `%s`.", addonName, version)
	pr, err := e.gp.CreatePullRequest(ctx, title, body, branch, "main")
	if err != nil {
		return "", fmt.Errorf("creating PR: %w", err)
	}

	return fmt.Sprintf("PR created: %s\nBranch: %s", pr.URL, branch), nil
}

func (e *ToolExecutor) syncArgocdApp(ctx context.Context, appName string) (string, error) {
	if appName == "" {
		return "Please specify an app_name.", nil
	}
	if err := e.ac.SyncApplication(ctx, appName); err != nil {
		return fmt.Sprintf("Sync failed: %v", err), nil
	}
	return fmt.Sprintf("Sync triggered for application %s.", appName), nil
}

func (e *ToolExecutor) refreshArgocdApp(ctx context.Context, appName string, hard bool) (string, error) {
	if appName == "" {
		return "Please specify an app_name.", nil
	}
	app, err := e.ac.RefreshApplication(ctx, appName, hard)
	if err != nil {
		return fmt.Sprintf("Refresh failed: %v", err), nil
	}
	refreshType := "normal"
	if hard {
		refreshType = "hard"
	}
	return fmt.Sprintf("Application %s refreshed (%s). Health: %s, Sync: %s", appName, refreshType, app.HealthStatus, app.SyncStatus), nil
}
```

- [ ] **Step 2: Register write tools in tools.go ExecuteTool switch**

In `internal/ai/tools.go`, add cases to `ExecuteTool`:

```go
case "enable_addon":
	return e.enableAddon(ctx, params["cluster_name"], params["addon_name"])
case "disable_addon":
	return e.disableAddon(ctx, params["cluster_name"], params["addon_name"])
case "update_addon_version":
	return e.updateAddonVersion(ctx, params["addon_name"], params["version"])
case "sync_argocd_app":
	return e.syncArgocdApp(ctx, params["app_name"])
case "refresh_argocd_app":
	hard := strings.EqualFold(params["hard"], "true")
	an := params["app_name"]
	if an == "" { an = params["name"] }
	return e.refreshArgocdApp(ctx, an, hard)
```

Also append `GetWriteToolDefinitions()...` to `GetToolDefinitions()` return value:

```go
func GetToolDefinitions() []ToolDefinition {
	tools := []ToolDefinition{
		// ...existing tools
	}
	tools = append(tools, GetWriteToolDefinitions()...)
	return tools
}
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && go build ./...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add internal/ai/tools_write.go internal/ai/tools.go
git commit -m "feat: add write tools to AI agent (enable/disable addon, update version, sync/refresh)"
```

---

## Task 7: Helm Values + Config Updates

**Files:**
- Modify: `charts/argocd-addons-platform/values.yaml`

- [ ] **Step 1: Add gitops and AI config fields to values.yaml**

Add to `charts/argocd-addons-platform/values.yaml`:

```yaml
gitops:
  actions:
    enabled: false

# Under existing ai: section, add new fields:
ai:
  # ...existing fields
  baseURL: ""
  authHeader: ""
  maxIterations: 8
```

- [ ] **Step 2: Verify helm template renders**

Run: `cd /Users/weissmmo/projects/github-moran/argocd-addons-platform && helm template test charts/argocd-addons-platform/ > /dev/null`
Expected: Success (or check if helm is available)

- [ ] **Step 3: Commit**

```bash
git add charts/argocd-addons-platform/values.yaml
git commit -m "feat: add gitops actions feature flag and custom AI provider config to Helm values"
```

---

## Summary

After all 7 tasks, the codebase will have:
1. **GitProvider write interface** with GitHub implementation (branch, file, PR)
2. **YAML mutator** for cluster-addons.yaml and addons-catalog.yaml
3. **ArgoCD client** with sync and refresh methods
4. **Custom OpenAI provider** support with configurable URL/auth/mTLS
5. **AI agent write tools** for enable/disable addon, update version, sync/refresh
6. **Configurable agent loop limit** (default 8, configurable via config)
7. **Helm values** for gitops feature flag and custom provider config

This enables the POC: tell the AI agent "enable keda on feedlot-dev" and it creates a GitHub PR.
