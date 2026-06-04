package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"
)

const (
	repoOwner  = "benniemosher"
	repoName   = "benniemosher.com"
	baseBranch = "main"
)

// Publisher handles committing book reviews to the target GitHub repo.
type Publisher struct {
	gh *github.Client
}

// New creates a Publisher authenticated with the given GitHub PAT.
func New(pat string) *Publisher {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: pat})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Publisher{gh: github.NewClient(tc)}
}

type publishRequest struct {
	Markdown string `json:"markdown"`
	ImageURL string `json:"image_url"`
}

type publishResponse struct {
	PRURL string `json:"pr_url"`
}

// HandlePublish is the HTTP handler for POST /publish.
func (p *Publisher) HandlePublish(w http.ResponseWriter, r *http.Request) {
	var req publishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Markdown) == "" || strings.TrimSpace(req.ImageURL) == "" {
		http.Error(w, "markdown and image_url are required", http.StatusBadRequest)
		return
	}

	prURL, err := p.Publish(r.Context(), req.Markdown, req.ImageURL)
	if err != nil {
		slog.Error("publish failed", "error", err)
		http.Error(w, "publish failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(publishResponse{PRURL: prURL}); err != nil {
		slog.Error("encode response failed", "error", err)
	}
}

// Publish commits the markdown post and cover image to benniemosher/benniemosher.com and opens a PR.
func (p *Publisher) Publish(ctx context.Context, markdown, imageURL string) (string, error) {
	return p.publish(ctx, publishRequest{Markdown: markdown, ImageURL: imageURL})
}

func (p *Publisher) publish(ctx context.Context, req publishRequest) (string, error) {
	date, title, slug := parsePostMeta(req.Markdown)
	imageExt := imageExtension(req.ImageURL)

	postPath := fmt.Sprintf("_posts/%s-%s.md", date, slug)
	imagePath := fmt.Sprintf("assets/img/book-covers/%s.%s", slug, imageExt)
	markdown := injectCoverImage(req.Markdown, slug, imageExt)
	branchName := fmt.Sprintf("book-review/%s-%s", date, slug)

	imageData, err := downloadImage(req.ImageURL)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}

	ref, _, err := p.gh.Git.GetRef(ctx, repoOwner, repoName, "heads/"+baseBranch)
	if err != nil {
		return "", fmt.Errorf("get ref: %w", err)
	}

	_, _, err = p.gh.Git.CreateRef(ctx, repoOwner, repoName, &github.Reference{
		Ref:    github.Ptr("refs/heads/" + branchName),
		Object: &github.GitObject{SHA: ref.Object.SHA},
	})
	if err != nil {
		return "", fmt.Errorf("create branch: %w", err)
	}

	_, _, err = p.gh.Repositories.CreateFile(ctx, repoOwner, repoName, imagePath, &github.RepositoryContentFileOptions{
		Message: github.Ptr(fmt.Sprintf("feat: add book cover for %s", slug)),
		Content: imageData,
		Branch:  github.Ptr(branchName),
	})
	if err != nil {
		return "", fmt.Errorf("commit image: %w", err)
	}

	_, _, err = p.gh.Repositories.CreateFile(ctx, repoOwner, repoName, postPath, &github.RepositoryContentFileOptions{
		Message: github.Ptr(fmt.Sprintf("feat: add book review for %s", slug)),
		Content: []byte(markdown),
		Branch:  github.Ptr(branchName),
	})
	if err != nil {
		return "", fmt.Errorf("commit post: %w", err)
	}

	pr, _, err := p.gh.PullRequests.Create(ctx, repoOwner, repoName, &github.NewPullRequest{
		Title: github.Ptr(fmt.Sprintf("feat: add book review - %s", title)),
		Head:  github.Ptr(branchName),
		Base:  github.Ptr(baseBranch),
		Body:  github.Ptr("Auto-generated book review post published via book-review-publisher."),
	})
	if err != nil {
		return "", fmt.Errorf("create pr: %w", err)
	}

	return pr.GetHTMLURL(), nil
}

// injectCoverImage sanitizes the front matter, removes a redundant H2 title
// and any existing markdown image for the cover, then injects a properly-sized
// HTML img tag after the first paragraph so it doesn't end up in the excerpt.
func injectCoverImage(markdown, slug, ext string) string {
	imgTag := fmt.Sprintf(
		`<img src="/assets/img/book-covers/%s.%s" alt="Book cover" `+
			`style="max-width:280px;height:auto;float:right;margin:0 0 1rem 1rem;">`,
		slug, ext,
	)

	body := markdown
	frontMatter := ""
	if strings.HasPrefix(markdown, "---") {
		end := strings.Index(markdown[3:], "---")
		if end >= 0 {
			cut := end + 6
			frontMatter = sanitizeFrontMatter(markdown[:cut])
			body = strings.TrimLeft(markdown[cut:], "\n")
		}
	}

	// Remove a leading H2 that repeats the title (layout renders it already).
	if strings.HasPrefix(body, "## ") {
		if nl := strings.Index(body, "\n"); nl >= 0 {
			body = strings.TrimLeft(body[nl:], "\n")
		}
	}

	// Remove any existing markdown image pointing at the book cover.
	var filtered []string
	for _, l := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(l)
		if strings.Contains(l, "/assets/img/book-covers/"+slug) ||
			(strings.HasPrefix(trimmed, "![") && strings.Contains(l, "book-cover")) {
			continue
		}
		filtered = append(filtered, l)
	}
	body = strings.TrimSpace(strings.Join(filtered, "\n"))

	// Insert the img tag after the first paragraph so the excerpt is text,
	// not an image. If there's no paragraph break, prepend it.
	if sep := strings.Index(body, "\n\n"); sep >= 0 {
		body = body[:sep] + "\n\n" + imgTag + "\n\n" + strings.TrimLeft(body[sep:], "\n")
	} else {
		body = imgTag + "\n\n" + body
	}

	if frontMatter != "" {
		return frontMatter + "\n" + body + "\n"
	}
	return body + "\n"
}

// sanitizeFrontMatter re-emits front matter with safely quoted field values so
// Ruby's YAML parser doesn't choke on colons or double-quotes inside values.
func sanitizeFrontMatter(fm string) string {
	// fm is "---\n...\n---" with no trailing newline required.
	inner := fm
	prefix := ""
	if strings.HasPrefix(fm, "---") {
		inner = fm[3:]
		prefix = "---"
	}
	suffix := ""
	if idx := strings.LastIndex(inner, "---"); idx >= 0 {
		suffix = inner[idx:]
		inner = inner[:idx]
	}

	var out []string

	for _, line := range strings.Split(inner, "\n") {
		colon := strings.Index(line, ": ")
		if colon < 0 {
			out = append(out, line)
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+2:])

		// Convert "categories: foo bar" to a YAML list.
		if key == "categories" &&
			!strings.HasPrefix(val, "-") && !strings.HasPrefix(val, "[") {
			out = append(out, key+":")
			for _, c := range strings.Fields(val) {
				out = append(out, "  - "+c)
			}
			continue
		}

		// Quote values that contain ": " or double-quotes to avoid YAML parse
		// failures in Ruby's Psych parser (the Jekyll YAML backend).
		val = quoteYAMLValue(val)
		out = append(out, key+": "+val)
	}

	return prefix + strings.Join(out, "\n") + suffix
}

func quoteYAMLValue(val string) string {
	if strings.Contains(val, ": ") || strings.Contains(val, `"`) {
		if !strings.HasPrefix(val, `"`) && !strings.HasPrefix(val, "'") {
			return "'" + strings.ReplaceAll(val, "'", "''") + "'"
		}
	}
	return val
}

// parsePostMeta extracts date, title, and slug from Jekyll front matter.
// Falls back to today's date and a generic slug if front matter is absent.
func parsePostMeta(markdown string) (date, title, slug string) {
	date = time.Now().Format("2006-01-02")
	title = "book-review"
	slug = "book-review"

	if !strings.HasPrefix(markdown, "---") {
		return
	}
	end := strings.Index(markdown[3:], "---")
	if end < 0 {
		return
	}
	fm := markdown[3 : end+3]

	for _, line := range strings.Split(fm, "\n") {
		if val, ok := cutFrontMatterField(line, "date"); ok {
			date = val
		}
		if val, ok := cutFrontMatterField(line, "title"); ok {
			title = val
			slug = slugify(val)
		}
	}
	return
}

func cutFrontMatterField(line, key string) (string, bool) {
	after, ok := strings.CutPrefix(line, key+":")
	if !ok {
		return "", false
	}
	val := strings.TrimSpace(after)
	val = strings.Trim(val, `"'`)
	return val, true
}

var nonAlphanumRE = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumRE.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func imageExtension(rawURL string) string {
	u := strings.SplitN(rawURL, "?", 2)[0]
	parts := strings.Split(u, ".")
	ext := strings.ToLower(parts[len(parts)-1])
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif", "avif":
		return ext
	default:
		return "jpg"
	}
}

func downloadImage(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d downloading image", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
