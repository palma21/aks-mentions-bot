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

// YouTubeSource implements YouTube Data API source
type YouTubeSource struct {
	apiKey string
	client *resty.Client
}

type youTubeSearchResponse struct {
	Items []youTubeVideo `json:"items"`
}

type youTubeVideo struct {
	ID struct {
		VideoID string `json:"videoId"`
	} `json:"id"`
	Snippet struct {
		Title        string `json:"title"`
		Description  string `json:"description"`
		ChannelTitle string `json:"channelTitle"`
		PublishedAt  string `json:"publishedAt"`
		Thumbnails   struct {
			Default struct {
				URL string `json:"url"`
			} `json:"default"`
		} `json:"thumbnails"`
	} `json:"snippet"`
}

type youTubeCommentsResponse struct {
	Items []youTubeComment `json:"items"`
}

type youTubeComment struct {
	ID      string `json:"id"`
	Snippet struct {
		TopLevelComment struct {
			Snippet struct {
				TextDisplay     string `json:"textDisplay"`
				AuthorDisplayName string `json:"authorDisplayName"`
				PublishedAt     string `json:"publishedAt"`
				LikeCount       int    `json:"likeCount"`
			} `json:"snippet"`
		} `json:"topLevelComment"`
	} `json:"snippet"`
}

// NewYouTubeSource creates a new YouTube source
func NewYouTubeSource(apiKey string) *YouTubeSource {
	return &YouTubeSource{
		apiKey: apiKey,
		client: resty.New().
			SetTimeout(30 * time.Second).
			SetHeader("User-Agent", "AKS-Mentions-Bot/1.0"),
	}
}

func (y *YouTubeSource) GetName() string {
	return "youtube"
}

func (y *YouTubeSource) IsEnabled() bool {
	return y.apiKey != ""
}

func (y *YouTubeSource) FetchMentions(ctx context.Context, keywords []string, since time.Duration) ([]models.Mention, error) {
	if !y.IsEnabled() {
		logrus.Debug("YouTube source disabled - missing API key")
		return nil, nil
	}

	var allMentions []models.Mention

	for _, keyword := range keywords {
		// Search for videos
		videoMentions, err := y.searchVideos(ctx, keyword, since)
		if err != nil {
			logrus.Errorf("Failed to search YouTube videos for keyword '%s': %v", keyword, err)
		} else {
			allMentions = append(allMentions, videoMentions...)
		}

		// Search for comments on relevant videos
		commentMentions, err := y.searchComments(ctx, keyword, since)
		if err != nil {
			logrus.Errorf("Failed to search YouTube comments for keyword '%s': %v", keyword, err)
		} else {
			allMentions = append(allMentions, commentMentions...)
		}
	}

	return y.deduplicateMentions(allMentions), nil
}

func (y *YouTubeSource) searchVideos(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	publishedAfter := time.Now().Add(-since).Format(time.RFC3339)
	query := url.QueryEscape(keyword)

	searchURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/search?part=snippet&q=%s&type=video&publishedAfter=%s&maxResults=50&key=%s",
		query, publishedAfter, y.apiKey)

	resp, err := y.client.R().
		SetContext(ctx).
		Get(searchURL)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("youtube API returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var searchResp youTubeSearchResponse
	if err := json.Unmarshal(resp.Body(), &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse YouTube response: %w", err)
	}

	var mentions []models.Mention

	for _, video := range searchResp.Items {
		// Check if the video content contains our keyword (case-insensitive)
		content := strings.ToLower(video.Snippet.Title + " " + video.Snippet.Description)
		if !strings.Contains(content, strings.ToLower(keyword)) {
			continue
		}

		publishedAt, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
		if err != nil {
			logrus.Errorf("Failed to parse YouTube timestamp: %v", err)
			continue
		}

		mention := models.Mention{
			ID:        fmt.Sprintf("youtube_video_%s", video.ID.VideoID),
			Source:    "youtube",
			Platform:  "YouTube",
			Title:     video.Snippet.Title,
			Content:   video.Snippet.Description,
			Author:    video.Snippet.ChannelTitle,
			URL:       fmt.Sprintf("https://www.youtube.com/watch?v=%s", video.ID.VideoID),
			CreatedAt: publishedAt,
			Keywords:  []string{keyword},
		}

		mentions = append(mentions, mention)
	}

	return mentions, nil
}

func (y *YouTubeSource) searchComments(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	// First, find relevant videos to search comments on
	videos, err := y.searchVideos(ctx, "kubernetes azure", since)
	if err != nil {
		return nil, err
	}

	var allComments []models.Mention

	// Limit to avoid too many API calls
	limit := 10
	if len(videos) > limit {
		videos = videos[:limit]
	}

	for _, video := range videos {
		// Extract video ID from URL or use existing ID
		videoID := y.extractVideoID(video.URL)
		if videoID == "" {
			continue
		}

		comments, err := y.getVideoComments(ctx, videoID, keyword)
		if err != nil {
			logrus.Errorf("Failed to get comments for video %s: %v", videoID, err)
			continue
		}

		allComments = append(allComments, comments...)
	}

	return allComments, nil
}

func (y *YouTubeSource) getVideoComments(ctx context.Context, videoID, keyword string) ([]models.Mention, error) {
	commentsURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/commentThreads?part=snippet&videoId=%s&maxResults=100&key=%s",
		videoID, y.apiKey)

	resp, err := y.client.R().
		SetContext(ctx).
		Get(commentsURL)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		// Comments might be disabled, skip this video
		if resp.StatusCode() == 403 {
			return nil, nil
		}
		return nil, fmt.Errorf("youtube comments API returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var commentsResp youTubeCommentsResponse
	if err := json.Unmarshal(resp.Body(), &commentsResp); err != nil {
		return nil, fmt.Errorf("failed to parse YouTube comments response: %w", err)
	}

	var mentions []models.Mention

	for _, comment := range commentsResp.Items {
		commentText := comment.Snippet.TopLevelComment.Snippet.TextDisplay
		
		// Check if the comment contains our keyword (case-insensitive)
		if !strings.Contains(strings.ToLower(commentText), strings.ToLower(keyword)) {
			continue
		}

		publishedAt, err := time.Parse(time.RFC3339, comment.Snippet.TopLevelComment.Snippet.PublishedAt)
		if err != nil {
			logrus.Errorf("Failed to parse YouTube comment timestamp: %v", err)
			continue
		}

		mention := models.Mention{
			ID:        fmt.Sprintf("youtube_comment_%s", comment.ID),
			Source:    "youtube",
			Platform:  "YouTube Comments",
			Title:     fmt.Sprintf("Comment on video %s", videoID),
			Content:   commentText,
			Author:    comment.Snippet.TopLevelComment.Snippet.AuthorDisplayName,
			URL:       fmt.Sprintf("https://www.youtube.com/watch?v=%s&lc=%s", videoID, comment.ID),
			CreatedAt: publishedAt,
			Score:     comment.Snippet.TopLevelComment.Snippet.LikeCount,
			Keywords:  []string{keyword},
		}

		mentions = append(mentions, mention)
	}

	return mentions, nil
}

func (y *YouTubeSource) extractVideoID(url string) string {
	// Extract video ID from YouTube URL
	if strings.Contains(url, "youtube.com/watch?v=") {
		parts := strings.Split(url, "v=")
		if len(parts) > 1 {
			return strings.Split(parts[1], "&")[0]
		}
	}
	return ""
}

func (y *YouTubeSource) deduplicateMentions(mentions []models.Mention) []models.Mention {
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
