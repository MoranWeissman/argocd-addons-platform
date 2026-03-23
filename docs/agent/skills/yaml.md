# YAML Manipulation Knowledge

## Safe Modification Rules
- Never reformat a file you don't fully own — comments and ordering matter for reviewers.
- Prefer targeted line-level edits over full-file rewrites.
- Validate with `yq e '.' file.yaml` (no-op parse) before committing.
- Use `yq` for structural changes; use string replacement only for simple scalar swaps.

## cluster-addons.yaml Label Patterns
Each addon entry is a label-like flag:
```yaml
addons:
  prometheus:
    enabled: true      # include in ApplicationSet rendering
  grafana:
    enabled: false     # excluded — ApplicationSet skips this entry
  loki:
    inMigration: true  # being migrated — special handling (see gitops.md)
```
To enable: set `enabled: true`. To disable: set `enabled: false` or remove the block entirely (behavior depends on ApplicationSet template defaults).

## addons-catalog.yaml Structure
```yaml
applicationsets:
  - name: monitoring          # ApplicationSet name
    appName: prometheus       # generated Application name suffix
    repoURL: https://...
    targetRevision: main
    inMigration: false        # true = migration gate active
    values:
      replicaCount: 2
```
`inMigration: true` signals that the addon is in a transitional state. Do not sync or prune while this flag is set without explicit confirmation.

## Common YAML Gotchas

| Gotcha | Example | Fix |
|--------|---------|-----|
| Boolean vs string | `enabled: yes` parses as `true` | Quote if string intended: `enabled: "yes"` |
| Null vs empty string | `value:` is null; `value: ""` is empty string | Be explicit |
| Multiline strings | `|` preserves newlines; `>` folds to spaces | Use `|` for scripts, `>` for long prose |
| Indentation tabs | YAML disallows tabs | Use spaces only (2-space convention) |
| Duplicate keys | Second key silently wins in most parsers | Lint with `yamllint` |
| Anchor/alias | `&anchor` defines, `*anchor` references | Don't modify anchors without updating all aliases |
| Integer-looking strings | `version: 1.10` may parse as float (1.1) | Quote: `version: "1.10"` |

## yq Patterns for Agent Use
```bash
# Read a value
yq e '.addons.prometheus.enabled' cluster-addons.yaml

# Set a value (in-place)
yq e -i '.addons.prometheus.enabled = true' cluster-addons.yaml

# Set inMigration flag
yq e -i '.applicationsets[] | select(.appName == "prometheus") | .inMigration = false' catalog.yaml

# Add a new addon block
yq e -i '.addons.newaddon = {"enabled": true}' cluster-addons.yaml
```

## Comment Preservation
`yq` v4 preserves comments on read-then-write. `python-yaml` (`PyYAML`) strips comments. If the file has meaningful comments (migration notes, owner annotations), use `yq` or string-based patching — never `json.loads(yaml.dumps(...))`.
