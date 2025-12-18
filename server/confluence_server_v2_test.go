package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		path     string
		expected string
	}{
		{
			name:     "base with trailing slash, path with leading slash",
			baseURL:  "https://host.example.com/",
			path:     "/spaces/VAUL/pages/123",
			expected: "https://host.example.com/spaces/VAUL/pages/123",
		},
		{
			name:     "base without trailing slash, path with leading slash",
			baseURL:  "https://host.example.com",
			path:     "/spaces/VAUL/pages/123",
			expected: "https://host.example.com/spaces/VAUL/pages/123",
		},
		{
			name:     "base with trailing slash, path without leading slash",
			baseURL:  "https://host.example.com/",
			path:     "spaces/VAUL/pages/123",
			expected: "https://host.example.com/spaces/VAUL/pages/123",
		},
		{
			name:     "base without trailing slash, path without leading slash",
			baseURL:  "https://host.example.com",
			path:     "spaces/VAUL/pages/123",
			expected: "https://host.example.com/spaces/VAUL/pages/123",
		},
		{
			name:     "empty path",
			baseURL:  "https://host.example.com/",
			path:     "",
			expected: "https://host.example.com/",
		},
		{
			name:     "path is just a slash",
			baseURL:  "https://host.example.com",
			path:     "/",
			expected: "https://host.example.com/",
		},
		{
			name:     "real confluence page path",
			baseURL:  "https://confluence.example.com/",
			path:     "/spaces/VAUL/pages/954892292/Testing+MM+Stage+and+Conf+Stage",
			expected: "https://confluence.example.com/spaces/VAUL/pages/954892292/Testing+MM+Stage+and+Conf+Stage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinURL(tt.baseURL, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

