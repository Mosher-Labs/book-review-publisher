# book-review-publisher

A webhook service that accepts a book review (markdown + cover image URL) and
publishes it to [benniemosher/me](https://github.com/benniemosher/me) by
committing files and opening a pull request via the GitHub API.

## How it works

1. Client sends a `POST /publish` with the post markdown and a cover image URL
2. Service parses Jekyll front matter to derive the date and slug
3. Cover image is downloaded and committed to `assets/img/book-covers/`
4. Markdown file is committed to `_posts/`
5. A pull request is opened in `benniemosher/me`
6. The PR URL is returned in the response

## API

### `POST /publish`

**Request:**

```json
{
  "markdown": "---\ntitle: \"Clean Code\"\ndate: 2024-03-15\n---\n\nPost content here.",
  "image_url": "https://example.com/clean-code-cover.jpg"
}
```

**Response:**

```json
{
  "pr_url": "https://github.com/benniemosher/me/pull/42"
}
```

### `GET /health`

Returns `{"status":"ok"}` when the service is running.

## Local development

```bash
# Install dependencies
go mod download

# Run tests
task test

# Run locally (requires GITHUB_PAT with write access to benniemosher/me)
export GITHUB_PAT=ghp_...
task run

# Send a test request
task publish:example
```

## Deployment

See [RUNBOOK.md](RUNBOOK.md) for deployment instructions including how to
create the required Kubernetes secret.

## References

- [AGENT.md](AGENT.md) - AI agent context for working on this service
- [RUNBOOK.md](RUNBOOK.md) - Operational runbook
