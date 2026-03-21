package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moran/argocd-addons-platform/internal/argocd"
	"github.com/moran/argocd-addons-platform/internal/config"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
	"github.com/moran/argocd-addons-platform/internal/models"
)

// ConnectionService manages connections and provides active provider instances.
type ConnectionService struct {
	store config.Store
}

// NewConnectionService creates a new ConnectionService.
func NewConnectionService(store config.Store) *ConnectionService {
	return &ConnectionService{store: store}
}

// List returns all connections with masked tokens.
func (s *ConnectionService) List() (*models.ConnectionsListResponse, error) {
	connections, err := s.store.ListConnections()
	if err != nil {
		return nil, err
	}

	activeName, err := s.store.GetActiveConnection()
	if err != nil {
		return nil, err
	}

	responses := make([]models.ConnectionResponse, 0, len(connections))
	for _, c := range connections {
		responses = append(responses, c.ToResponse(c.Name == activeName))
	}

	return &models.ConnectionsListResponse{
		Connections:      responses,
		ActiveConnection: activeName,
	}, nil
}

// Create adds a new connection.
func (s *ConnectionService) Create(req models.CreateConnectionRequest) error {
	// Parse repo URL into provider/owner/repo if provided
	if err := req.Git.ParseRepoURL(); err != nil {
		return fmt.Errorf("invalid git URL: %w", err)
	}
	conn := models.Connection{
		Name:        req.Name,
		Description: req.Description,
		Git:         req.Git,
		Argocd:      req.Argocd,
		IsDefault:   req.SetAsDefault,
	}
	// Auto-derive connection name from git repo if not provided
	if conn.Name == "" || conn.Name == "default" {
		conn.Name = deriveConnectionName(conn.Git)
	}
	return s.store.SaveConnection(conn)
}

// deriveConnectionName builds a connection name from the git config.
func deriveConnectionName(git models.GitRepoConfig) string {
	switch git.Provider {
	case models.GitProviderGitHub:
		if git.Owner != "" && git.Repo != "" {
			return git.Owner + "/" + git.Repo
		}
	case models.GitProviderAzureDevOps:
		if git.Organization != "" && git.Project != "" && git.Repository != "" {
			return git.Organization + "/" + git.Project + "/" + git.Repository
		}
	}
	return "default"
}

// Delete removes a connection.
func (s *ConnectionService) Delete(name string) error {
	return s.store.DeleteConnection(name)
}

// SetActive sets the active connection.
func (s *ConnectionService) SetActive(name string) error {
	slog.Info("active connection changed", "connection", name)
	return s.store.SetActiveConnection(name)
}

// GetActiveGitProvider returns a GitProvider for the currently active connection.
func (s *ConnectionService) GetActiveGitProvider() (gitprovider.GitProvider, error) {
	conn, err := s.getActiveConn()
	if err != nil {
		return nil, err
	}
	return s.buildGitProvider(conn)
}

// GetActiveArgocdClient returns an ArgoCD client for the currently active connection.
// If server_url is empty, it uses in-cluster mode with the pod's ServiceAccount token.
func (s *ConnectionService) GetActiveArgocdClient() (*argocd.Client, error) {
	conn, err := s.getActiveConn()
	if err != nil {
		return nil, err
	}
	return s.buildArgocdClient(conn)
}

func (s *ConnectionService) buildArgocdClient(conn *models.Connection) (*argocd.Client, error) {
	// If token is empty and we're running in K8s, use ServiceAccount token
	if conn.Argocd.Token == "" {
		slog.Info("argocd: no token configured, attempting in-cluster ServiceAccount auth")
		return argocd.NewInClusterClient(conn.Argocd.ServerURL, conn.Argocd.Namespace)
	}
	return argocd.NewClient(conn.Argocd.ServerURL, conn.Argocd.Token, conn.Argocd.Insecure), nil
}

// GetGitProviderForConnection returns a GitProvider for a specific named connection.
func (s *ConnectionService) GetGitProviderForConnection(name string) (gitprovider.GitProvider, error) {
	conn, err := s.store.GetConnection(name)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("connection %q not found", name)
	}
	return s.buildGitProvider(conn)
}

// TestConnection tests both Git and ArgoCD connectivity for the active connection.
func (s *ConnectionService) TestConnection(ctx context.Context) (gitErr, argocdErr error) {
	conn, err := s.getActiveConn()
	if err != nil {
		return err, err
	}

	gp, err := s.buildGitProvider(conn)
	if err != nil {
		gitErr = err
	} else {
		gitErr = gp.TestConnection(ctx)
	}

	ac, err := s.buildArgocdClient(conn)
	if err != nil {
		argocdErr = err
	} else {
		argocdErr = ac.TestConnection(ctx)
	}

	return gitErr, argocdErr
}

// TestCredentials tests Git and ArgoCD connectivity for unsaved credentials.
func (s *ConnectionService) TestCredentials(ctx context.Context, conn *models.Connection) (gitErr, argocdErr error) {
	// Parse repo URL if provided
	if err := conn.Git.ParseRepoURL(); err != nil {
		return err, nil
	}
	gp, err := s.buildGitProvider(conn)
	if err != nil {
		gitErr = err
	} else {
		gitErr = gp.TestConnection(ctx)
	}

	ac, err := s.buildArgocdClient(conn)
	if err != nil {
		argocdErr = err
	} else {
		argocdErr = ac.TestConnection(ctx)
	}

	return gitErr, argocdErr
}

func (s *ConnectionService) getActiveConn() (*models.Connection, error) {
	activeName, err := s.store.GetActiveConnection()
	if err != nil {
		return nil, err
	}
	if activeName == "" {
		return nil, fmt.Errorf("no active connection configured")
	}

	conn, err := s.store.GetConnection(activeName)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("active connection %q not found", activeName)
	}

	return conn, nil
}

func (s *ConnectionService) buildGitProvider(conn *models.Connection) (gitprovider.GitProvider, error) {
	switch conn.Git.Provider {
	case models.GitProviderGitHub:
		return gitprovider.NewGitHubProvider(conn.Git.Owner, conn.Git.Repo, conn.Git.Token), nil
	case models.GitProviderAzureDevOps:
		return gitprovider.NewAzureDevOpsProvider(conn.Git.Organization, conn.Git.Project, conn.Git.Repository, conn.Git.PAT), nil
	default:
		return nil, fmt.Errorf("unsupported git provider: %s", conn.Git.Provider)
	}
}
