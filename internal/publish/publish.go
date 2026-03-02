package publish

import "context"

// Snippet is the content to publish.
type Snippet struct {
	Title    string // display title
	Filename string // e.g. "export.md" (determines rendering on host)
	Content  string // raw file content
}

// Result is returned after successful publish.
type Result struct {
	URL string // shareable URL of the published snippet
}

// Publisher publishes a snippet to a hosting service.
type Publisher interface {
	Publish(ctx context.Context, s Snippet) (*Result, error)
}
