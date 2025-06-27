package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	
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
		// Be specific to Azure Kubernetes Service
		tags = append(tags, "azure", "microsoft-azure", "aks")
	case strings.Contains(keyword, "azure kubernetes service"):
		tags = append(tags, "azure", "microsoft-azure", "aks", "azure-kubernetes-service")
	case strings.Contains(keyword, "azure kubernetes"):
		tags = append(tags, "azure", "microsoft-azure", "aks")
	case strings.Contains(keyword, "azure container service"):
		tags = append(tags, "azure", "microsoft-azure", "containerization")
	case strings.Contains(keyword, "kubefleet"):
		tags = append(tags, "azure", "kubernetes", "fleet-management")
	case strings.Contains(keyword, "kaito"):
		tags = append(tags, "azure", "ai", "kubernetes", "machine-learning")
	default:
		// For any other keyword, try to make it Azure-specific
		if strings.Contains(keyword, "azure") {
			tag := strings.ReplaceAll(keyword, " ", "-")
			tags = append(tags, tag, "azure", "microsoft-azure")
		} else {
			// Don't search for non-Azure keywords
			return tags // Return empty slice
		}
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
	return false // LinkedIn implementation is placeholder only - not fetching real content
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
	
	// Try to search LinkedIn Pulse articles
	return l.scrapeLinkedInContent(ctx, keyword, "pulse", since)
}

func (l *LinkedInSource) searchPublicPosts(ctx context.Context, keyword string, since time.Duration) ([]models.Mention, error) {
	// Search for public LinkedIn posts that mention the keyword
	
	logrus.Infof("LinkedIn: Searching public posts for keyword: %s", keyword)
	
	// Try to search public LinkedIn posts
	return l.scrapeLinkedInContent(ctx, keyword, "posts", since)
}

func (l *LinkedInSource) scrapeLinkedInContent(ctx context.Context, keyword string, contentType string, since time.Duration) ([]models.Mention, error) {
	var mentions []models.Mention
	
	// LinkedIn search URL
	var searchURL string
	if contentType == "pulse" {
		// Search specifically for Pulse articles
		searchURL = fmt.Sprintf("https://www.linkedin.com/search/results/content/?keywords=%s+pulse", url.QueryEscape(keyword))
	} else {
		// Search for general content
		searchURL = fmt.Sprintf("https://www.linkedin.com/search/results/content/?keywords=%s", url.QueryEscape(keyword))
	}
	
	logrus.Infof("Scraping LinkedIn %s for keyword: %s", contentType, keyword)
	
	// Create HTTP client with proper headers to mimic a browser
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return mentions, fmt.Errorf("failed to create LinkedIn request: %w", err)
	}
	
	// Set browser-like headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	
	resp, err := client.Do(req)
	if err != nil {
		logrus.Warnf("Failed to fetch LinkedIn search results for keyword '%s': %v", keyword, err)
		// Return a placeholder mention indicating the search was attempted
		mention := models.Mention{
			ID:        fmt.Sprintf("linkedin_%s_%s_%d", contentType, strings.ReplaceAll(keyword, " ", "_"), time.Now().Unix()),
			Source:    "linkedin",
			Platform:  "LinkedIn",
			Title:     fmt.Sprintf("LinkedIn %s search attempted: %s", contentType, keyword),
			Content:   fmt.Sprintf("Attempted to search LinkedIn %s for '%s' but encountered connection issues. Manual review recommended.", contentType, keyword),
			Author:    "LinkedIn Search Bot",
			URL:       searchURL,
			CreatedAt: time.Now().UTC(),
			Keywords:  []string{keyword},
		}
		return []models.Mention{mention}, nil
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		logrus.Warnf("LinkedIn returned status %d for keyword '%s'", resp.StatusCode, keyword)
		// Return a placeholder mention indicating the search was attempted
		mention := models.Mention{
			ID:        fmt.Sprintf("linkedin_%s_%s_%d", contentType, strings.ReplaceAll(keyword, " ", "_"), time.Now().Unix()),
			Source:    "linkedin",
			Platform:  "LinkedIn", 
			Title:     fmt.Sprintf("LinkedIn %s search: %s (Status: %d)", contentType, keyword, resp.StatusCode),
			Content:   fmt.Sprintf("Searched LinkedIn %s for '%s' but received status %d. This may indicate rate limiting or access restrictions. Consider manual review.", contentType, keyword, resp.StatusCode),
			Author:    "LinkedIn Search Bot",
			URL:       searchURL,
			CreatedAt: time.Now().UTC(),
			Keywords:  []string{keyword},
		}
		return []models.Mention{mention}, nil
	}
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mentions, fmt.Errorf("failed to read LinkedIn response: %w", err)
	}
	
	// Parse HTML content
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return mentions, fmt.Errorf("failed to parse LinkedIn HTML: %w", err)
	}
	
	// Extract posts from LinkedIn's HTML structure
	posts := l.extractLinkedInPosts(doc, keyword, contentType, since)
	
	if len(posts) == 0 {
		logrus.Infof("No LinkedIn %s found for keyword '%s', returning search indicator", contentType, keyword)
		// Return a search indicator if no posts were found
		mention := models.Mention{
			ID:        fmt.Sprintf("linkedin_%s_%s_%d", contentType, strings.ReplaceAll(keyword, " ", "_"), time.Now().Unix()),
			Source:    "linkedin",
			Platform:  "LinkedIn",
			Title:     fmt.Sprintf("LinkedIn %s search: %s", contentType, keyword),
			Content:   fmt.Sprintf("Searched LinkedIn %s for '%s'. Content may be available but requires authentication or manual review.", contentType, keyword),
			Author:    "LinkedIn Search Bot",
			URL:       searchURL,
			CreatedAt: time.Now().UTC(),
			Keywords:  []string{keyword},
		}
		mentions = append(mentions, mention)
	} else {
		mentions = append(mentions, posts...)
	}
	
	logrus.Infof("Found %d LinkedIn %s for keyword '%s'", len(mentions), contentType, keyword)
	return mentions, nil
}

func (l *LinkedInSource) extractLinkedInPosts(n *html.Node, keyword, contentType string, since time.Duration) []models.Mention {
	var mentions []models.Mention
	
	// LinkedIn's structure changes frequently, so we'll look for common patterns
	if n.Type == html.ElementNode {
		// Look for potential post containers
		if (n.Data == "div" || n.Data == "article") && l.hasRelevantClass(n) {
			mention := l.extractPostFromNode(n, keyword, contentType)
			if mention != nil {
				// Check if the post is recent enough
				if mention.CreatedAt.After(time.Now().Add(-since)) {
					mentions = append(mentions, *mention)
				}
			}
		}
	}
	
	// Recursively search child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		childMentions := l.extractLinkedInPosts(c, keyword, contentType, since)
		mentions = append(mentions, childMentions...)
		
		// Limit the number of mentions to prevent huge payloads
		if len(mentions) >= 20 {
			break
		}
	}
	
	return mentions
}

func (l *LinkedInSource) hasRelevantClass(n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			// Look for classes that might indicate post content
			classVal := strings.ToLower(attr.Val)
			if strings.Contains(classVal, "feed-shared-update") ||
				strings.Contains(classVal, "update-components-text") ||
				strings.Contains(classVal, "feed-shared-text") ||
				strings.Contains(classVal, "share-update-card") ||
				strings.Contains(classVal, "article") ||
				strings.Contains(classVal, "post") ||
				strings.Contains(classVal, "content") {
				return true
			}
		}
	}
	return false
}

func (l *LinkedInSource) extractPostFromNode(n *html.Node, keyword, contentType string) *models.Mention {
	// Extract text content from the node
	text := l.extractTextContent(n)
	
	// Check if the text contains our keyword (case-insensitive)
	if !strings.Contains(strings.ToLower(text), strings.ToLower(keyword)) {
		return nil
	}
	
	// Look for a link
	postURL := l.extractLinkFromNode(n)
	if postURL == "" {
		// If no specific post link found, use the search URL
		postURL = fmt.Sprintf("https://www.linkedin.com/search/results/content/?keywords=%s", url.QueryEscape(keyword))
	}
	
	// Try to extract author information
	author := l.extractAuthorFromNode(n)
	if author == "" {
		author = "LinkedIn User"
	}
	
	// Create a mention
	mention := &models.Mention{
		ID:        fmt.Sprintf("linkedin_%s_%s_%d", contentType, strings.ReplaceAll(keyword, " ", "_"), time.Now().Unix()),
		Source:    "linkedin",
		Platform:  "LinkedIn",
		Title:     fmt.Sprintf("LinkedIn %s mentioning %s", contentType, keyword),
		Content:   l.truncateText(text, 300),
		Author:    author,
		URL:       postURL,
		CreatedAt: time.Now().UTC(),
		Keywords:  []string{keyword},
	}
	
	return mention
}

func (l *LinkedInSource) extractTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return strings.TrimSpace(n.Data)
	}
	
	var text strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text.WriteString(l.extractTextContent(c))
		text.WriteString(" ")
	}
	
	return strings.TrimSpace(text.String())
}

func (l *LinkedInSource) extractLinkFromNode(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key == "href" {
				href := attr.Val
				// Convert relative URLs to absolute
				if strings.HasPrefix(href, "/") {
					href = "https://www.linkedin.com" + href
				}
				return href
			}
		}
	}
	
	// Recursively search for links in child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if link := l.extractLinkFromNode(c); link != "" {
			return link
		}
	}
	
	return ""
}

func (l *LinkedInSource) extractAuthorFromNode(n *html.Node) string {
	// Look for author information in various LinkedIn patterns
	if n.Type == html.ElementNode {
		// Check for common author class patterns
		for _, attr := range n.Attr {
			if attr.Key == "class" {
				classVal := strings.ToLower(attr.Val)
				if strings.Contains(classVal, "author") ||
					strings.Contains(classVal, "name") ||
					strings.Contains(classVal, "actor") {
					text := l.extractTextContent(n)
					if text != "" && len(text) < 100 { // Reasonable author name length
						return text
					}
				}
			}
		}
	}
	
	// Recursively search for author information
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if author := l.extractAuthorFromNode(c); author != "" {
			return author
		}
	}
	
	return ""
}

func (l *LinkedInSource) truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	
	// Find the last space before the limit
	truncated := text[:maxLength]
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > 0 {
		truncated = truncated[:lastSpace]
	}
	
	return truncated + "..."
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
