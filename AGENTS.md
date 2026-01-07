# AGENTS.md - Project Conventions

## Workflow Rules

> **CRITICAL: Follow these rules strictly**

1. **ALWAYS work on only ONE task at a time** - Complete it fully before moving to the next
2. **COMMIT to git after EACH task is FINISHED**
3. **ALWAYS ASK USER FOR FEEDBACK before committing**

---

## Go Standards

- Go version: **^1.25**
- Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- Use `golangci-lint` with project config

## Development Workflow

- **TDD is mandatory**: Write tests BEFORE implementation
- Every feature requires unit tests (minimum 80% coverage)
- Use table-driven tests for multiple scenarios
- **ALWAYS use the Makefile** for running tests, build, lint, etc.

## Makefile Commands

| Command             | Purpose         |
|---------------------|-----------------|
| `make build`        | Build binary    |
| `make test`         | Run all tests   |
| `make lint`         | Run linters     |
| `make docker-build` | Build container |
| `make helm-lint`    | Lint Helm chart |

## Documentation

- All docs in `docs/` directory
- Keep `README.md` up to date with features and Mermaid diagrams
- Update `docs/metrics.md` for metric changes

## Git Conventions

- Use conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`
- Use **Semantic Versioning 2.0** (https://semver.org): `MAJOR.MINOR.PATCH`
- PRs require passing CI and review
- Update `CHANGELOG.md` for user-facing changes
