# AGENT.md - book-review-publisher

## Purpose

HTTP webhook service written in Go that accepts a book review (markdown content
+ cover image URL) and publishes it to `benniemosher/me` by committing files
and opening a GitHub pull request.

## Repository structure

```text
book-review-publisher/
├── cmd/server/main.go          # HTTP server entrypoint, route registration
├── internal/publisher/
│   ├── publisher.go            # Core logic: front matter parsing, GitHub API calls
│   └── publisher_test.go       # Unit tests for parsing and utility functions
├── manifests/                  # Kubernetes resources (deployed via ArgoCD)
│   ├── deployment.yaml
│   ├── ingress.yaml
│   └── service.yaml
├── argocd/application.yaml     # ArgoCD Application (add to homelab-gitops)
├── Dockerfile                  # Multi-stage build: golang:1.25-alpine → alpine:3.22
└── Taskfile.yml                # Common development tasks
```

## Key architecture decisions

- **Standard library HTTP**: Uses `net/http` with Go 1.22+ method+path routing
  (`GET /health`, `POST /publish`) — no external router dependency.
- **PyGitHub equivalent**: Uses `github.com/google/go-github/v72` with
  `golang.org/x/oauth2` for GitHub API authentication.
- **Front matter parser**: Inline parser in `publisher.go` — no external
  dependency. Extracts `title` and `date` fields from Jekyll-style front matter.
- **Branch naming**: `book-review/{date}-{slug}` — one branch per review,
  prevents conflicts.
- **Secret**: `GITHUB_PAT` environment variable, injected from Kubernetes
  secret `book-review-publisher-secrets` (key: `github-pat`).

## Running locally

```bash
export GITHUB_PAT=ghp_your_pat_here
task run
# or: go run ./cmd/server
```

Service listens on `:8080`. Test with:

```bash
task publish:example
```

## Running tests

```bash
task test
# or: go test ./...
```

Tests cover: front matter parsing, slugification, image extension detection.
No GitHub API calls are made in tests (pure unit tests).

## Making changes

- All Go code lives under `internal/publisher/` (business logic) or
  `cmd/server/` (HTTP wiring).
- Add new fields to `publishRequest` struct in `publisher.go` to extend the API.
- Update `parsePostMeta` to extract additional front matter fields.
- Never hardcode `repoOwner`, `repoName`, or `baseBranch` — they are constants
  at the top of `publisher.go`.

## Git workflow

Always use pull requests — never commit directly to `main`. See RUNBOOK.md for
deployment steps after merging.

## Pre-commit hooks

```bash
task lint:install   # one-time setup
task lint           # run all hooks
```

Hooks include: go-fmt, go-vet, go-unit-tests, yamllint, markdownlint,
actionlint, checkov, conventional commits.
