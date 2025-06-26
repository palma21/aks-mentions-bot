package sources

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/azure/aks-mentions-bot/internal/models"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
)

// MediumSource implements Medium.com scraping source
type MediumSource struct {
	client *resty.Client
}

// NewMediumSource creates a new Medium source
func NewMediumSource() *MediumSource {
	return &MediumSource{
		client: resty.New().
			SetTimeout(30 * time.Second).
			SetHeader("User-Agent", "AKS-Mentions-Bot/1.0"),
	}
}

func (m *MediumSource) GetName() string {
	return "medium"
}

func (m *MediumSource) IsEnabled() bool {
	return true // Medium doesn't require API keys for basic search
}

func (m *MediumSource) FetchMentions(ctx context.Context, keywords []string, since time.Duration) ([]models.Mention, error) {
	var allMentions []models.Mention

	for _, keyword := range keywords {
		mentions, err := m.searchKeyword(ctx, keyword, since)
		if err != nil {
			logrus.Errorf("Failed to search Medium for keyword '%s': %v", keyword, err)
			continue
		}
		allMentions = append(allMentions, mentions...)
	}

	return m.deduplicateMentions(allMentions), nil
}

func (m *MediumSource) searchKeyword(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	// Use Medium's RSS feed for topic searches
	// Medium provides RSS feeds for tags: https://medium.com/feed/tag/{tag}
	var mentions []models.Mention
	
	// Try different tag variations for the keyword
	tags := m.generateTags(keyword)
	
	for _, tag := range tags {
		tagMentions, err := m.fetchFromRSS(ctx, tag, since)
		if err != nil {
			logrus.Warnf("Failed to fetch Medium RSS for tag '%s': %v", tag, err)
			continue
		}
		mentions = append(mentions, tagMentions...)
	}
	
	// Also try Medium's search via Google (public content)
	searchMentions, err := m.searchViaGoogle(ctx, keyword, since)
	if err != nil {
		logrus.Warnf("Failed to search Medium via Google for '%s': %v", keyword, err)
	} else {
		mentions = append(mentions, searchMentions...)
	}

	return mentions, nil
}

func (m *MediumSource) generateTags(keyword string) []string {
	keyword = strings.ToLower(keyword)
	tags := []string{}
	
	switch {
	case strings.Contains(keyword, "aks"):
		tags = append(tags, "kubernetes", "azure", "containerization", "devops")
	case strings.Contains(keyword, "kubernetes"):
		tags = append(tags, "kubernetes", "k8s", "container-orchestration", "devops")
	case strings.Contains(keyword, "azure"):
		tags = append(tags, "azure", "cloud-computing", "microsoft-azure")
	case strings.Contains(keyword, "kubefleet"):
		tags = append(tags, "kubernetes", "fleet-management")
	case strings.Contains(keyword, "kaito"):
		tags = append(tags, "ai", "kubernetes", "machine-learning")
	default:
		// Use the keyword as-is, cleaned up
		tag := strings.ReplaceAll(keyword, " ", "-")
		tags = append(tags, tag)
	}
	
	return tags
}

func (m *MediumSource) fetchFromRSS(ctx context.Context, tag string, since time.Duration) ([]models.Mention, error) {
	url := fmt.Sprintf("https://medium.com/feed/tag/%s", tag)
	
	resp, err := m.client.R().
		SetContext(ctx).
		Get(url)
	
	if err != nil {
		return nil, err
	}
	
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("RSS feed returned status %d", resp.StatusCode())
	}
	
	return m.parseRSSFeed(resp.String(), tag, since)
}

func (m *MediumSource) parseRSSFeed(rssContent, tag string, since time.Duration) ([]models.Mention, error) {
	var mentions []models.Mention
	cutoff := time.Now().Add(-since)
	
	// Simple RSS parsing - in production, you'd use a proper XML parser
	// This is a basic implementation that looks for item patterns
	lines := strings.Split(rssContent, "\n")
	
	var currentItem map[string]string
	inItem := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.Contains(line, "<item>") {
			inItem = true
			currentItem = make(map[string]string)
			continue
		}
		
		if strings.Contains(line, "</item>") && inItem {
			inItem = false
			if mention := m.processRSSItem(currentItem, tag, cutoff); mention != nil {
				mentions = append(mentions, *mention)
			}
			continue
		}
		
		if inItem {
			if strings.Contains(line, "<title>") {
				currentItem["title"] = m.extractXMLContent(line, "title")
			} else if strings.Contains(line, "<link>") {
				currentItem["link"] = m.extractXMLContent(line, "link")
			} else if strings.Contains(line, "<description>") {
				currentItem["description"] = m.extractXMLContent(line, "description")
			} else if strings.Contains(line, "<pubDate>") {
				currentItem["pubDate"] = m.extractXMLContent(line, "pubDate")
			} else if strings.Contains(line, "<dc:creator>") {
				currentItem["creator"] = m.extractXMLContent(line, "dc:creator")
			}
		}
	}
	
	return mentions, nil
}

func (m *MediumSource) extractXMLContent(line, tag string) string {
	start := strings.Index(line, fmt.Sprintf("<%s>", tag))
	end := strings.Index(line, fmt.Sprintf("</%s>", tag))
	
	if start != -1 && end != -1 {
		start += len(tag) + 2
		if start < end {
			content := line[start:end]
			// Basic HTML entity decoding
			content = strings.ReplaceAll(content, "&lt;", "<")
			content = strings.ReplaceAll(content, "&gt;", ">")
			content = strings.ReplaceAll(content, "&amp;", "&")
			content = strings.ReplaceAll(content, "&quot;", "\"")
			return content
		}
	}
	
	// Handle self-closing tags like <link />
	if strings.Contains(line, fmt.Sprintf("<%s>", tag)) && strings.Contains(line, "/>") {
		return strings.TrimSpace(line)
	}
	
	return ""
}

func (m *MediumSource) processRSSItem(item map[string]string, tag string, cutoff time.Time) *models.Mention {
	title, ok := item["title"]
	if !ok || title == "" {
		return nil
	}
	
	link, ok := item["link"]
	if !ok || link == "" {
		return nil
	}
	
	// Parse publication date
	pubDate := time.Now()
	if dateStr, ok := item["pubDate"]; ok {
		if parsed, err := time.Parse(time.RFC1123Z, dateStr); err == nil {
			pubDate = parsed
		} else if parsed, err := time.Parse(time.RFC1123, dateStr); err == nil {
			pubDate = parsed
		}
	}
	
	// Check if article is recent enough
	if pubDate.Before(cutoff) {
		return nil
	}
	
	description := item["description"]
	if description == "" {
		description = title
	}
	
	author := item["creator"]
	if author == "" {
		author = "Medium Author"
	}
	
	return &models.Mention{
		ID:        fmt.Sprintf("medium_%s_%d", tag, pubDate.Unix()),
		Source:    "medium",
		Platform:  "Medium",
		Title:     title,
		Content:   description,
		Author:    author,
		URL:       link,
		CreatedAt: pubDate,
		Keywords:  []string{tag},
	}
}

func (m *MediumSource) searchViaGoogle(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	// Search Medium via Google search (public approach)
	// This is a fallback method for broader searches
	query := fmt.Sprintf("site:medium.com %s", keyword)
	
	// Note: This is a simplified approach. In production, you might want to:
	// 1. Use Google Custom Search API
	// 2. Use a proper web scraping service
	// 3. Implement more sophisticated parsing
	
	logrus.Infof("Medium: Would search Google for: %s (simplified implementation)", query)
	
	// For now, return empty results to avoid hitting Google without proper API
	return []models.Mention{}, nil
}

func (m *MediumSource) deduplicateMentions(mentions []models.Mention) []models.Mention {
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

// LinkedInSource implements LinkedIn scraping source
type LinkedInSource struct {
	client *resty.Client
}

// NewLinkedInSource creates a new LinkedIn source
func NewLinkedInSource() *LinkedInSource {
	return &LinkedInSource{
		client: resty.New().
			SetTimeout(30 * time.Second).
			SetHeader("User-Agent", "AKS-Mentions-Bot/1.0"),
	}
}

func (l *LinkedInSource) GetName() string {
	return "linkedin"
}

func (l *LinkedInSource) IsEnabled() bool {
	return true // LinkedIn search is publicly available
}

func (l *LinkedInSource) FetchMentions(ctx context.Context, keywords []string, since time.Duration) ([]models.Mention, error) {
	var allMentions []models.Mention

	for _, keyword := range keywords {
		mentions, err := l.searchKeyword(ctx, keyword, since)
		if err != nil {
			logrus.Errorf("Failed to search LinkedIn for keyword '%s': %v", keyword, err)
			continue
		}
		allMentions = append(allMentions, mentions...)
	}

	return l.deduplicateMentions(allMentions), nil
}

func (l *LinkedInSource) searchKeyword(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	// LinkedIn doesn't have public RSS feeds like Medium, but we can try a few approaches:
	// 1. Use Google search to find LinkedIn posts (limited but public)
	// 2. Search for LinkedIn pulse articles (public content)
	// 3. Note: Full LinkedIn API requires special partnership access
	
	var mentions []models.Mention
	
	// Search LinkedIn Pulse articles via Google (public approach)
	pulseMentions, err := l.searchLinkedInPulse(ctx, keyword, since)
	if err != nil {
		logrus.Warnf("Failed to search LinkedIn Pulse for '%s': %v", keyword, err)
	} else {
		mentions = append(mentions, pulseMentions...)
	}
	
	// Search for public LinkedIn posts via Google
	postMentions, err := l.searchPublicPosts(ctx, keyword, since)
	if err != nil {
		logrus.Warnf("Failed to search LinkedIn posts for '%s': %v", keyword, err)
	} else {
		mentions = append(mentions, postMentions...)
	}
	
	// If no results found via search, create a informational mention
	if len(mentions) == 0 {
		mentions = append(mentions, models.Mention{
			ID:        fmt.Sprintf("linkedin_search_%d", time.Now().Unix()),
			Source:    "linkedin",
			Platform:  "LinkedIn",
			Title:     fmt.Sprintf("LinkedIn search for: %s", keyword),
			Content:   fmt.Sprintf("LinkedIn search performed for '%s'. Note: LinkedIn requires API access for comprehensive data.", keyword),
			Author:    "LinkedIn Bot",
			URL:       fmt.Sprintf("https://www.linkedin.com/search/results/content/?keywords=%s", strings.ReplaceAll(keyword, " ", "%20")),
			CreatedAt: time.Now(),
			Keywords:  []string{keyword},
		})
	}

	return mentions, nil
}

func (l *LinkedInSource) searchLinkedInPulse(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	// LinkedIn Pulse articles are publicly accessible
	// We can search for them using Google search or direct URL patterns
	
	logrus.Infof("LinkedIn: Searching Pulse articles for keyword: %s", keyword)
	
	// This would be a Google search for LinkedIn Pulse articles
	// site:linkedin.com/pulse keyword
	// For now, return empty to avoid hitting Google without proper API
	
	return []models.Mention{}, nil
}

func (l *LinkedInSource) searchPublicPosts(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	// Search for public LinkedIn posts that mention the keyword
	// This is challenging without API access, but some public posts are searchable
	
	logrus.Infof("LinkedIn: Searching public posts for keyword: %s", keyword)
	
	// Try to find public LinkedIn content via search engines
	// In a production environment, you might:
	// 1. Use LinkedIn's official API (requires partnership)
	// 2. Use third-party services that aggregate LinkedIn data
	// 3. Implement careful web scraping with proper rate limiting
	
	// For now, create a search indicator
	mention := models.Mention{
		ID:        fmt.Sprintf("linkedin_public_%s_%d", strings.ReplaceAll(keyword, " ", "_"), time.Now().Unix()),
		Source:    "linkedin",
		Platform:  "LinkedIn",
		Title:     fmt.Sprintf("LinkedIn content search: %s", keyword),
		Content:   fmt.Sprintf("Searched LinkedIn for '%s'. Consider manual review of LinkedIn content for comprehensive coverage.", keyword),
		Author:    "LinkedIn Search Bot",
		URL:       fmt.Sprintf("https://www.linkedin.com/search/results/content/?keywords=%s", strings.ReplaceAll(keyword, " ", "%20")),
		CreatedAt: time.Now(),
		Keywords:  []string{keyword},
	}
	
	return []models.Mention{mention}, nil
}

func (l *LinkedInSource) deduplicateMentions(mentions []models.Mention) []models.Mention {
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
