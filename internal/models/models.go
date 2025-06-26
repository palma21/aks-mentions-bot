package models

import "time"

// Mention represents a mention found across various platforms
type Mention struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"`       // "reddit", "stackoverflow", "hackernews", etc.
	Platform    string    `json:"platform"`     // URL or platform identifier
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Author      string    `json:"author"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"created_at"`
	Sentiment   string    `json:"sentiment"`    // "positive", "negative", "neutral"
	Score       int       `json:"score"`        // upvotes, likes, etc.
	CommentCount int      `json:"comment_count"`
	Keywords    []string  `json:"keywords"`     // Keywords that matched
	Relevance   float64   `json:"relevance"`    // Relevance score (0-1)
}

// Report represents a periodic report of mentions
type Report struct {
	GeneratedAt   time.Time              `json:"generated_at"`
	Period        string                 `json:"period"`        // "daily" or "weekly"
	TotalMentions int                    `json:"total_mentions"`
	Mentions      []Mention              `json:"mentions"`
	Summary       map[string]interface{} `json:"summary"`
}

// Alert represents an urgent notification
type Alert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`      // "critical", "urgent", "info"
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Mention   *Mention  `json:"mention,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
