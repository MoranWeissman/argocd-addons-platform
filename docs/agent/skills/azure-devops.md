# Azure DevOps Git Knowledge

## API Differences vs GitHub
- Base URL: `https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}/...`
- Auth: `Authorization: Basic <base64(:PAT)>` — note the colon prefix before PAT.
- PRs use `pullRequests` not `pulls`. Vote and complete are separate calls.
- File updates use `pushes` API (batch commits), not per-file `contents` endpoint.

## PR Lifecycle

### Create PR
```
POST /_apis/git/repositories/{repo}/pullRequests?api-version=7.1
Body: { sourceRefName, targetRefName, title, description }
Response: pullRequestId
```

### Approve (Vote)
```
PUT /_apis/git/repositories/{repo}/pullRequests/{id}/reviewers/{reviewer-id}?api-version=7.1
Body: { vote: 10 }   // 10=approve, 5=approve with suggestions, -10=reject
```
The `reviewer-id` must be the object ID (GUID) of the user/service account, not their email.

### Complete (Merge)
```
PATCH /_apis/git/repositories/{repo}/pullRequests/{id}?api-version=7.1
Body: { status: "completed", completionOptions: { mergeStrategy: "squash", deleteSourceBranch: true } }
```
Note: `PATCH` to complete, not `PUT`. The PR must be in `Active` state and all policies satisfied.

## Branch Policies
- Required reviewers, build validation, comment resolution — all block completion.
- To bypass, the token's account must have "Bypass policies" permission on the branch.
- Bypassing is set at the repository security level, not via API at call time.

## PAT Scopes Required
| Operation | Scope |
|-----------|-------|
| Read repo | `Code (Read)` |
| Push commits, create PRs | `Code (Read & Write)` |
| Read/write work items | `Work Items (Read & Write)` |
| Read build status | `Build (Read)` |

PATs expire. An expired PAT returns an HTML login page (200 OK, Content-Type: text/html) instead of JSON — detect this by checking `Content-Type` before parsing.

## Common Errors

| Symptom | Cause | Fix |
|---------|-------|-----|
| Response is HTML login page | PAT expired or missing | Refresh PAT; check `Authorization` header format |
| `TF401027: You need the Git 'GenericContribute'` | Branch policy blocks direct push | Open a PR instead of pushing directly |
| `GitPushRefDoesNotExist` | Source branch doesn't exist | Create the branch before pushing |
| `400 Bad Request` on PR complete | Policy not satisfied or wrong merge strategy | Check active policies; use `bypassReason` if bypass is permitted |
| PR stuck in `Active` after vote | Another required reviewer hasn't voted | Check `reviewers[].vote` on the PR object |

## V1 vs V2 Repo Structures
- **V1** (Old ArgoCD / Azure DevOps): `cluster-addons.yaml` and `values/` directory live in an Azure DevOps Git repo. PAT auth, Azure DevOps PR workflow.
- **V2** (New ArgoCD / GitHub): Same logical structure but in a GitHub repo. GitHub token auth, GitHub PR workflow.

Migration means: read V1 values from Azure DevOps → transform → write V2 values to GitHub. Never write back to V1 during migration — it is the source of truth until cutover.

## Push Commits (Files) API
Azure DevOps doesn't have a per-file update endpoint. Use the `pushes` API:
```
POST /_apis/git/repositories/{repo}/pushes?api-version=7.1
Body: {
  refUpdates: [{ name: "refs/heads/{branch}", oldObjectId: "{current-tip-sha}" }],
  commits: [{ comment: "msg", changes: [{ changeType: "edit", item: { path }, newContent: { content, contentType: "rawtext" } }] }]
}
```
Get `oldObjectId` from `GET /refs?filter=heads/{branch}` → `value[0].objectId`.
