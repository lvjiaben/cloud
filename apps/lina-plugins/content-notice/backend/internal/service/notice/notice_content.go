// This file removes executable HTML while preserving normal announcement rich text and images.

package notice

import "github.com/microcosm-cc/bluemonday"

// contentPolicy is configured once and only read after construction.
var contentPolicy = newContentPolicy()

// newContentPolicy preserves the rich-text editor's formatting with validated CSS values.
func newContentPolicy() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("style").OnElements("span", "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "img", "table", "td", "th")
	policy.AllowStyles("color", "background-color", "text-align", "font-size", "font-family", "font-weight", "line-height", "width", "height").Globally()
	return policy
}

// sanitizeContent applies the same policy to stored and historically saved announcement HTML.
func sanitizeContent(value string) string { return contentPolicy.Sanitize(value) }
