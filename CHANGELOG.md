# Changelog

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
