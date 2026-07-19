# book-review-publisher - Project Memory

## Project overview

Go HTTP webhook service that publishes book reviews to `benniemosher/me`.
Accepts markdown + image URL, commits both files to the target repo, and opens
a pull request via the GitHub API.

**Key details:**

- **Language:** Go 1.25
- **Framework:** `net/http` standard library (no external router)
- **GitHub client:** `github.com/google/go-github/v89`
- **Deployment:** k3s via ArgoCD (GitOps)
- **Secret:** `GITHUB_PAT` env var from Kubernetes sealed secret

## Repository structure

```text
book-review-publisher/
├── cmd/server/main.go          # Entrypoint, route registration
├── internal/publisher/
│   ├── publisher.go            # Core logic
│   └── publisher_test.go       # Unit tests
├── manifests/                  # Kubernetes manifests (ArgoCD source)
├── argocd/application.yaml     # ArgoCD Application (lives in homelab-gitops)
├── Dockerfile                  # Multi-stage Go build
└── Taskfile.yml                # Developer tasks
```

## Development workflow

```bash
# Run tests
task test

# Run locally
export GITHUB_PAT=ghp_...
task run

# Format code
task fmt

# Run all linters
task lint
```

## Git workflow

**CRITICAL: Always use pull requests. Never commit directly to main.**

1. `git checkout -b feat/description`
2. Make changes (write linter-compliant code from the start)
3. `pre-commit run --all-files`
4. `git commit -m "feat: description"`
5. `gh pr create`

**Commit format:** Conventional Commits

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `chore:` maintenance
- `refactor:` restructuring

## Pre-commit hooks

```bash
task lint:install   # one-time setup
task lint           # run manually
```

## Code standards

- Line length: 120 chars for Markdown, 80 for YAML (default yamllint)
- Go: `gofmt` formatted, `go vet` clean
- All YAML uses 2-space indentation
- One Kubernetes resource per file in `manifests/`

## Common pitfalls

- Do not add `//nolint` directives without a comment explaining why
- Do not commit a plain (unsealed) Kubernetes secret — always use kubeseal
- The `argocd/application.yaml` must be copied to `homelab-gitops`, not
  applied directly from this repo
- If adding new dependencies, run `go mod tidy` before committing

## References

- [AGENT.md](AGENT.md) - Architecture details for AI agents
- [RUNBOOK.md](RUNBOOK.md) - Deployment and operational procedures
- [go-github docs](https://pkg.go.dev/github.com/google/go-github/v89/github)
- [homelab-gitops](https://github.com/Mosher-Labs/homelab-gitops)

---

**Last updated:** 2026-06-04
