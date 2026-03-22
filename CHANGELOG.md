# Changelog

## [1.26.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.25.0...v1.26.0) (2026-03-22)


### Features

* dev mode flag for credential env var fallback ([4e12f52](https://github.com/MoranWeissman/argocd-addons-platform/commit/4e12f5283e2b0e58071b12be3061a8008af322af))

## [1.25.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.24.5...v1.25.0) (2026-03-22)


### Features

* auto-discover ArgoCD URL and pre-fill connection form ([af41238](https://github.com/MoranWeissman/argocd-addons-platform/commit/af41238ec5490039786067bbd21108bc0afcdbce))

## [1.24.5](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.24.4...v1.24.5) (2026-03-22)


### Bug Fixes

* show actual auth method used when testing connections ([ae2aacb](https://github.com/MoranWeissman/argocd-addons-platform/commit/ae2aacbbde634e71b87160e52e2e3e7ee510ddbc))

## [1.24.4](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.24.3...v1.24.4) (2026-03-22)


### Bug Fixes

* auto-discover ArgoCD server service via K8s API ([b92960e](https://github.com/MoranWeissman/argocd-addons-platform/commit/b92960e8951c1aa414f1ea36c7d1ebc042ab1597))

## [1.24.3](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.24.2...v1.24.3) (2026-03-22)


### Bug Fixes

* ArgoCD connection — env var fallback and better error handling ([bbba134](https://github.com/MoranWeissman/argocd-addons-platform/commit/bbba134efd60ddd232acf94a0849c5688ff65991))

## [1.24.2](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.24.1...v1.24.2) (2026-03-22)


### Bug Fixes

* fallback to GITHUB_TOKEN env var when token not provided ([8077b46](https://github.com/MoranWeissman/argocd-addons-platform/commit/8077b463b1a1ee16cc99832ca7f711d5da6de449))

## [1.24.1](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.24.0...v1.24.1) (2026-03-22)


### Bug Fixes

* add test-credentials debug logging ([c0aef4c](https://github.com/MoranWeissman/argocd-addons-platform/commit/c0aef4c2ec3544da01a40fa512f782d5b37c9f1d))

## [1.24.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.23.0...v1.24.0) (2026-03-21)


### Features

* add K8sStore for encrypted connection storage ([80ff25e](https://github.com/MoranWeissman/argocd-addons-platform/commit/80ff25e92ebd7b4497a5f36a176f2b26caf1d067))
* add searchable autocomplete dropdowns for Azure DevOps project and repo selection ([#34](https://github.com/MoranWeissman/argocd-addons-platform/issues/34)) ([722b267](https://github.com/MoranWeissman/argocd-addons-platform/commit/722b267e8121e299f29791fe4708bd63f29468a3))
* adopt release-please for automated semver bumps ([#23](https://github.com/MoranWeissman/argocd-addons-platform/issues/23)) ([e46280a](https://github.com/MoranWeissman/argocd-addons-platform/commit/e46280a6dd2ecc69de6b708815d27d6c14f5c85d))
* AES-256-GCM encrypted storage for migration credentials ([#40](https://github.com/MoranWeissman/argocd-addons-platform/issues/40)) ([efa60e7](https://github.com/MoranWeissman/argocd-addons-platform/commit/efa60e774763c9adfcdb85f58bfcc126119cc31a))
* Azure DevOps auto-discover projects and repos from PAT + org ([#22](https://github.com/MoranWeissman/argocd-addons-platform/issues/22)) ([07fcbf3](https://github.com/MoranWeissman/argocd-addons-platform/commit/07fcbf3af3809e7fd4d65142456a5c7707039269))
* configure AI provider via Settings UI ([bdbbd09](https://github.com/MoranWeissman/argocd-addons-platform/commit/bdbbd0996db659532b99192f1a2e51d79ca45eb7))
* descriptive step names, PR merge button, YOLO auto-merge, universal resume ([#61](https://github.com/MoranWeissman/argocd-addons-platform/issues/61)) ([6868ab7](https://github.com/MoranWeissman/argocd-addons-platform/commit/6868ab74f580975ff9546c20efb0b2ee3239fb8b))
* **helm:** add RBAC and encryption key for connection Secret store ([aa83f27](https://github.com/MoranWeissman/argocd-addons-platform/commit/aa83f2763d4d195a608106bd4d42c1228915ffde))
* **helm:** remove connection ConfigMap, use K8s Secret store ([77a0f2e](https://github.com/MoranWeissman/argocd-addons-platform/commit/77a0f2e2ead64c0b42626dc739e4d0b534d8dc9c))
* **helm:** simplify values — connections managed via UI ([032a0aa](https://github.com/MoranWeissman/argocd-addons-platform/commit/032a0aa95fb019157ab475aa7ed8f78a1805b603))
* migration dialog with scope selection and addon/cluster discovery from OLD repo ([#38](https://github.com/MoranWeissman/argocd-addons-platform/issues/38)) ([416812b](https://github.com/MoranWeissman/argocd-addons-platform/commit/416812bf895c4ceecc372f9317f9cd0290ef85c1))
* migration wizard v2 — live logs, YOLO/gates mode, session management ([#51](https://github.com/MoranWeissman/argocd-addons-platform/issues/51)) ([c318a62](https://github.com/MoranWeissman/argocd-addons-platform/commit/c318a6230740da37b83c525b9c867529aa99fe10))
* pipeline-style layout with side-by-side stages and logs ([#55](https://github.com/MoranWeissman/argocd-addons-platform/issues/55)) ([2958d45](https://github.com/MoranWeissman/argocd-addons-platform/commit/2958d4516b4beae4ccafaff5962f7676f8c16521))
* redesign connection form with test-before-save UX ([cd8eb17](https://github.com/MoranWeissman/argocd-addons-platform/commit/cd8eb1731bacd93dcdd56afd25c1296d1c8e9c74))
* update login page with high-res background and dynamic cover ([#37](https://github.com/MoranWeissman/argocd-addons-platform/issues/37)) ([75214d8](https://github.com/MoranWeissman/argocd-addons-platform/commit/75214d8c7494954eddfe560b4296fa01b798b182))
* use K8sStore for connections in Kubernetes mode ([dde11c5](https://github.com/MoranWeissman/argocd-addons-platform/commit/dde11c5bc5a2fb4577fbe517fde07b69a4248870))


### Bug Fixes

* add resourceVersion optimistic concurrency and extra tests ([8ed0924](https://github.com/MoranWeissman/argocd-addons-platform/commit/8ed0924db45c81b593afa8438c3d31cf9fa1a323))
* add VERSION to release-please extra-files so it gets updated on every release ([#46](https://github.com/MoranWeissman/argocd-addons-platform/issues/46)) ([30f2026](https://github.com/MoranWeissman/argocd-addons-platform/commit/30f2026b4ae09f93630d7a9ea64c6b2459403d33))
* auto-derive connection name from git repo path ([12131b4](https://github.com/MoranWeissman/argocd-addons-platform/commit/12131b46875e30a5e13b6567d6ccc797718a65dd))
* connection update, delete migration, per-step logs, branch cleanup ([#63](https://github.com/MoranWeissman/argocd-addons-platform/issues/63)) ([fec7a3a](https://github.com/MoranWeissman/argocd-addons-platform/commit/fec7a3a4c47622c6f044fd83216808f971af8210))
* let release-please update Helm chart version via extra-files ([#29](https://github.com/MoranWeissman/argocd-addons-platform/issues/29)) ([07118cc](https://github.com/MoranWeissman/argocd-addons-platform/commit/07118ccd71d3dbe75eba8cbdf20bf49672017190))
* merge release-please and build into single workflow ([#25](https://github.com/MoranWeissman/argocd-addons-platform/issues/25)) ([edf1afd](https://github.com/MoranWeissman/argocd-addons-platform/commit/edf1afd1249cf95fec683e183c0ee885318fa0a0))
* migration executor using request context and nil providers ([#42](https://github.com/MoranWeissman/argocd-addons-platform/issues/42)) ([f0e03d8](https://github.com/MoranWeissman/argocd-addons-platform/commit/f0e03d8a972985889b644eadc422422bd939b420))
* migration resume, compact pipeline UI, step timeouts, panic recovery ([#57](https://github.com/MoranWeissman/argocd-addons-platform/issues/57)) ([2a20522](https://github.com/MoranWeissman/argocd-addons-platform/commit/2a20522637f446c3128ff827b04b3ba0c10c3653))
* migration settings — data volume, separate connection errors, clear credentials ([#32](https://github.com/MoranWeissman/argocd-addons-platform/issues/32)) ([9b9de5a](https://github.com/MoranWeissman/argocd-addons-platform/commit/9b9de5ae4a89815c5df7087624fb4b3aa8b0bbb8))
* persist migration state in K8s ConfigMaps (survives pod restarts) ([#53](https://github.com/MoranWeissman/argocd-addons-platform/issues/53)) ([07a4422](https://github.com/MoranWeissman/argocd-addons-platform/commit/07a442265bf5b712f3b376957e67492d318087ec))
* read version from release-please manifest instead of stale VERSION file ([#35](https://github.com/MoranWeissman/argocd-addons-platform/issues/35)) ([aea26cb](https://github.com/MoranWeissman/argocd-addons-platform/commit/aea26cbe44d3aefba703d603adbed06920519152))
* rename VERSION to version.txt for release-please compatibility ([#49](https://github.com/MoranWeissman/argocd-addons-platform/issues/49)) ([846552b](https://github.com/MoranWeissman/argocd-addons-platform/commit/846552b4707dec803c2813338f765f48e841bd31))
* resume from any state, wider logs panel, remove cancel button ([#59](https://github.com/MoranWeissman/argocd-addons-platform/issues/59)) ([b219e86](https://github.com/MoranWeissman/argocd-addons-platform/commit/b219e86dc143e88ded170b75d27f22569c2a50e7))
* sync VERSION file to 1.17.1 (release-please not updating it) ([#44](https://github.com/MoranWeissman/argocd-addons-platform/issues/44)) ([e9bebc2](https://github.com/MoranWeissman/argocd-addons-platform/commit/e9bebc254195d08cd596a43329a2eaad6c463df4))
* URL-encode connection name in update/delete API calls ([#65](https://github.com/MoranWeissman/argocd-addons-platform/issues/65)) ([9ed5816](https://github.com/MoranWeissman/argocd-addons-platform/commit/9ed58166d7e2cc58c948b8dcd0d40c11daa782df))
* use fetch origin main for Helm chart update step ([#27](https://github.com/MoranWeissman/argocd-addons-platform/issues/27)) ([4b7e8ae](https://github.com/MoranWeissman/argocd-addons-platform/commit/4b7e8aecbeb760c93657c17e5dd7ca66aa1ac284))
* use sessionStorage instead of localStorage for AI chat persistence ([8a108b0](https://github.com/MoranWeissman/argocd-addons-platform/commit/8a108b0131fdadd7f2f63bc05a8318fdf2ae8c81))

## [1.23.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.22.0...v1.23.0) (2026-03-21)


### Features

* configure AI provider via Settings UI ([bdbbd09](https://github.com/MoranWeissman/argocd-addons-platform/commit/bdbbd0996db659532b99192f1a2e51d79ca45eb7))

## [1.22.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.21.1...v1.22.0) (2026-03-20)


### Features

* redesign connection form with test-before-save UX ([cd8eb17](https://github.com/MoranWeissman/argocd-addons-platform/commit/cd8eb1731bacd93dcdd56afd25c1296d1c8e9c74))

## [1.21.1](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.21.0...v1.21.1) (2026-03-20)


### Bug Fixes

* auto-derive connection name from git repo path ([12131b4](https://github.com/MoranWeissman/argocd-addons-platform/commit/12131b46875e30a5e13b6567d6ccc797718a65dd))

## [1.21.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.20.2...v1.21.0) (2026-03-20)


### Features

* add K8sStore for encrypted connection storage ([80ff25e](https://github.com/MoranWeissman/argocd-addons-platform/commit/80ff25e92ebd7b4497a5f36a176f2b26caf1d067))
* **helm:** add RBAC and encryption key for connection Secret store ([aa83f27](https://github.com/MoranWeissman/argocd-addons-platform/commit/aa83f2763d4d195a608106bd4d42c1228915ffde))
* **helm:** remove connection ConfigMap, use K8s Secret store ([77a0f2e](https://github.com/MoranWeissman/argocd-addons-platform/commit/77a0f2e2ead64c0b42626dc739e4d0b534d8dc9c))
* **helm:** simplify values — connections managed via UI ([032a0aa](https://github.com/MoranWeissman/argocd-addons-platform/commit/032a0aa95fb019157ab475aa7ed8f78a1805b603))
* use K8sStore for connections in Kubernetes mode ([dde11c5](https://github.com/MoranWeissman/argocd-addons-platform/commit/dde11c5bc5a2fb4577fbe517fde07b69a4248870))


### Bug Fixes

* add resourceVersion optimistic concurrency and extra tests ([8ed0924](https://github.com/MoranWeissman/argocd-addons-platform/commit/8ed0924db45c81b593afa8438c3d31cf9fa1a323))

## [1.20.2](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.20.1...v1.20.2) (2026-03-20)


### Bug Fixes

* URL-encode connection name in update/delete API calls ([#65](https://github.com/MoranWeissman/argocd-addons-platform/issues/65)) ([9ed5816](https://github.com/MoranWeissman/argocd-addons-platform/commit/9ed58166d7e2cc58c948b8dcd0d40c11daa782df))

## [1.20.1](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.20.0...v1.20.1) (2026-03-20)


### Bug Fixes

* connection update, delete migration, per-step logs, branch cleanup ([#63](https://github.com/MoranWeissman/argocd-addons-platform/issues/63)) ([fec7a3a](https://github.com/MoranWeissman/argocd-addons-platform/commit/fec7a3a4c47622c6f044fd83216808f971af8210))

## [1.20.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.19.2...v1.20.0) (2026-03-19)


### Features

* descriptive step names, PR merge button, YOLO auto-merge, universal resume ([#61](https://github.com/MoranWeissman/argocd-addons-platform/issues/61)) ([6868ab7](https://github.com/MoranWeissman/argocd-addons-platform/commit/6868ab74f580975ff9546c20efb0b2ee3239fb8b))

## [1.19.2](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.19.1...v1.19.2) (2026-03-19)


### Bug Fixes

* resume from any state, wider logs panel, remove cancel button ([#59](https://github.com/MoranWeissman/argocd-addons-platform/issues/59)) ([b219e86](https://github.com/MoranWeissman/argocd-addons-platform/commit/b219e86dc143e88ded170b75d27f22569c2a50e7))

## [1.19.1](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.19.0...v1.19.1) (2026-03-19)


### Bug Fixes

* migration resume, compact pipeline UI, step timeouts, panic recovery ([#57](https://github.com/MoranWeissman/argocd-addons-platform/issues/57)) ([2a20522](https://github.com/MoranWeissman/argocd-addons-platform/commit/2a20522637f446c3128ff827b04b3ba0c10c3653))

## [1.19.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.18.1...v1.19.0) (2026-03-19)


### Features

* pipeline-style layout with side-by-side stages and logs ([#55](https://github.com/MoranWeissman/argocd-addons-platform/issues/55)) ([2958d45](https://github.com/MoranWeissman/argocd-addons-platform/commit/2958d4516b4beae4ccafaff5962f7676f8c16521))

## [1.18.1](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.18.0...v1.18.1) (2026-03-19)


### Bug Fixes

* persist migration state in K8s ConfigMaps (survives pod restarts) ([#53](https://github.com/MoranWeissman/argocd-addons-platform/issues/53)) ([07a4422](https://github.com/MoranWeissman/argocd-addons-platform/commit/07a442265bf5b712f3b376957e67492d318087ec))

## [1.18.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.17.2...v1.18.0) (2026-03-19)


### Features

* migration wizard v2 — live logs, YOLO/gates mode, session management ([#51](https://github.com/MoranWeissman/argocd-addons-platform/issues/51)) ([c318a62](https://github.com/MoranWeissman/argocd-addons-platform/commit/c318a6230740da37b83c525b9c867529aa99fe10))

## [1.17.2](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.17.1...v1.17.2) (2026-03-19)


### Bug Fixes

* add VERSION to release-please extra-files so it gets updated on every release ([#46](https://github.com/MoranWeissman/argocd-addons-platform/issues/46)) ([30f2026](https://github.com/MoranWeissman/argocd-addons-platform/commit/30f2026b4ae09f93630d7a9ea64c6b2459403d33))
* rename VERSION to version.txt for release-please compatibility ([#49](https://github.com/MoranWeissman/argocd-addons-platform/issues/49)) ([846552b](https://github.com/MoranWeissman/argocd-addons-platform/commit/846552b4707dec803c2813338f765f48e841bd31))
* sync VERSION file to 1.17.1 (release-please not updating it) ([#44](https://github.com/MoranWeissman/argocd-addons-platform/issues/44)) ([e9bebc2](https://github.com/MoranWeissman/argocd-addons-platform/commit/e9bebc254195d08cd596a43329a2eaad6c463df4))

## [1.17.1](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.17.0...v1.17.1) (2026-03-19)


### Bug Fixes

* migration executor using request context and nil providers ([#42](https://github.com/MoranWeissman/argocd-addons-platform/issues/42)) ([f0e03d8](https://github.com/MoranWeissman/argocd-addons-platform/commit/f0e03d8a972985889b644eadc422422bd939b420))

## [1.17.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.16.0...v1.17.0) (2026-03-19)


### Features

* AES-256-GCM encrypted storage for migration credentials ([#40](https://github.com/MoranWeissman/argocd-addons-platform/issues/40)) ([efa60e7](https://github.com/MoranWeissman/argocd-addons-platform/commit/efa60e774763c9adfcdb85f58bfcc126119cc31a))

## [1.16.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.15.0...v1.16.0) (2026-03-19)


### Features

* migration dialog with scope selection and addon/cluster discovery from OLD repo ([#38](https://github.com/MoranWeissman/argocd-addons-platform/issues/38)) ([416812b](https://github.com/MoranWeissman/argocd-addons-platform/commit/416812bf895c4ceecc372f9317f9cd0290ef85c1))

## [1.15.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.14.0...v1.15.0) (2026-03-19)


### Features

* update login page with high-res background and dynamic cover ([#37](https://github.com/MoranWeissman/argocd-addons-platform/issues/37)) ([75214d8](https://github.com/MoranWeissman/argocd-addons-platform/commit/75214d8c7494954eddfe560b4296fa01b798b182))


### Bug Fixes

* read version from release-please manifest instead of stale VERSION file ([#35](https://github.com/MoranWeissman/argocd-addons-platform/issues/35)) ([aea26cb](https://github.com/MoranWeissman/argocd-addons-platform/commit/aea26cbe44d3aefba703d603adbed06920519152))

## [1.14.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.13.2...v1.14.0) (2026-03-19)


### Features

* add searchable autocomplete dropdowns for Azure DevOps project and repo selection ([#34](https://github.com/MoranWeissman/argocd-addons-platform/issues/34)) ([722b267](https://github.com/MoranWeissman/argocd-addons-platform/commit/722b267e8121e299f29791fe4708bd63f29468a3))


### Bug Fixes

* migration settings — data volume, separate connection errors, clear credentials ([#32](https://github.com/MoranWeissman/argocd-addons-platform/issues/32)) ([9b9de5a](https://github.com/MoranWeissman/argocd-addons-platform/commit/9b9de5ae4a89815c5df7087624fb4b3aa8b0bbb8))

## [1.13.2](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.13.1...v1.13.2) (2026-03-19)


### Bug Fixes

* let release-please update Helm chart version via extra-files ([#29](https://github.com/MoranWeissman/argocd-addons-platform/issues/29)) ([07118cc](https://github.com/MoranWeissman/argocd-addons-platform/commit/07118ccd71d3dbe75eba8cbdf20bf49672017190))

## [1.13.1](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.13.0...v1.13.1) (2026-03-19)


### Bug Fixes

* use fetch origin main for Helm chart update step ([#27](https://github.com/MoranWeissman/argocd-addons-platform/issues/27)) ([4b7e8ae](https://github.com/MoranWeissman/argocd-addons-platform/commit/4b7e8aecbeb760c93657c17e5dd7ca66aa1ac284))

## [1.13.0](https://github.com/MoranWeissman/argocd-addons-platform/compare/v1.12.0...v1.13.0) (2026-03-19)


### Features

* adopt release-please for automated semver bumps ([#23](https://github.com/MoranWeissman/argocd-addons-platform/issues/23)) ([e46280a](https://github.com/MoranWeissman/argocd-addons-platform/commit/e46280a6dd2ecc69de6b708815d27d6c14f5c85d))
* Azure DevOps auto-discover projects and repos from PAT + org ([#22](https://github.com/MoranWeissman/argocd-addons-platform/issues/22)) ([07fcbf3](https://github.com/MoranWeissman/argocd-addons-platform/commit/07fcbf3af3809e7fd4d65142456a5c7707039269))


### Bug Fixes

* merge release-please and build into single workflow ([#25](https://github.com/MoranWeissman/argocd-addons-platform/issues/25)) ([edf1afd](https://github.com/MoranWeissman/argocd-addons-platform/commit/edf1afd1249cf95fec683e183c0ee885318fa0a0))
