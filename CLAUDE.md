# ngcore

## Commit convention

Write all commit messages following the Angular convention:
`<type>(<scope>): <summary>`, types: build/ci/docs/feat/fix/perf/refactor/test/style/chore/revert.
Header max 100 chars. Scope is the package name (e.g. `ngp2p`, `ngtypes`) when applicable.
Enforced by `.githooks/commit-msg` (`core.hooksPath` is set to `.githooks`).
Do not add Co-Authored-By lines.

Commit author email: me@c0mm4nd.com
