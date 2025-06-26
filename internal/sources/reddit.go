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

// RedditSource implements Reddit API source
type RedditSource struct {
	clientID     string
	clientSecret string
	client       *resty.Client
	accessToken  string
}

type redditAuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type redditSearchResponse struct {
	Data struct {
		Children []struct {
			Data redditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type redditPost struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Selftext    string  `json:"selftext"`
	Author      string  `json:"author"`
	Subreddit   string  `json:"subreddit"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	Created     float64 `json:"created_utc"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
}

// NewRedditSource creates a new Reddit source
func NewRedditSource(clientID, clientSecret string) *RedditSource {
	return &RedditSource{
		clientID:     clientID,
		clientSecret: clientSecret,
		client:       resty.New().SetTimeout(30 * time.Second),
	}
}

func (r *RedditSource) GetName() string {
	return "reddit"
}

func (r *RedditSource) IsEnabled() bool {
	return r.clientID != "" && r.clientSecret != ""
}

func (r *RedditSource) FetchMentions(ctx context.Context, keywords []string, since time.Duration) ([]models.Mention, error) {
	if !r.IsEnabled() {
		logrus.Debug("Reddit source disabled - missing credentials")
		return nil, nil
	}

	if err := r.authenticate(); err != nil {
		return nil, fmt.Errorf("reddit authentication failed: %w", err)
	}

	var allMentions []models.Mention
	
	for _, keyword := range keywords {
		mentions, err := r.searchKeyword(ctx, keyword, since)
		if err != nil {
			logrus.Errorf("Failed to search Reddit for keyword '%s': %v", keyword, err)
			continue
		}
		allMentions = append(allMentions, mentions...)
	}

	return r.deduplicateMentions(allMentions), nil
}

func (r *RedditSource) authenticate() error {
	resp, err := r.client.R().
		SetHeader("User-Agent", "AKS-Mentions-Bot/1.0").
		SetBasicAuth(r.clientID, r.clientSecret).
		SetFormData(map[string]string{
			"grant_type": "client_credentials",
		}).
		Post("https://www.reddit.com/api/v1/access_token")

	if err != nil {
		return err
	}

	var authResp redditAuthResponse
	if err := json.Unmarshal(resp.Body(), &authResp); err != nil {
		return err
	}

	r.accessToken = authResp.AccessToken
	return nil
}

func (r *RedditSource) searchKeyword(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	// Search multiple subreddits relevant to Kubernetes/Azure
	subreddits := []string{
		"kubernetes",
		"azure",
		"devops",
		"docker",
		"cloudcomputing",
		"sysadmin",
		"programming",
	}

	var allMentions []models.Mention

	for _, subreddit := range subreddits {
		mentions, err := r.searchSubreddit(ctx, subreddit, keyword, since)
		if err != nil {
			logrus.Errorf("Failed to search subreddit %s: %v", subreddit, err)
			continue
		}
		allMentions = append(allMentions, mentions...)
	}

	return allMentions, nil
}

func (r *RedditSource) searchSubreddit(ctx context.Context, subreddit, keyword string, since time.Duration) ([]models.Mention, error) {
	// Build search query
	query := url.QueryEscape(keyword)
	searchURL := fmt.Sprintf("https://oauth.reddit.com/r/%s/search.json?q=%s&restrict_sr=1&sort=new&limit=100", subreddit, query)

	resp, err := r.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+r.accessToken).
		SetHeader("User-Agent", "AKS-Mentions-Bot/1.0").
		Get(searchURL)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("reddit API returned status %d", resp.StatusCode())
	}

	var searchResp redditSearchResponse
	if err := json.Unmarshal(resp.Body(), &searchResp); err != nil {
		return nil, err
	}

	var mentions []models.Mention
	cutoff := time.Now().Add(-since)

	for _, child := range searchResp.Data.Children {
		post := child.Data
		createdAt := time.Unix(int64(post.Created), 0)

		// Skip posts older than our cutoff
		if createdAt.Before(cutoff) {
			continue
		}

		// Check if the post content contains our keyword (case-insensitive)
		content := strings.ToLower(post.Title + " " + post.Selftext)
		if !strings.Contains(content, strings.ToLower(keyword)) {
			continue
		}

		mention := models.Mention{
			ID:           fmt.Sprintf("reddit_%s", post.ID),
			Source:       "reddit",
			Platform:     fmt.Sprintf("r/%s", post.Subreddit),
			Title:        post.Title,
			Content:      post.Selftext,
			Author:       post.Author,
			URL:          fmt.Sprintf("https://reddit.com%s", post.Permalink),
			CreatedAt:    createdAt,
			Score:        post.Score,
			CommentCount: post.NumComments,
			Keywords:     []string{keyword},
		}

		mentions = append(mentions, mention)
	}

	return mentions, nil
}

func (r *RedditSource) deduplicateMentions(mentions []models.Mention) []models.Mention {
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
