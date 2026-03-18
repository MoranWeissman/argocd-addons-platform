package gitprovider

import (
	"context"
	"fmt"
	"net/http"
)

// AzureDevOpsProvider implements GitProvider for Azure DevOps repositories.
// This is a stub for future implementation.
type AzureDevOpsProvider struct {
	client       *http.Client
	organisation string
	project      string
	repository   string
	baseURL      string
}

// NewAzureDevOpsProvider creates a new Azure DevOps-backed GitProvider.
// The token is used as a personal access token for authentication.
func NewAzureDevOpsProvider(organisation, project, repository, token string) *AzureDevOpsProvider {
	return &AzureDevOpsProvider{
		client:       &http.Client{},
		organisation: organisation,
		project:      project,
		repository:   repository,
		baseURL:      fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/git/repositories/%s", organisation, project, repository),
	}
}

// GetFileContent is not yet implemented for Azure DevOps.
func (a *AzureDevOpsProvider) GetFileContent(_ context.Context, _, _ string) ([]byte, error) {
	return nil, fmt.Errorf("azure devops: GetFileContent not implemented")
}

// ListDirectory is not yet implemented for Azure DevOps.
func (a *AzureDevOpsProvider) ListDirectory(_ context.Context, _, _ string) ([]string, error) {
	return nil, fmt.Errorf("azure devops: ListDirectory not implemented")
}

// ListPullRequests is not yet implemented for Azure DevOps.
func (a *AzureDevOpsProvider) ListPullRequests(_ context.Context, _ string) ([]PullRequest, error) {
	return nil, fmt.Errorf("azure devops: ListPullRequests not implemented")
}

// TestConnection is not yet implemented for Azure DevOps.
func (a *AzureDevOpsProvider) TestConnection(_ context.Context) error {
	return fmt.Errorf("azure devops: TestConnection not implemented")
}

// CreateBranch is not yet implemented for Azure DevOps.
func (a *AzureDevOpsProvider) CreateBranch(_ context.Context, _, _ string) error {
	return fmt.Errorf("azure devops: write operations not implemented")
}

// CreateOrUpdateFile is not yet implemented for Azure DevOps.
func (a *AzureDevOpsProvider) CreateOrUpdateFile(_ context.Context, _ string, _ []byte, _, _ string) error {
	return fmt.Errorf("azure devops: write operations not implemented")
}

// DeleteFile is not yet implemented for Azure DevOps.
func (a *AzureDevOpsProvider) DeleteFile(_ context.Context, _, _, _ string) error {
	return fmt.Errorf("azure devops: write operations not implemented")
}

// CreatePullRequest is not yet implemented for Azure DevOps.
func (a *AzureDevOpsProvider) CreatePullRequest(_ context.Context, _, _, _, _ string) (*PullRequest, error) {
	return nil, fmt.Errorf("azure devops: write operations not implemented")
}
