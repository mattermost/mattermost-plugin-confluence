package serializer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double slash in path",
			input:    "https://host.example.com//spaces/VAUL/pages/123",
			expected: "https://host.example.com/spaces/VAUL/pages/123",
		},
		{
			name:     "multiple double slashes",
			input:    "https://host.example.com//spaces//VAUL//pages/123",
			expected: "https://host.example.com/spaces/VAUL/pages/123",
		},
		{
			name:     "trailing slash only",
			input:    "https://host.example.com/",
			expected: "https://host.example.com/",
		},
		{
			name:     "no trailing slash",
			input:    "https://host.example.com",
			expected: "https://host.example.com",
		},
		{
			name:     "normal URL with path",
			input:    "https://host.example.com/spaces/VAUL",
			expected: "https://host.example.com/spaces/VAUL",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "   ",
		},
		{
			name:     "URL with query params",
			input:    "https://host.example.com//spaces/VAUL?param=value",
			expected: "https://host.example.com/spaces/VAUL?param=value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeURLs(t *testing.T) {
	event := &ConfluenceServerEvent{
		BaseURL: "https://host.example.com//",
		User: &ConfluenceServerUser{
			URL: "https://host.example.com//users/test",
		},
		Space: ConfluenceServerSpace{
			URL: "https://host.example.com//spaces/TEST",
		},
		Page: &ConfluenceServerPage{
			URL:     "https://host.example.com//pages/123",
			TinyURL: "https://host.example.com//x/abc",
			EditURL: "https://host.example.com//pages/123/edit",
			Ancestors: []ConfluenceServerPageAncestor{
				{URL: "https://host.example.com//pages/parent"},
			},
		},
		Comment: &ConfluenceServerComment{
			URL: "https://host.example.com//comments/456",
			ParentComment: &ConfluenceServerParentComment{
				URL: "https://host.example.com//comments/parent",
			},
		},
		Blog: &ConfluenceServerBlogPost{
			URL: "https://host.example.com//blog/789",
		},
	}

	event.sanitizeURLs()

	assert.Equal(t, "https://host.example.com/", event.BaseURL)
	assert.Equal(t, "https://host.example.com/users/test", event.User.URL)
	assert.Equal(t, "https://host.example.com/spaces/TEST", event.Space.URL)
	assert.Equal(t, "https://host.example.com/pages/123", event.Page.URL)
	assert.Equal(t, "https://host.example.com/x/abc", event.Page.TinyURL)
	assert.Equal(t, "https://host.example.com/pages/123/edit", event.Page.EditURL)
	assert.Equal(t, "https://host.example.com/pages/parent", event.Page.Ancestors[0].URL)
	assert.Equal(t, "https://host.example.com/comments/456", event.Comment.URL)
	assert.Equal(t, "https://host.example.com/comments/parent", event.Comment.ParentComment.URL)
	assert.Equal(t, "https://host.example.com/blog/789", event.Blog.URL)
}
