package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/azure/aks-mentions-bot/internal/config"
	"github.com/azure/aks-mentions-bot/internal/models"
	"github.com/azure/aks-mentions-bot/internal/notifications"
	"github.com/azure/aks-mentions-bot/internal/sources"
	"github.com/azure/aks-mentions-bot/internal/storage"
	"github.com/sirupsen/logrus"
)

// Service handles monitoring of mentions across various platforms
type Service struct {
	config              *config.Config
	storage             storage.StorageInterface
	notificationService notifications.NotificationInterface
	sources             []sources.Source
	metrics             *Metrics
	mu                  sync.RWMutex
}

// Metrics holds monitoring metrics
type Metrics struct {
	TotalMentions      int            `json:"total_mentions"`
	LastRun            time.Time      `json:"last_run"`
	LastRunDuration    string         `json:"last_run_duration"`
	SourceMetrics      map[string]int `json:"source_metrics"`
	SentimentBreakdown map[string]int `json:"sentiment_breakdown"`
	ErrorCount         int            `json:"error_count"`
}

// NewService creates a new monitoring service
func NewService(cfg *config.Config, storage storage.StorageInterface, notificationService notifications.NotificationInterface) *Service {
	service := &Service{
		config:              cfg,
		storage:             storage,
		notificationService: notificationService,
		metrics: &Metrics{
			SourceMetrics:      make(map[string]int),
			SentimentBreakdown: make(map[string]int),
		},
	}

	// Initialize data sources
	service.initializeSources()

	return service
}

func (s *Service) initializeSources() {
	s.sources = []sources.Source{
		sources.NewRedditSource(s.config.RedditClientID, s.config.RedditClientSecret),
		sources.NewStackOverflowSource(),
		sources.NewHackerNewsSource(),
		sources.NewTwitterSource(s.config.TwitterBearerToken),
		sources.NewYouTubeSource(s.config.YouTubeAPIKey),
		sources.NewMediumSource(),
		// LinkedIn source uses a hybrid approach:
		// 1. LinkedIn's direct APIs require restricted permissions and only allow
		//    accessing content you own or have explicit permissions for
		// 2. Public content monitoring requires Google search for LinkedIn Pulse articles,
		//    company page posts, and discussions - this provides real, relevant content
		// 3. Alternative: Could implement LinkedIn Company Pages API if we get
		//    organization-level access to specific companies (Microsoft, etc.)
		sources.NewLinkedInSource(),
	}
}

// RunMonitoring performs the main monitoring task
func (s *Service) RunMonitoring() error {
	start := time.Now()
	logrus.Info("Starting monitoring run")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var allMentions []models.Mention
	var wg sync.WaitGroup
	mentionsChan := make(chan []models.Mention, len(s.sources))
	errorsChan := make(chan error, len(s.sources))

	// Determine the time window to search
	// For consistency, always search the configured period regardless of last run time
	var searchWindow time.Duration
	switch s.config.ReportSchedule {
	case "daily":
		searchWindow = 24 * time.Hour
		logrus.Info("Searching for mentions in the last 24 hours (daily schedule)")
	case "weekly":
		searchWindow = 7 * 24 * time.Hour
		logrus.Info("Searching for mentions in the last 7 days (weekly schedule)")
	default:
		// Fallback - use time since last run, but minimum 24 hours
		timeSinceLastRun := time.Since(s.getLastRunTime())
		if timeSinceLastRun < 24*time.Hour {
			searchWindow = 24 * time.Hour
			logrus.Infof("Using minimum 24-hour search window (time since last run: %v)", timeSinceLastRun)
		} else {
			searchWindow = timeSinceLastRun
			logrus.Infof("Using time since last run as search window: %v", timeSinceLastRun)
		}
	}

	logrus.Infof("Searching %d sources for mentions in the last %v", len(s.sources), searchWindow)

	// Fetch mentions from all sources concurrently
	for _, source := range s.sources {
		wg.Add(1)
		go func(src sources.Source) {
			defer wg.Done()

			logrus.Infof("Fetching mentions from %s (window: %v)", src.GetName(), searchWindow)
			mentions, err := src.FetchMentions(ctx, s.config.Keywords, searchWindow)

			if err != nil {
				logrus.Errorf("Error fetching from %s: %v", src.GetName(), err)
				errorsChan <- err
				return
			}

			logrus.Infof("Found %d mentions from %s", len(mentions), src.GetName())
			mentionsChan <- mentions
		}(source)
	}

	// Close channels when all goroutines complete
	go func() {
		wg.Wait()
		close(mentionsChan)
		close(errorsChan)
	}()

	// Collect results
	for mentions := range mentionsChan {
		allMentions = append(allMentions, mentions...)
	}

	// Count errors
	errorCount := 0
	for range errorsChan {
		errorCount++
	}

	logrus.Infof("Collected %d total mentions from all sources", len(allMentions))

	// Filter mentions for context relevance
	if s.config.EnableContextFiltering {
		allMentions = s.filterByContext(allMentions)
		logrus.Infof("After context filtering: %d mentions", len(allMentions))
	}

	// Perform sentiment analysis
	if s.config.EnableSentimentAnalysis {
		s.analyzeSentiment(allMentions)
	}

	// Store mentions
	if err := s.storeMentions(allMentions); err != nil {
		logrus.Errorf("Failed to store mentions: %v", err)
		return err
	}

	// Update metrics
	s.updateMetrics(allMentions, time.Since(start), errorCount)

	// Generate and send report
	if err := s.generateAndSendReport(allMentions); err != nil {
		logrus.Errorf("Failed to send report: %v", err)
		return err
	}

	logrus.Infof("Monitoring run completed in %v", time.Since(start))
	return nil
}

func (s *Service) filterByContext(mentions []models.Mention) []models.Mention {
	var filtered []models.Mention

	for _, mention := range mentions {
		if s.isRelevantMention(mention) {
			filtered = append(filtered, mention)
		}
	}

	return filtered
}

func (s *Service) isRelevantMention(mention models.Mention) bool {
	content := strings.ToLower(mention.Content + " " + mention.Title)

	// Primary AKS/Azure indicators - must have at least one
	azureIndicators := []string{
		"aks", "azure kubernetes service", "azure kubernetes", "azure container service",
		"microsoft azure", "azurecr.io", "azure cli", "az aks", "azure devops",
		"azure container registry", "azure container apps", "kaito", "kubefleet",
		"azure kubernetes fleet", "azure kubernetes fleet manager",
	}

	// Secondary indicators that can boost relevance when combined with Azure
	kubernetesIndicators := []string{
		"kubernetes", "k8s", "container", "cluster", "deployment", "helm",
		"kubectl", "namespace", "pod", "service", "ingress", "nodepool",
	}

	// Keywords that indicate it's NOT about AKS
	negativeIndicators := []string{
		"rifle", "gun", "weapon", "firearm", "assault", "military", "bullet",
		"ammunition", "shoot", "trigger", "barrel", "stock", "caliber",
		"aws", "amazon", "eks", "gcp", "google", "gke", "openshift",
		"rancher", "docker desktop", "minikube", "kind", "k3s",
	}

	// Check for negative indicators first
	for _, indicator := range negativeIndicators {
		if strings.Contains(content, indicator) {
			return false
		}
	}

	// Check for primary Azure indicators
	azureScore := 0
	for _, indicator := range azureIndicators {
		if strings.Contains(content, indicator) {
			azureScore++
		}
	}

	// If we have Azure indicators, it's relevant
	if azureScore > 0 {
		return true
	}

	// If no Azure indicators, check if it's from our search terms and has Kubernetes context
	// This handles cases where the title might be "AKS" but content doesn't mention Azure
	if mention.Source == "reddit" || mention.Source == "stackoverflow" || mention.Source == "hackernews" {
		// For these sources, require both our search terms AND Kubernetes context
		hasSearchTerm := strings.Contains(content, "aks") || strings.Contains(content, "azure kubernetes")
		hasK8sContext := false
		for _, indicator := range kubernetesIndicators {
			if strings.Contains(content, indicator) {
				hasK8sContext = true
				break
			}
		}
		return hasSearchTerm && hasK8sContext
	}

	// For other sources (Medium, YouTube, etc.), be more restrictive
	return false
}

func (s *Service) analyzeSentiment(mentions []models.Mention) {
	// Basic sentiment analysis - in production, you'd use Azure Cognitive Services
	for i := range mentions {
		mentions[i].Sentiment = s.basicSentimentAnalysis(mentions[i].Content)
	}
}

func (s *Service) basicSentimentAnalysis(content string) string {
	content = strings.ToLower(content)

	positiveWords := []string{"good", "great", "excellent", "love", "awesome", "fantastic", "helpful", "works", "solved", "success"}
	negativeWords := []string{"bad", "terrible", "awful", "hate", "broken", "error", "fail", "problem", "issue", "bug"}

	positiveCount := 0
	negativeCount := 0

	for _, word := range positiveWords {
		if strings.Contains(content, word) {
			positiveCount++
		}
	}

	for _, word := range negativeWords {
		if strings.Contains(content, word) {
			negativeCount++
		}
	}

	if positiveCount > negativeCount {
		return "positive"
	} else if negativeCount > positiveCount {
		return "negative"
	}

	return "neutral"
}

func (s *Service) storeMentions(mentions []models.Mention) error {
	if len(mentions) == 0 {
		return nil
	}

	data, err := json.Marshal(mentions)
	if err != nil {
		return fmt.Errorf("failed to marshal mentions: %w", err)
	}

	filename := fmt.Sprintf("mentions-%s.json", time.Now().Format("2006-01-02-15-04-05"))
	return s.storage.Store(filename, data)
}

func (s *Service) generateAndSendReport(mentions []models.Mention) error {
	report := s.generateReport(mentions)
	return s.notificationService.SendReport(report)
}

func (s *Service) generateReport(mentions []models.Mention) *models.Report {
	report := &models.Report{
		GeneratedAt:   time.Now(),
		Period:        s.config.ReportSchedule,
		TotalMentions: len(mentions),
		Mentions:      mentions,
		Summary:       make(map[string]interface{}),
	}

	// Generate summary statistics
	sourceCount := make(map[string]int)
	sentimentCount := make(map[string]int)

	for _, mention := range mentions {
		sourceCount[mention.Source]++
		sentimentCount[mention.Sentiment]++
	}

	report.Summary["sources"] = sourceCount
	report.Summary["sentiment"] = sentimentCount
	report.Summary["top_sources"] = s.getTopSources(sourceCount)

	return report
}

func (s *Service) getTopSources(sourceCount map[string]int) []string {
	type sourceScore struct {
		source string
		count  int
	}

	var scores []sourceScore
	for source, count := range sourceCount {
		scores = append(scores, sourceScore{source, count})
	}

	// Simple sort by count (descending)
	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].count > scores[i].count {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	var topSources []string
	for i, score := range scores {
		if i >= 5 { // Top 5 sources
			break
		}
		topSources = append(topSources, fmt.Sprintf("%s (%d)", score.source, score.count))
	}

	return topSources
}

func (s *Service) updateMetrics(mentions []models.Mention, duration time.Duration, errorCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metrics.TotalMentions = len(mentions)
	s.metrics.LastRun = time.Now()
	s.metrics.LastRunDuration = duration.String()
	s.metrics.ErrorCount = errorCount

	// Reset counters
	s.metrics.SourceMetrics = make(map[string]int)
	s.metrics.SentimentBreakdown = make(map[string]int)

	// Count by source and sentiment
	for _, mention := range mentions {
		s.metrics.SourceMetrics[mention.Source]++
		s.metrics.SentimentBreakdown[mention.Sentiment]++
	}
}

func (s *Service) getLastRunTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.metrics.LastRun.IsZero() {
		// Default to 24 hours ago for first run
		return time.Now().Add(-24 * time.Hour)
	}

	return s.metrics.LastRun
}

// GetMetrics returns current metrics as JSON
func (s *Service) GetMetrics() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, _ := json.MarshalIndent(s.metrics, "", "  ")
	return string(data)
}

// GenerateTestReport creates a report from provided sample mentions for testing
func (s *Service) GenerateTestReport(mentions []models.Mention) *models.Report {
	// Apply sentiment analysis to mentions that don't have it
	for i := range mentions {
		if mentions[i].Sentiment == "" {
			mentions[i].Sentiment = s.basicSentimentAnalysis(mentions[i].Content)
		}
	}

	return s.generateReport(mentions)
}

// RunUrgentCheck performs a focused check for urgent mentions (security issues, breaking changes, etc.)
// This runs every 4 hours and only notifies about truly urgent content
func (s *Service) RunUrgentCheck() error {
	start := time.Now()
	logrus.Info("Starting urgent mentions check")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var allMentions []models.Mention
	var wg sync.WaitGroup
	mentionsChan := make(chan []models.Mention, len(s.sources))
	errorsChan := make(chan error, len(s.sources))

	// For urgent checks, only look at the last 4 hours
	searchWindow := 4 * time.Hour
	logrus.Info("Searching for urgent mentions in the last 4 hours")

	// Fetch mentions from all sources concurrently
	for _, source := range s.sources {
		wg.Add(1)
		go func(src sources.Source) {
			defer wg.Done()

			logrus.Infof("Checking %s for urgent mentions (4h window)", src.GetName())
			mentions, err := src.FetchMentions(ctx, s.config.Keywords, searchWindow)

			if err != nil {
				logrus.Errorf("Error fetching urgent mentions from %s: %v", src.GetName(), err)
				errorsChan <- err
				return
			}

			mentionsChan <- mentions
		}(source)
	}

	// Close channels when all goroutines complete
	go func() {
		wg.Wait()
		close(mentionsChan)
		close(errorsChan)
	}()

	// Collect results
	for mentions := range mentionsChan {
		allMentions = append(allMentions, mentions...)
	}

	logrus.Infof("Found %d total mentions for urgent check", len(allMentions))

	// Filter for urgent mentions only
	urgentMentions := s.filterUrgentMentions(allMentions)

	if len(urgentMentions) == 0 {
		logrus.Info("No urgent mentions found")
		return nil
	}

	logrus.Infof("Found %d urgent mentions requiring immediate notification", len(urgentMentions))

	// Store urgent mentions
	if err := s.storeMentions(urgentMentions); err != nil {
		logrus.Errorf("Failed to store urgent mentions: %v", err)
		return err
	}

	// Send urgent notification
	if err := s.sendUrgentNotification(urgentMentions); err != nil {
		logrus.Errorf("Failed to send urgent notification: %v", err)
		return err
	}

	logrus.Infof("Urgent check completed in %v, sent %d urgent alerts", time.Since(start), len(urgentMentions))
	return nil
}

// filterUrgentMentions identifies mentions that require immediate attention
func (s *Service) filterUrgentMentions(mentions []models.Mention) []models.Mention {
	var urgent []models.Mention

	for _, mention := range mentions {
		if s.isUrgentMention(mention) {
			urgent = append(urgent, mention)
		}
	}

	return urgent
}

// isUrgentMention determines if a mention requires immediate notification
func (s *Service) isUrgentMention(mention models.Mention) bool {
	content := strings.ToLower(mention.Content + " " + mention.Title)

	// Security-related urgent keywords
	securityKeywords := []string{
		"security vulnerability", "cve", "exploit", "breach", "attack",
		"malware", "ransomware", "phishing", "zero day", "critical security",
		"security patch", "hotfix", "urgent update", "immediate action required",
	}

	// Breaking changes and service issues
	breakingKeywords := []string{
		"breaking change", "deprecated", "end of life", "eol", "sunset",
		"service outage", "downtime", "incident", "degraded performance",
		"api changes", "breaking api", "mandatory upgrade",
	}

	// High-impact announcements
	highImpactKeywords := []string{
		"microsoft announcement", "azure announcement", "urgent notice",
		"action required", "immediate attention", "critical update",
		"service retirement", "feature retirement",
	}

	// Check for urgent keywords
	for _, keyword := range securityKeywords {
		if strings.Contains(content, keyword) {
			logrus.Infof("Urgent mention detected (security): %s in %s", keyword, mention.Title)
			return true
		}
	}

	for _, keyword := range breakingKeywords {
		if strings.Contains(content, keyword) {
			logrus.Infof("Urgent mention detected (breaking): %s in %s", keyword, mention.Title)
			return true
		}
	}

	for _, keyword := range highImpactKeywords {
		if strings.Contains(content, keyword) {
			logrus.Infof("Urgent mention detected (high-impact): %s in %s", keyword, mention.Title)
			return true
		}
	}

	return false
}

// sendUrgentNotification sends immediate notifications for urgent mentions
func (s *Service) sendUrgentNotification(mentions []models.Mention) error {
	if len(mentions) == 0 {
		return nil
	}

	// Create urgent report with correct structure
	report := &models.Report{
		GeneratedAt:   time.Now(),
		Period:        "4-hour urgent check",
		TotalMentions: len(mentions),
		Mentions:      mentions,
		Summary: map[string]interface{}{
			"title":       "🚨 URGENT AKS Mentions Alert",
			"description": fmt.Sprintf("Found %d urgent AKS-related mentions requiring immediate attention", len(mentions)),
			"type":        "urgent",
		},
	}

	// Send notification
	if err := s.notificationService.SendReport(report); err != nil {
		return fmt.Errorf("failed to send urgent notification: %w", err)
	}

	return nil
}
