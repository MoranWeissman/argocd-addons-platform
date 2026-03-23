# GitHub API Knowledge

## PR Workflow (Minimal Steps)
1. Create branch from base: `POST /repos/{owner}/{repo}/git/refs`
2. Push file changes: `PUT /repos/{owner}/{repo}/contents/{path}` (with SHA of existing file)
3. Open PR: `POST /repos/{owner}/{repo}/pulls`
4. (Optional) Request review, add labels
5. Merge: `PUT /repos/{owner}/{repo}/pulls/{number}/merge`

Always capture the PR number and URL after creation — needed for status checks and merge.

## Merge Strategies
| Strategy | API `merge_method` | Result |
|----------|-------------------|--------|
| Merge commit | `merge` | Preserves all commits; adds merge commit |
| Squash | `squash` | All commits → one; cleaner history |
| Rebase | `rebase` | Replays commits on base; linear history |

For migration PRs, squash is preferred — one atomic change per addon migration.

## Token Permissions Required
- `repo` scope — full access to private repos (read + write + PRs)
- `contents: write` (fine-grained) — push file changes
- `pull_requests: write` (fine-grained) — open and merge PRs
- `workflows: write` — only if triggering GitHub Actions

If the token has `repo` scope but you get 404 on a private repo, the token's owner may not have access to that repo.

## Common Errors

| HTTP Code | Meaning | Fix |
|-----------|---------|-----|
| `401` | Bad or expired token | Refresh token; check `Authorization: Bearer <token>` header |
| `403` | Token valid, action not permitted | Check token scopes and repo permissions |
| `404` | Repo not found or no access | Confirm repo name and token has `repo` scope |
| `409` | Merge conflict | Resolve conflicts before merging; rebase the branch |
| `422 Unprocessable` | Validation error (e.g., branch already exists, PR already open) | Check `errors[].message` in response body |
| `429` | Rate limit exceeded | Check `X-RateLimit-Remaining` header; wait for reset |

## Rate Limits
- Authenticated: 5000 requests/hour
- Search API: 30 requests/minute (separate limit)
- Check `X-RateLimit-Reset` header (Unix timestamp) before retrying.
- Secondary rate limits apply to rapid POST requests — add small delays between PR operations.

## Branch Protection Effects on Automation
- Required status checks: merge is blocked until CI passes. Poll `GET /repos/{owner}/{repo}/commits/{sha}/check-runs`.
- Required reviews: a human (or a bot with appropriate permissions) must approve. The token can approve if the bot account is not the PR author.
- Dismiss stale reviews: pushing a new commit invalidates previous approvals — time your pushes before requesting review.
- `Allow bypass`: admin tokens can bypass protection; check if your token account has admin access.

## File Update Pattern
When updating an existing file via API, always fetch the current SHA first:
```
GET /repos/{owner}/{repo}/contents/{path}  → response.sha
PUT /repos/{owner}/{repo}/contents/{path}  → body includes { sha: <current_sha> }
```
Missing or wrong SHA → 409 conflict.
