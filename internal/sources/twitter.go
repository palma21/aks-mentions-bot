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

// TwitterSource implements Twitter/X API source
type TwitterSource struct {
	bearerToken string
	client      *resty.Client
}

type twitterSearchResponse struct {
	Data []twitterTweet `json:"data"`
	Meta struct {
		ResultCount int    `json:"result_count"`
		NextToken   string `json:"next_token"`
	} `json:"meta"`
}

type twitterTweet struct {
	ID           string `json:"id"`
	Text         string `json:"text"`
	AuthorID     string `json:"author_id"`
	CreatedAt    string `json:"created_at"`
	PublicMetrics struct {
		RetweetCount int `json:"retweet_count"`
		LikeCount    int `json:"like_count"`
		ReplyCount   int `json:"reply_count"`
	} `json:"public_metrics"`
	ReferencedTweets []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"referenced_tweets"`
}

// NewTwitterSource creates a new Twitter source
func NewTwitterSource(bearerToken string) *TwitterSource {
	return &TwitterSource{
		bearerToken: bearerToken,
		client: resty.New().
			SetTimeout(30 * time.Second).
			SetHeader("User-Agent", "AKS-Mentions-Bot/1.0"),
	}
}

func (t *TwitterSource) GetName() string {
	return "twitter"
}

func (t *TwitterSource) IsEnabled() bool {
	return t.bearerToken != ""
}

func (t *TwitterSource) FetchMentions(ctx context.Context, keywords []string, since time.Duration) ([]models.Mention, error) {
	if !t.IsEnabled() {
		logrus.Debug("Twitter source disabled - missing bearer token")
		return nil, nil
	}

	var allMentions []models.Mention

	for _, keyword := range keywords {
		mentions, err := t.searchKeyword(ctx, keyword, since)
		if err != nil {
			logrus.Errorf("Failed to search Twitter for keyword '%s': %v", keyword, err)
			continue
		}
		allMentions = append(allMentions, mentions...)
	}

	return t.deduplicateMentions(allMentions), nil
}

func (t *TwitterSource) searchKeyword(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	// Build search query
	startTime := time.Now().Add(-since).Format(time.RFC3339)
	
	// Create a more specific query to avoid false positives
	query := t.buildSearchQuery(keyword)
	encodedQuery := url.QueryEscape(query)

	searchURL := fmt.Sprintf("https://api.twitter.com/2/tweets/search/recent?query=%s&start_time=%s&max_results=100&tweet.fields=created_at,author_id,public_metrics,referenced_tweets",
		encodedQuery, startTime)

	resp, err := t.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+t.bearerToken).
		Get(searchURL)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("twitter API returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var searchResp twitterSearchResponse
	if err := json.Unmarshal(resp.Body(), &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse Twitter response: %w", err)
	}

	var mentions []models.Mention

	for _, tweet := range searchResp.Data {
		// Skip retweets to avoid duplicates
		if t.isRetweet(tweet) {
			continue
		}

		createdAt, err := time.Parse(time.RFC3339, tweet.CreatedAt)
		if err != nil {
			logrus.Errorf("Failed to parse Twitter timestamp: %v", err)
			continue
		}

		mention := models.Mention{
			ID:           fmt.Sprintf("twitter_%s", tweet.ID),
			Source:       "twitter",
			Platform:     "X.com (Twitter)",
			Title:        "", // Twitter doesn't have titles
			Content:      tweet.Text,
			Author:       tweet.AuthorID, // In production, you'd resolve this to username
			URL:          fmt.Sprintf("https://twitter.com/i/status/%s", tweet.ID),
			CreatedAt:    createdAt,
			Score:        tweet.PublicMetrics.LikeCount,
			CommentCount: tweet.PublicMetrics.ReplyCount,
			Keywords:     []string{keyword},
		}

		mentions = append(mentions, mention)
	}

	return mentions, nil
}

func (t *TwitterSource) buildSearchQuery(keyword string) string {
	// Build a more sophisticated query to reduce false positives
	switch strings.ToLower(keyword) {
	case "aks":
		// For AKS, make sure we exclude weapon-related content
		return fmt.Sprintf(`"%s" (azure OR kubernetes OR microsoft OR container) -rifle -gun -weapon -firearm`, keyword)
	case "azure kubernetes service":
		return fmt.Sprintf(`"%s" OR "AKS"`, keyword)
	case "kubefleet":
		return fmt.Sprintf(`"%s" OR "kube fleet"`, keyword)
	case "kaito":
		return fmt.Sprintf(`"%s" (kubernetes OR k8s OR azure)`, keyword)
	default:
		return fmt.Sprintf(`"%s"`, keyword)
	}
}

func (t *TwitterSource) isRetweet(tweet twitterTweet) bool {
	for _, ref := range tweet.ReferencedTweets {
		if ref.Type == "retweeted" {
			return true
		}
	}
	return false
}

func (t *TwitterSource) deduplicateMentions(mentions []models.Mention) []models.Mention {
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
