package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/azure/aks-mentions-bot/internal/models"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
)

// HackerNewsSource implements Hacker News API source
type HackerNewsSource struct {
	client *resty.Client
}

type hackerNewsItem struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Text        string `json:"text"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
}

// NewHackerNewsSource creates a new Hacker News source
func NewHackerNewsSource() *HackerNewsSource {
	return &HackerNewsSource{
		client: resty.New().
			SetTimeout(30 * time.Second).
			SetHeader("User-Agent", "AKS-Mentions-Bot/1.0"),
	}
}

func (h *HackerNewsSource) GetName() string {
	return "hackernews"
}

func (h *HackerNewsSource) IsEnabled() bool {
	return true // Hacker News API doesn't require authentication
}

func (h *HackerNewsSource) FetchMentions(ctx context.Context, keywords []string, since time.Duration) ([]models.Mention, error) {
	// Get recent item IDs
	itemIDs, err := h.getRecentItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent items: %w", err)
	}

	var allMentions []models.Mention
	cutoff := time.Now().Add(-since)

	// Limit to avoid too many API calls
	limit := 500
	if len(itemIDs) > limit {
		itemIDs = itemIDs[:limit]
	}

	for _, itemID := range itemIDs {
		select {
		case <-ctx.Done():
			return allMentions, ctx.Err()
		default:
		}

		item, err := h.getItem(ctx, itemID)
		if err != nil {
			logrus.Debugf("Failed to get HN item %d: %v", itemID, err)
			continue
		}

		if item == nil || item.Time == 0 {
			continue
		}

		createdAt := time.Unix(item.Time, 0)
		if createdAt.Before(cutoff) {
			continue
		}

		// Check if the item contains any of our keywords
		content := strings.ToLower(item.Title + " " + item.Text)
		var matchedKeywords []string

		for _, keyword := range keywords {
			if strings.Contains(content, strings.ToLower(keyword)) {
				matchedKeywords = append(matchedKeywords, keyword)
			}
		}

		if len(matchedKeywords) == 0 {
			continue
		}

		mention := models.Mention{
			ID:           fmt.Sprintf("hackernews_%d", item.ID),
			Source:       "hackernews",
			Platform:     "Hacker News",
			Title:        item.Title,
			Content:      item.Text,
			Author:       item.By,
			URL:          fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID),
			CreatedAt:    createdAt,
			Score:        item.Score,
			CommentCount: item.Descendants,
			Keywords:     matchedKeywords,
		}

		// Use external URL if available and it's a story
		if item.Type == "story" && item.URL != "" {
			mention.URL = item.URL
		}

		allMentions = append(allMentions, mention)
	}

	return allMentions, nil
}

func (h *HackerNewsSource) getRecentItems(ctx context.Context) ([]int, error) {
	// Get new stories
	resp, err := h.client.R().
		SetContext(ctx).
		Get("https://hacker-news.firebaseio.com/v0/newstories.json")

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("hacker news API returned status %d", resp.StatusCode())
	}

	var itemIDs []int
	if err := json.Unmarshal(resp.Body(), &itemIDs); err != nil {
		return nil, err
	}

	return itemIDs, nil
}

func (h *HackerNewsSource) getItem(ctx context.Context, itemID int) (*hackerNewsItem, error) {
	resp, err := h.client.R().
		SetContext(ctx).
		Get(fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", itemID))

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("hacker news API returned status %d for item %d", resp.StatusCode(), itemID)
	}

	var item hackerNewsItem
	if err := json.Unmarshal(resp.Body(), &item); err != nil {
		return nil, err
	}

	return &item, nil
}
