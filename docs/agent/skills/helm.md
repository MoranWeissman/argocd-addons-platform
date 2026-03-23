# Helm Operational Knowledge

## Values Merging Order (last wins)
1. Chart `values.yaml` (defaults)
2. Parent chart `values.yaml` (if subchart)
3. `-f values-override.yaml` files (left to right)
4. `--set key=value` flags (highest precedence)

In ArgoCD, the Helm source `valueFiles` list follows the same left-to-right merging. A `values-production.yaml` passed after `values.yaml` will override defaults.

## Reading a Live Helm Release
```bash
helm get values <release> -n <namespace>          # user-supplied values only
helm get values <release> -n <namespace> --all    # merged effective values
helm get manifest <release> -n <namespace>        # rendered Kubernetes manifests
helm history <release> -n <namespace>             # revision history
```

## Chart Structure Relevant to ArgoCD Addons
```
charts/argocd-addons-platform/
  Chart.yaml          # name, version, appVersion, dependencies
  values.yaml         # defaults (all addons, feature flags)
  values-production.yaml  # production overrides
  templates/
    applicationset.yaml   # ApplicationSet per addon group
    _helpers.tpl          # named templates (labels, selectors)
```

## Common Helm Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `UPGRADE FAILED: another operation in progress` | Previous release is locked | `helm rollback` or delete the secret `sh.helm.release.v1.*` |
| `rendered manifests contain a resource that already exists` | Resource exists outside Helm | Add `--force` or adopt the resource |
| `coalesce.go: cannot overwrite table with non table` | Tried to set a map key as a scalar | Fix the `--set` expression (use dot notation correctly) |
| `no matches for kind X in version Y` | CRD not installed yet | Install CRD chart first; order matters |
| `required value missing` | Template uses `required` and value not set | Provide the value in overrides |

## Values File Gotchas
- Boolean `true`/`false` in YAML is a bool; `"true"` is a string. Helm templates may handle these differently.
- Empty map `{}` vs omitted key: an omitted key means the parent chart default is used; `{}` explicitly overrides to empty.
- `null` in values.yaml removes a key set by a dependency chart.
- Global values: `global.someKey` is accessible in all subcharts as `.Values.global.someKey`.

## Helm in ArgoCD Context
- ArgoCD renders Helm templates server-side and stores the result. `helm template` locally may differ if ArgoCD uses a different Helm version.
- `argocd app diff` shows what ArgoCD thinks would change — use before sync.
- If a value change doesn't appear to take effect, check if the Application has `ignoreDifferences` configured.
- `helm upgrade --dry-run` is safe for validation but does not account for ArgoCD's ignoreDifferences rules.
