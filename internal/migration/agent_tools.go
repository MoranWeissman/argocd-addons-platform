package migration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moran/argocd-addons-platform/internal/ai"
	"github.com/moran/argocd-addons-platform/internal/argocd"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
)

// MigrationToolExecutor handles tool calls from the migration agent.
type MigrationToolExecutor struct {
	newGP     gitprovider.GitProvider
	oldGP     gitprovider.GitProvider
	newArgoCD *argocd.Client
	oldArgoCD *argocd.Client
	logFn     func(repo, action, detail string) // logs to the step log
}

// NewMigrationToolExecutor creates a tool executor for migration operations.
func NewMigrationToolExecutor(
	newGP, oldGP gitprovider.GitProvider,
	newArgoCD, oldArgoCD *argocd.Client,
	logFn func(repo, action, detail string),
) *MigrationToolExecutor {
	return &MigrationToolExecutor{
		newGP: newGP, oldGP: oldGP,
		newArgoCD: newArgoCD, oldArgoCD: oldArgoCD,
		logFn: logFn,
	}
}

// GetMigrationToolDefinitions returns all migration tool definitions for the LLM.
func GetMigrationToolDefinitions() []ai.ToolDefinition {
	return []ai.ToolDefinition{
		// Git — Read
		toolDef("git_read_file", "Read a file from a git repository",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo to read from"},"path":{"type":"string","description":"File path in the repo"},"branch":{"type":"string","description":"Branch name (default: main)"}},"required":["repo","path"]}`),
		toolDef("git_list_directory", "List files and folders in a directory",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo"},"path":{"type":"string","description":"Directory path"}},"required":["repo","path"]}`),
		toolDef("git_list_pull_requests", "List pull requests in a repo",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo"},"state":{"type":"string","enum":["open","closed","all"],"description":"PR state filter (default: open)"}},"required":["repo"]}`),
		toolDef("git_get_pr_status", "Get the status of a specific pull request",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo"},"pr_number":{"type":"integer","description":"Pull request number"}},"required":["repo","pr_number"]}`),

		// Git — Write
		toolDef("git_create_pr", "Create a branch, modify a file, and open a pull request",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo"},"file_path":{"type":"string","description":"Path to the file to modify"},"content":{"type":"string","description":"New file content"},"branch_name":{"type":"string","description":"Branch name to create"},"commit_message":{"type":"string","description":"Commit message"},"pr_title":{"type":"string","description":"PR title"},"pr_body":{"type":"string","description":"PR description"}},"required":["repo","file_path","content","branch_name","commit_message","pr_title","pr_body"]}`),
		toolDef("git_merge_pr", "Approve and merge a pull request",
			`{"type":"object","properties":{"repo":{"type":"string","enum":["new","old"],"description":"Which repo"},"pr_number":{"type":"integer","description":"Pull request number"}},"required":["repo","pr_number"]}`),

		// ArgoCD — Read
		toolDef("argocd_get_app", "Get application status, health, and sync state from ArgoCD",
			`{"type":"object","properties":{"instance":{"type":"string","enum":["new","old"],"description":"Which ArgoCD instance"},"app_name":{"type":"string","description":"Application name"}},"required":["instance","app_name"]}`),
		toolDef("argocd_list_apps", "List all applications in an ArgoCD instance",
			`{"type":"object","properties":{"instance":{"type":"string","enum":["new","old"],"description":"Which ArgoCD instance"}},"required":["instance"]}`),

		// ArgoCD — Write
		toolDef("argocd_sync_app", "Trigger sync on an ArgoCD application",
			`{"type":"object","properties":{"instance":{"type":"string","enum":["new","old"],"description":"Which ArgoCD instance"},"app_name":{"type":"string","description":"Application name"}},"required":["instance","app_name"]}`),
		toolDef("argocd_refresh_app", "Trigger hard refresh on an ArgoCD application",
			`{"type":"object","properties":{"instance":{"type":"string","enum":["new","old"],"description":"Which ArgoCD instance"},"app_name":{"type":"string","description":"Application name"}},"required":["instance","app_name"]}`),

		// Communication
		toolDef("log", "Write a message to the migration step log for the user to see",
			`{"type":"object","properties":{"message":{"type":"string","description":"Human-readable log message"}},"required":["message"]}`),
	}
}

func toolDef(name, description, paramsJSON string) ai.ToolDefinition {
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  json.RawMessage(paramsJSON),
		},
	}
}

// ExecuteTool dispatches a tool call to the appropriate handler.
func (e *MigrationToolExecutor) ExecuteTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	// Git — Read
	case "git_read_file":
		return e.execGitReadFile(ctx, args)
	case "git_list_directory":
		return e.execGitListDir(ctx, args)
	case "git_list_pull_requests":
		return e.execGitListPRs(ctx, args)
	case "git_get_pr_status":
		return e.execGitGetPRStatus(ctx, args)

	// Git — Write
	case "git_create_pr":
		return e.execGitCreatePR(ctx, args)
	case "git_merge_pr":
		return e.execGitMergePR(ctx, args)

	// ArgoCD — Read
	case "argocd_get_app":
		return e.execArgoGetApp(ctx, args)
	case "argocd_list_apps":
		return e.execArgoListApps(ctx, args)

	// ArgoCD — Write
	case "argocd_sync_app":
		return e.execArgoSyncApp(ctx, args)
	case "argocd_refresh_app":
		return e.execArgoRefreshApp(ctx, args)

	// Communication
	case "log":
		return e.execLog(args)

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// --- Git Read Tools ---

func (e *MigrationToolExecutor) resolveGP(repo string) (gitprovider.GitProvider, error) {
	switch repo {
	case "new":
		if e.newGP == nil {
			return nil, fmt.Errorf("NEW repo git provider not configured")
		}
		return e.newGP, nil
	case "old":
		if e.oldGP == nil {
			return nil, fmt.Errorf("OLD repo git provider not configured — configure it in Migration Settings")
		}
		return e.oldGP, nil
	default:
		return nil, fmt.Errorf("repo must be 'new' or 'old', got: %s", repo)
	}
}

func (e *MigrationToolExecutor) resolveArgoCD(instance string) (*argocd.Client, error) {
	switch instance {
	case "new":
		if e.newArgoCD == nil {
			return nil, fmt.Errorf("NEW ArgoCD client not configured")
		}
		return e.newArgoCD, nil
	case "old":
		if e.oldArgoCD == nil {
			return nil, fmt.Errorf("OLD ArgoCD client not configured — configure it in Migration Settings")
		}
		return e.oldArgoCD, nil
	default:
		return nil, fmt.Errorf("instance must be 'new' or 'old', got: %s", instance)
	}
}

func (e *MigrationToolExecutor) execGitReadFile(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo   string `json:"repo"`
		Path   string `json:"path"`
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}
	branch := p.Branch
	if branch == "" {
		branch = "main"
	}
	data, err := gp.GetFileContent(ctx, p.Path, branch)
	if err != nil {
		return fmt.Sprintf("File not found or error reading %s: %s", p.Path, err.Error()), nil
	}
	// Truncate large files
	content := string(data)
	if len(content) > 4000 {
		content = content[:4000] + "\n... (truncated, file is " + fmt.Sprintf("%d", len(data)) + " bytes)"
	}
	return content, nil
}

func (e *MigrationToolExecutor) execGitListDir(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo string `json:"repo"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}
	entries, err := gp.ListDirectory(ctx, p.Path, "main")
	if err != nil {
		return fmt.Sprintf("Error listing directory %s: %s", p.Path, err.Error()), nil
	}
	return fmt.Sprintf("Contents of %s:\n%s", p.Path, joinLines(entries)), nil
}

func (e *MigrationToolExecutor) execGitListPRs(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo  string `json:"repo"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}
	state := p.State
	if state == "" {
		state = "open"
	}
	prs, err := gp.ListPullRequests(ctx, state)
	if err != nil {
		return "", err
	}
	if len(prs) == 0 {
		return "No pull requests found", nil
	}
	var lines []string
	for _, pr := range prs {
		lines = append(lines, fmt.Sprintf("#%d: %s (%s) — %s", pr.ID, pr.Title, pr.Status, pr.URL))
	}
	return joinLines(lines), nil
}

func (e *MigrationToolExecutor) execGitGetPRStatus(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo     string `json:"repo"`
		PRNumber int    `json:"pr_number"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}
	prs, err := gp.ListPullRequests(ctx, "all")
	if err != nil {
		return "", err
	}
	for _, pr := range prs {
		if pr.ID == p.PRNumber {
			return fmt.Sprintf("PR #%d: %s\nStatus: %s\nURL: %s", pr.ID, pr.Title, pr.Status, pr.URL), nil
		}
	}
	return fmt.Sprintf("PR #%d not found", p.PRNumber), nil
}

// --- Git Write Tools ---

func (e *MigrationToolExecutor) execGitCreatePR(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo          string `json:"repo"`
		FilePath      string `json:"file_path"`
		Content       string `json:"content"`
		BranchName    string `json:"branch_name"`
		CommitMessage string `json:"commit_message"`
		PRTitle       string `json:"pr_title"`
		PRBody        string `json:"pr_body"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}

	e.logFn(repoLabel(p.Repo), "creating", "Creating branch and pushing changes...")

	if err := gp.CreateBranch(ctx, p.BranchName, "main"); err != nil {
		return "", fmt.Errorf("creating branch: %w", err)
	}
	if err := gp.CreateOrUpdateFile(ctx, p.FilePath, []byte(p.Content), p.BranchName, p.CommitMessage); err != nil {
		return "", fmt.Errorf("pushing file: %w", err)
	}

	e.logFn(repoLabel(p.Repo), "creating", "Opening pull request...")

	pr, err := gp.CreatePullRequest(ctx, p.PRTitle, p.PRBody, p.BranchName, "main")
	if err != nil {
		return "", fmt.Errorf("creating PR: %w", err)
	}

	e.logFn(repoLabel(p.Repo), "waiting", fmt.Sprintf("PR #%d created: %s", pr.ID, pr.URL))

	return fmt.Sprintf("PR #%d created successfully: %s", pr.ID, pr.URL), nil
}

func (e *MigrationToolExecutor) execGitMergePR(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Repo     string `json:"repo"`
		PRNumber int    `json:"pr_number"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	gp, err := e.resolveGP(p.Repo)
	if err != nil {
		return "", err
	}

	e.logFn(repoLabel(p.Repo), "merging", fmt.Sprintf("Merging PR #%d...", p.PRNumber))

	if err := gp.MergePullRequest(ctx, p.PRNumber); err != nil {
		return "", fmt.Errorf("merging PR #%d: %w", p.PRNumber, err)
	}

	e.logFn(repoLabel(p.Repo), "completed", fmt.Sprintf("PR #%d merged", p.PRNumber))

	return fmt.Sprintf("PR #%d merged successfully", p.PRNumber), nil
}

// --- ArgoCD Read Tools ---

func (e *MigrationToolExecutor) execArgoGetApp(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Instance string `json:"instance"`
		AppName  string `json:"app_name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	ac, err := e.resolveArgoCD(p.Instance)
	if err != nil {
		return "", err
	}
	app, err := ac.GetApplication(ctx, p.AppName)
	if err != nil {
		return fmt.Sprintf("Application %q not found in %s ArgoCD: %s", p.AppName, p.Instance, err.Error()), nil
	}
	return fmt.Sprintf("Application: %s\nSync: %s\nHealth: %s\nNamespace: %s",
		app.Name, app.SyncStatus, app.HealthStatus, app.Namespace), nil
}

func (e *MigrationToolExecutor) execArgoListApps(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Instance string `json:"instance"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	ac, err := e.resolveArgoCD(p.Instance)
	if err != nil {
		return "", err
	}
	apps, err := ac.ListApplications(ctx)
	if err != nil {
		return "", err
	}
	var lines []string
	for _, app := range apps {
		lines = append(lines, fmt.Sprintf("%s (sync=%s, health=%s)", app.Name, app.SyncStatus, app.HealthStatus))
	}
	if len(lines) == 0 {
		return "No applications found", nil
	}
	return joinLines(lines), nil
}

// --- ArgoCD Write Tools ---

func (e *MigrationToolExecutor) execArgoSyncApp(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Instance string `json:"instance"`
		AppName  string `json:"app_name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	ac, err := e.resolveArgoCD(p.Instance)
	if err != nil {
		return "", err
	}

	e.logFn(argoLabel(p.Instance), "syncing", fmt.Sprintf("Triggering sync on %s...", p.AppName))

	if err := ac.SyncApplication(ctx, p.AppName); err != nil {
		return "", fmt.Errorf("sync failed: %w", err)
	}

	e.logFn(argoLabel(p.Instance), "completed", fmt.Sprintf("Sync triggered for %s", p.AppName))

	return fmt.Sprintf("Sync triggered successfully for %s", p.AppName), nil
}

func (e *MigrationToolExecutor) execArgoRefreshApp(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Instance string `json:"instance"`
		AppName  string `json:"app_name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	ac, err := e.resolveArgoCD(p.Instance)
	if err != nil {
		return "", err
	}

	e.logFn(argoLabel(p.Instance), "refreshing", fmt.Sprintf("Hard refresh on %s...", p.AppName))

	app, err := ac.RefreshApplication(ctx, p.AppName, true)
	if err != nil {
		return "", fmt.Errorf("refresh failed: %w", err)
	}

	e.logFn(argoLabel(p.Instance), "completed", fmt.Sprintf("Refresh complete: sync=%s, health=%s", app.SyncStatus, app.HealthStatus))

	return fmt.Sprintf("Refresh complete. Sync: %s, Health: %s", app.SyncStatus, app.HealthStatus), nil
}

// --- Communication ---

func (e *MigrationToolExecutor) execLog(args json.RawMessage) (string, error) {
	var p struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	e.logFn("AGENT", "info", p.Message)
	return "Logged", nil
}

// --- Helpers ---

func repoLabel(repo string) string {
	if repo == "old" {
		return "OLD REPO"
	}
	return "NEW REPO"
}

func argoLabel(instance string) string {
	if instance == "old" {
		return "OLD ARGOCD"
	}
	return "NEW ARGOCD"
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
