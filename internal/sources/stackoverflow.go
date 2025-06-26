package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/azure/aks-mentions-bot/internal/models"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
)

// StackOverflowSource implements Stack Overflow API source
type StackOverflowSource struct {
	client *resty.Client
}

type stackOverflowResponse struct {
	Items []stackOverflowQuestion `json:"items"`
}

type stackOverflowQuestion struct {
	QuestionID      int      `json:"question_id"`
	Title           string   `json:"title"`
	Body            string   `json:"body"`
	Tags            []string `json:"tags"`
	Owner           struct {
		DisplayName string `json:"display_name"`
	} `json:"owner"`
	CreationDate    int64  `json:"creation_date"`
	Score           int    `json:"score"`
	ViewCount       int    `json:"view_count"`
	AnswerCount     int    `json:"answer_count"`
	Link            string `json:"link"`
	IsAnswered      bool   `json:"is_answered"`
}

// NewStackOverflowSource creates a new Stack Overflow source
func NewStackOverflowSource() *StackOverflowSource {
	return &StackOverflowSource{
		client: resty.New().
			SetTimeout(30 * time.Second).
			SetHeader("User-Agent", "AKS-Mentions-Bot/1.0"),
	}
}

func (s *StackOverflowSource) GetName() string {
	return "stackoverflow"
}

func (s *StackOverflowSource) IsEnabled() bool {
	return true // Stack Overflow API doesn't require authentication for basic searches
}

func (s *StackOverflowSource) FetchMentions(ctx context.Context, keywords []string, since time.Duration) ([]models.Mention, error) {
	var allMentions []models.Mention

	for _, keyword := range keywords {
		mentions, err := s.searchKeyword(ctx, keyword, since)
		if err != nil {
			logrus.Errorf("Failed to search Stack Overflow for keyword '%s': %v", keyword, err)
			continue
		}
		allMentions = append(allMentions, mentions...)
	}

	return s.deduplicateMentions(allMentions), nil
}

func (s *StackOverflowSource) searchKeyword(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	fromDate := time.Now().Add(-since).Unix()
	
	// Build search query with relevant tags
	query := url.QueryEscape(keyword)
	tags := []string{"azure", "kubernetes", "docker", "containers", "devops"}
	
	searchURL := fmt.Sprintf("https://api.stackexchange.com/2.3/search/advanced?order=desc&sort=creation&q=%s&tagged=%s&site=stackoverflow&fromdate=%d&pagesize=100&filter=withbody",
		query, strings.Join(tags, ";"), fromDate)

	resp, err := s.client.R().
		SetContext(ctx).
		Get(searchURL)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("stack overflow API returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var searchResp stackOverflowResponse
	if err := json.Unmarshal(resp.Body(), &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse Stack Overflow response: %w", err)
	}

	var mentions []models.Mention

	for _, question := range searchResp.Items {
		// Check if the question content contains our keyword (case-insensitive)
		content := strings.ToLower(question.Title + " " + question.Body)
		if !strings.Contains(content, strings.ToLower(keyword)) {
			continue
		}

		createdAt := time.Unix(question.CreationDate, 0)

		mention := models.Mention{
			ID:           fmt.Sprintf("stackoverflow_%d", question.QuestionID),
			Source:       "stackoverflow",
			Platform:     "Stack Overflow",
			Title:        question.Title,
			Content:      s.stripHTMLTags(question.Body),
			Author:       question.Owner.DisplayName,
			URL:          question.Link,
			CreatedAt:    createdAt,
			Score:        question.Score,
			CommentCount: question.AnswerCount,
			Keywords:     []string{keyword},
		}

		mentions = append(mentions, mention)
	}

	return mentions, nil
}

func (s *StackOverflowSource) stripHTMLTags(content string) string {
	// Basic HTML tag removal - in production, use a proper HTML parser
	content = strings.ReplaceAll(content, "<p>", "\n")
	content = strings.ReplaceAll(content, "</p>", "\n")
	content = strings.ReplaceAll(content, "<br>", "\n")
	content = strings.ReplaceAll(content, "<br/>", "\n")
	content = strings.ReplaceAll(content, "<code>", "`")
	content = strings.ReplaceAll(content, "</code>", "`")
	
	// Remove other HTML tags
	for strings.Contains(content, "<") && strings.Contains(content, ">") {
		start := strings.Index(content, "<")
		end := strings.Index(content, ">")
		if start < end {
			content = content[:start] + content[end+1:]
		} else {
			break
		}
	}
	
	return strings.TrimSpace(content)
}

func (s *StackOverflowSource) deduplicateMentions(mentions []models.Mention) []models.Mention {
	seen := make(map[string]bool)
	var unique []models.Mention

	for _, mention := range mentions {
		if !seen[mention.ID] {
			seen[mention.ID] = true
			unique = append(unique, mention)
		}
	}

	return unique
}
