package sources

import (
	"testing"

	"github.com/azure/aks-mentions-bot/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestRedditSource_GetName(t *testing.T) {
	source := NewRedditSource("client_id", "client_secret")
	assert.Equal(t, "reddit", source.GetName())
}

func TestRedditSource_IsEnabled(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		expected     bool
	}{
		{
			name:         "Both credentials provided",
			clientID:     "client_id",
			clientSecret: "client_secret",
			expected:     true,
		},
		{
			name:         "Missing client ID",
			clientID:     "",
			clientSecret: "client_secret",
			expected:     false,
		},
		{
			name:         "Missing client secret",
			clientID:     "client_id",
			clientSecret: "",
			expected:     false,
		},
		{
			name:         "Both missing",
			clientID:     "",
			clientSecret: "",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewRedditSource(tt.clientID, tt.clientSecret)
			assert.Equal(t, tt.expected, source.IsEnabled())
		})
	}
}

func TestStackOverflowSource_GetName(t *testing.T) {
	source := NewStackOverflowSource()
	assert.Equal(t, "stackoverflow", source.GetName())
}

func TestStackOverflowSource_IsEnabled(t *testing.T) {
	source := NewStackOverflowSource()
	assert.True(t, source.IsEnabled())
}

func TestStackOverflowSource_stripHTMLTags(t *testing.T) {
	source := NewStackOverflowSource()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic HTML tags",
			input:    "<p>Hello <strong>world</strong></p>",
			expected: "Hello world",
		},
		{
			name:     "Code tags",
			input:    "Use <code>kubectl apply</code> to deploy",
			expected: "Use `kubectl apply` to deploy",
		},
		{
			name:     "Line breaks",
			input:    "Line 1<br>Line 2<br/>Line 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "No HTML tags",
			input:    "Plain text content",
			expected: "Plain text content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := source.stripHTMLTags(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHackerNewsSource_GetName(t *testing.T) {
	source := NewHackerNewsSource()
	assert.Equal(t, "hackernews", source.GetName())
}

func TestHackerNewsSource_IsEnabled(t *testing.T) {
	source := NewHackerNewsSource()
	assert.True(t, source.IsEnabled())
}

func TestTwitterSource_GetName(t *testing.T) {
	source := NewTwitterSource("bearer_token")
	assert.Equal(t, "twitter", source.GetName())
}

func TestTwitterSource_IsEnabled(t *testing.T) {
	tests := []struct {
		name        string
		bearerToken string
		expected    bool
	}{
		{
			name:        "Token provided",
			bearerToken: "bearer_token",
			expected:    true,
		},
		{
			name:        "No token",
			bearerToken: "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewTwitterSource(tt.bearerToken)
			assert.Equal(t, tt.expected, source.IsEnabled())
		})
	}
}

func TestTwitterSource_buildSearchQuery(t *testing.T) {
	source := NewTwitterSource("bearer_token")

	tests := []struct {
		name     string
		keyword  string
		expected string
	}{
		{
			name:     "AKS keyword",
			keyword:  "aks",
			expected: `"aks" (azure OR kubernetes OR microsoft OR container) -rifle -gun -weapon -firearm`,
		},
		{
			name:     "Azure Kubernetes Service",
			keyword:  "azure kubernetes service",
			expected: `"azure kubernetes service" OR "AKS"`,
		},
		{
			name:     "KubeFleet",
			keyword:  "kubefleet",
			expected: `"kubefleet" OR "kube fleet"`,
		},
		{
			name:     "KAITO",
			keyword:  "kaito",
			expected: `"kaito" (kubernetes OR k8s OR azure)`,
		},
		{
			name:     "Other keyword",
			keyword:  "other",
			expected: `"other"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := source.buildSearchQuery(tt.keyword)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestYouTubeSource_GetName(t *testing.T) {
	source := NewYouTubeSource("api_key")
	assert.Equal(t, "youtube", source.GetName())
}

func TestYouTubeSource_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{
			name:     "API key provided",
			apiKey:   "api_key",
			expected: true,
		},
		{
			name:     "No API key",
			apiKey:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewYouTubeSource(tt.apiKey)
			assert.Equal(t, tt.expected, source.IsEnabled())
		})
	}
}

func TestYouTubeSource_extractVideoID(t *testing.T) {
	source := NewYouTubeSource("api_key")

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Standard YouTube URL",
			url:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "YouTube URL with additional parameters",
			url:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=10s",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "Invalid URL",
			url:      "https://example.com/video",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := source.extractVideoID(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeduplicateMentions(t *testing.T) {
	source := NewRedditSource("client_id", "client_secret")

	mentions := []models.Mention{
		{ID: "1", Title: "First mention"},
		{ID: "2", Title: "Second mention"},
		{ID: "1", Title: "Duplicate mention"},
		{ID: "3", Title: "Third mention"},
	}

	unique := source.deduplicateMentions(mentions)

	assert.Len(t, unique, 3)
	assert.Equal(t, "1", unique[0].ID)
	assert.Equal(t, "2", unique[1].ID)
	assert.Equal(t, "3", unique[2].ID)
}
