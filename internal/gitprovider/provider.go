package gitprovider

import "context"

// PullRequest represents a pull request from any Git provider.
type PullRequest struct {
	ID           int
	Title        string
	Description  string
	Author       string
	Status       string // "open", "closed", "merged"
	SourceBranch string
	TargetBranch string
	URL          string
	CreatedAt    string
	UpdatedAt    string
	ClosedAt     string
}

// GitProvider defines the operations supported against a Git hosting service.
type GitProvider interface {
	GetFileContent(ctx context.Context, path, ref string) ([]byte, error)
	ListDirectory(ctx context.Context, path, ref string) ([]string, error)
	ListPullRequests(ctx context.Context, state string) ([]PullRequest, error)
	TestConnection(ctx context.Context) error
}
