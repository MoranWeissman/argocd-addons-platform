package argocd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/moran/argocd-addons-platform/internal/models"
)

// Client is a REST API client for ArgoCD.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// NewClient creates an ArgoCD client with bearer token authentication.
// Use this for local development or personal access token (PAT) mode.
// When insecure is true, TLS certificate verification is skipped.
func NewClient(serverURL, token string, insecure bool) *Client {
	transport := &http.Transport{}
	if insecure {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // intentional for self-hosted ArgoCD
		}
	}

	return &Client{
		baseURL:    strings.TrimRight(serverURL, "/"),
		httpClient: &http.Client{Transport: transport},
		token:      token,
	}
}

// NewInClusterClient creates an ArgoCD client for in-cluster use.
// It discovers the ArgoCD server via Kubernetes service DNS and reads the
// ServiceAccount token from the standard mount path.
func NewInClusterClient(namespace string) (*Client, error) {
	const saTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	tokenBytes, err := os.ReadFile(saTokenPath)
	if err != nil {
		return nil, fmt.Errorf("reading service account token: %w", err)
	}

	serverURL := fmt.Sprintf("https://argocd-server.%s.svc.cluster.local", namespace)

	// In-cluster communication typically uses cluster-internal CAs that the
	// default system pool may not trust, so we skip verification.
	return NewClient(serverURL, strings.TrimSpace(string(tokenBytes)), true), nil
}

// TestConnection verifies that the client can reach the ArgoCD server.
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.doGet(ctx, "/api/version")
	if err != nil {
		return fmt.Errorf("argocd connection test failed: %w", err)
	}
	return nil
}

// ListClusters returns all clusters registered in ArgoCD.
func (c *Client) ListClusters(ctx context.Context) ([]models.ArgocdCluster, error) {
	body, err := c.doGet(ctx, "/api/v1/clusters")
	if err != nil {
		return nil, fmt.Errorf("listing clusters: %w", err)
	}

	var raw struct {
		Items []argocdClusterItem `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding clusters response: %w", err)
	}

	clusters := make([]models.ArgocdCluster, 0, len(raw.Items))
	for _, item := range raw.Items {
		clusters = append(clusters, item.toModel())
	}
	return clusters, nil
}

// ListApplications returns all applications managed by ArgoCD.
func (c *Client) ListApplications(ctx context.Context) ([]models.ArgocdApplication, error) {
	body, err := c.doGet(ctx, "/api/v1/applications")
	if err != nil {
		return nil, fmt.Errorf("listing applications: %w", err)
	}

	var raw struct {
		Items []argocdApplicationItem `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding applications response: %w", err)
	}

	apps := make([]models.ArgocdApplication, 0, len(raw.Items))
	for _, item := range raw.Items {
		apps = append(apps, item.toModel())
	}
	return apps, nil
}

// GetApplication returns a single ArgoCD application by name.
func (c *Client) GetApplication(ctx context.Context, name string) (*models.ArgocdApplication, error) {
	body, err := c.doGet(ctx, "/api/v1/applications/"+name)
	if err != nil {
		return nil, fmt.Errorf("getting application %q: %w", name, err)
	}

	var raw argocdApplicationItem
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding application response: %w", err)
	}

	app := raw.toModel()
	return &app, nil
}

// GetVersion returns ArgoCD server version information.
func (c *Client) GetVersion(ctx context.Context) (map[string]string, error) {
	body, err := c.doGet(ctx, "/api/version")
	if err != nil {
		return nil, fmt.Errorf("getting version: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding version response: %w", err)
	}

	result := make(map[string]string)
	for k, v := range raw {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result, nil
}

// ListApplicationsSummary returns all applications with summary data (no history/resources).
// This is the same as ListApplications and is suitable for list views and health overviews.
func (c *Client) ListApplicationsSummary(ctx context.Context) ([]models.ArgocdApplication, error) {
	return c.ListApplications(ctx)
}

// doGet performs an authenticated GET request and returns the response body.
func (c *Client) doGet(ctx context.Context, path string) ([]byte, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request to %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, path, string(body))
	}

	return body, nil
}

// ---------- internal types for mapping ArgoCD API JSON ----------

// argocdClusterItem mirrors the nested JSON structure returned by the ArgoCD
// clusters API.
type argocdClusterItem struct {
	Name       string `json:"name"`
	Server     string `json:"server"`
	ServerVersion string `json:"serverVersion"`
	Namespaces []string `json:"namespaces"`
	Info       struct {
		ConnectionState struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"connectionState"`
		ServerVersion string `json:"serverVersion"`
	} `json:"info"`
}

func (c argocdClusterItem) toModel() models.ArgocdCluster {
	serverVersion := c.ServerVersion
	if serverVersion == "" {
		serverVersion = c.Info.ServerVersion
	}

	info := make(map[string]interface{})
	if c.Info.ConnectionState.Message != "" {
		info["connectionMessage"] = c.Info.ConnectionState.Message
	}

	return models.ArgocdCluster{
		Name:            c.Name,
		Server:          c.Server,
		ConnectionState: c.Info.ConnectionState.Status,
		ServerVersion:   serverVersion,
		Namespaces:      c.Namespaces,
		Info:            info,
	}
}

// argocdApplicationItem mirrors the nested JSON structure returned by the
// ArgoCD applications API.
type argocdApplicationItem struct {
	Metadata struct {
		Name              string `json:"name"`
		Namespace         string `json:"namespace"`
		CreationTimestamp  string `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec struct {
		Project string `json:"project"`
		Source  struct {
			RepoURL        string `json:"repoURL"`
			Path           string `json:"path"`
			TargetRevision string `json:"targetRevision"`
			Chart          string `json:"chart"`
			Helm           *struct {
				Parameters []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"parameters"`
			} `json:"helm"`
		} `json:"source"`
		Destination struct {
			Server    string `json:"server"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"destination"`
	} `json:"spec"`
	Status struct {
		Sync struct {
			Status string `json:"status"`
		} `json:"sync"`
		Health struct {
			Status             string `json:"status"`
			LastTransitionTime string `json:"lastTransitionTime"`
		} `json:"health"`
		ReconciledAt   string `json:"reconciledAt"`
		OperationState *struct {
			Phase      string `json:"phase"`
			StartedAt  string `json:"startedAt"`
			FinishedAt string `json:"finishedAt"`
			Message    string `json:"message"`
		} `json:"operationState"`
		History []struct {
			ID              int    `json:"id"`
			DeployedAt      string `json:"deployedAt"`
			DeployStartedAt string `json:"deployStartedAt"`
			Revision        string `json:"revision"`
			Revisions       []string `json:"revisions"`
			Source          *struct {
				RepoURL        string `json:"repoURL"`
				Path           string `json:"path"`
				TargetRevision string `json:"targetRevision"`
			} `json:"source"`
		} `json:"history"`
		Resources []struct {
			Group     string `json:"group"`
			Version   string `json:"version"`
			Kind      string `json:"kind"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			Health    *struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"health"`
			RequiresPruning bool `json:"requiresPruning"`
		} `json:"resources"`
	} `json:"status"`
}

func (a argocdApplicationItem) toModel() models.ArgocdApplication {
	app := models.ArgocdApplication{
		Name:                 a.Metadata.Name,
		Namespace:            a.Metadata.Namespace,
		Project:              a.Spec.Project,
		SourceRepoURL:        a.Spec.Source.RepoURL,
		SourcePath:           a.Spec.Source.Path,
		SourceTargetRevision: a.Spec.Source.TargetRevision,
		SourceChart:          a.Spec.Source.Chart,
		DestinationServer:    a.Spec.Destination.Server,
		DestinationName:      a.Spec.Destination.Name,
		DestinationNamespace: a.Spec.Destination.Namespace,
		SyncStatus:           a.Status.Sync.Status,
		HealthStatus:         a.Status.Health.Status,
		CreatedAt:            a.Metadata.CreationTimestamp,
		HealthLastTransition: a.Status.Health.LastTransitionTime,
		ReconciledAt:         a.Status.ReconciledAt,
	}

	if a.Status.OperationState != nil {
		app.OperationState = a.Status.OperationState.Phase
		app.OperationPhase = a.Status.OperationState.Phase
		app.OperationStartedAt = a.Status.OperationState.StartedAt
		app.OperationFinishedAt = a.Status.OperationState.FinishedAt
		app.OperationMessage = a.Status.OperationState.Message
	}

	if a.Spec.Source.Helm != nil {
		for _, p := range a.Spec.Source.Helm.Parameters {
			app.SourceHelmParameters = append(app.SourceHelmParameters, models.HelmParameter{
				Name:  p.Name,
				Value: p.Value,
			})
		}
	}

	for _, h := range a.Status.History {
		app.History = append(app.History, models.AppHistoryEntry{
			ID:              h.ID,
			DeployedAt:      h.DeployedAt,
			DeployStartedAt: h.DeployStartedAt,
			Revision:        h.Revision,
		})
	}

	for _, r := range a.Status.Resources {
		res := models.AppResource{
			Group:     r.Group,
			Kind:      r.Kind,
			Namespace: r.Namespace,
			Name:      r.Name,
			Status:    r.Status,
		}
		if r.Health != nil {
			res.Health = r.Health.Status
			res.Message = r.Health.Message
		}
		app.Resources = append(app.Resources, res)
	}

	return app
}
