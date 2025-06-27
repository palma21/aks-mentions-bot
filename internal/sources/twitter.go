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
	ID            string `json:"id"`
	Text          string `json:"text"`
	AuthorID      string `json:"author_id"`
	CreatedAt     string `json:"created_at"`
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
			SetTimeout(30*time.Second).
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
	searchedKeywords := make(map[string]bool)

	for i, keyword := range keywords {
		// Skip redundant keywords that are already covered by combined searches
		if strings.ToLower(keyword) == "azure kubernetes service" {
			if searchedKeywords["aks"] {
				logrus.Debugf("Skipping '%s' - already covered by AKS search", keyword)
				continue
			}
		}

		query := t.buildSearchQuery(keyword)
		if query == "" {
			logrus.Debugf("Skipping keyword '%s' - covered by previous search", keyword)
			continue
		}

		// Add delay between keyword searches to avoid rate limiting
		if i > 0 {
			logrus.Debugf("Adding 3-second delay before searching for keyword '%s' to avoid Twitter rate limits", keyword)
			select {
			case <-ctx.Done():
				return allMentions, ctx.Err()
			case <-time.After(3 * time.Second):
				// Continue with next keyword
			}
		}

		logrus.Infof("Searching Twitter for keyword: %s", keyword)
		mentions, err := t.searchKeyword(ctx, keyword, since)
		if err != nil {
			logrus.Errorf("Failed to search Twitter for keyword '%s': %v", keyword, err)
			// Continue with other keywords instead of failing completely
			continue
		}

		logrus.Infof("Found %d mentions on Twitter for keyword '%s'", len(mentions), keyword)
		allMentions = append(allMentions, mentions...)
		searchedKeywords[strings.ToLower(keyword)] = true
	}

	deduplicated := t.deduplicateMentions(allMentions)
	logrus.Infof("Total Twitter mentions after deduplication: %d", len(deduplicated))

	return deduplicated, nil
}

func (t *TwitterSource) searchKeyword(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	// Build search query
	startTime := time.Now().Add(-since).Format(time.RFC3339)

	// Create a more specific query to avoid false positives
	query := t.buildSearchQuery(keyword)
	encodedQuery := url.QueryEscape(query)

	searchURL := fmt.Sprintf("https://api.twitter.com/2/tweets/search/recent?query=%s&start_time=%s&max_results=100&tweet.fields=created_at,author_id,public_metrics,referenced_tweets",
		encodedQuery, startTime)

	logrus.Debugf("Twitter API request for keyword '%s': %s", keyword, searchURL)

	resp, err := t.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+t.bearerToken).
		Get(searchURL)

	if err != nil {
		return nil, err
	}

	// Handle rate limiting (429) - fail fast to avoid blocking other sources
	if resp.StatusCode() == 429 {
		logrus.Warnf("Twitter API rate limit hit for keyword '%s' - skipping to avoid blocking other sources", keyword)

		// Get rate limit reset time from headers
		resetTime := resp.Header().Get("x-rate-limit-reset")
		if resetTime != "" {
			logrus.Infof("Twitter rate limit will reset at: %s", resetTime)
		}

		// Return empty results instead of waiting - this allows other sources to complete
		// and notifications to be sent for mentions found from other sources
		logrus.Infof("Skipping Twitter search for keyword '%s' due to rate limit - other sources can still provide mentions", keyword)
		return []models.Mention{}, nil
	}

	if resp.StatusCode() != 200 {
		logrus.Errorf("Twitter API error for keyword '%s': status %d, body: %s", keyword, resp.StatusCode(), string(resp.Body()))
		return nil, fmt.Errorf("twitter API returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var searchResp twitterSearchResponse
	if err := json.Unmarshal(resp.Body(), &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse Twitter response: %w", err)
	}

	logrus.Infof("Twitter API returned %d tweets for keyword '%s'", len(searchResp.Data), keyword)

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

// searchKeywordWithRetry performs a retry with limited attempts
func (t *TwitterSource) searchKeywordWithRetry(ctx context.Context, keyword string, since time.Duration, attempt int) ([]models.Mention, error) {
	if attempt > 2 {
		logrus.Errorf("Twitter API max retries exceeded for keyword '%s'", keyword)
		return nil, fmt.Errorf("twitter API max retries exceeded for keyword '%s'", keyword)
	}

	startTime := time.Now().Add(-since).Format(time.RFC3339)
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

	if resp.StatusCode() == 429 {
		logrus.Warnf("Twitter API still rate limited on retry %d for keyword '%s' - skipping", attempt, keyword)
		// Return empty results instead of failing completely
		return []models.Mention{}, nil
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
			Title:        "",
			Content:      tweet.Text,
			Author:       tweet.AuthorID,
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
	// Also combine related keywords to reduce API calls
	switch strings.ToLower(keyword) {
	case "aks":
		// For AKS, make sure we exclude weapon-related content
		return fmt.Sprintf(`("%s" OR "Azure Kubernetes Service") (azure OR kubernetes OR microsoft OR container OR k8s) -rifle -gun -weapon -firearm -AK47`, keyword)
	case "azure kubernetes service":
		// Skip this if we already searched for "aks" - they're combined above
		return ""
	case "azure kubernetes fleet manager", "kubefleet":
		return fmt.Sprintf(`("Azure Kubernetes Fleet Manager" OR "KubeFleet" OR "kube fleet") (azure OR kubernetes)`)
	case "kaito":
		return fmt.Sprintf(`"%s" (kubernetes OR k8s OR azure OR "AI inference")`, keyword)
	case "azure container service":
		return fmt.Sprintf(`"%s" (azure OR container OR microsoft)`, keyword)
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
