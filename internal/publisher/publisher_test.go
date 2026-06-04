package publisher

import (
	"testing"
)

func TestParsePostMeta(t *testing.T) {
	t.Run("valid front matter", func(t *testing.T) {
		md := "---\ntitle: \"The Pragmatic Programmer\"\ndate: 2024-03-15\n---\n\nContent here."
		date, title, slug := parsePostMeta(md)
		if date != "2024-03-15" {
			t.Errorf("date = %q, want %q", date, "2024-03-15")
		}
		if title != "The Pragmatic Programmer" {
			t.Errorf("title = %q, want %q", title, "The Pragmatic Programmer")
		}
		if slug != "the-pragmatic-programmer" {
			t.Errorf("slug = %q, want %q", slug, "the-pragmatic-programmer")
		}
	})

	t.Run("no front matter falls back to defaults", func(t *testing.T) {
		_, _, slug := parsePostMeta("just some content with no front matter")
		if slug != "book-review" {
			t.Errorf("slug = %q, want %q", slug, "book-review")
		}
	})
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"The Pragmatic Programmer", "the-pragmatic-programmer"},
		{"Clean Code: A Handbook", "clean-code-a-handbook"},
		{"  spaces  ", "spaces"},
		{"Go in Action!", "go-in-action"},
	}
	for _, c := range cases {
		got := slugify(c.input)
		if got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestImageExtension(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://example.com/cover.png", "png"},
		{"https://example.com/cover.jpg?size=large", "jpg"},
		{"https://example.com/cover.JPEG", "jpeg"},
		{"https://example.com/cover.unknown", "jpg"},
		{"https://example.com/cover", "jpg"},
	}
	for _, c := range cases {
		got := imageExtension(c.url)
		if got != c.want {
			t.Errorf("imageExtension(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}
