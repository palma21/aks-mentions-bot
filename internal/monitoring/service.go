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
	TotalMentions     int       `json:"total_mentions"`
	LastRun           time.Time `json:"last_run"`
	LastRunDuration   string    `json:"last_run_duration"`
	SourceMetrics     map[string]int `json:"source_metrics"`
	SentimentBreakdown map[string]int `json:"sentiment_breakdown"`
	ErrorCount        int       `json:"error_count"`
}

// NewService creates a new monitoring service
func NewService(cfg *config.Config, storage storage.StorageInterface, notificationService notifications.NotificationInterface) *Service {
	service := &Service{
		config:              cfg,
		storage:             storage,
		notificationService: notificationService,
		metrics: &Metrics{
			SourceMetrics:     make(map[string]int),
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

	// Fetch mentions from all sources concurrently
	for _, source := range s.sources {
		wg.Add(1)
		go func(src sources.Source) {
			defer wg.Done()
			
			logrus.Infof("Fetching mentions from %s", src.GetName())
			mentions, err := src.FetchMentions(ctx, s.config.Keywords, time.Since(s.getLastRunTime()))
			
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
	
	// Keywords that indicate it's about Azure Kubernetes Service
	positiveIndicators := []string{
		"azure", "microsoft", "kubernetes", "container", "cluster", "deployment",
		"helm", "kubectl", "namespace", "pod", "service", "ingress", "nodepool",
		"fleet manager", "kaito", "kubefleet",
	}
	
	// Keywords that indicate it's NOT about AKS (like weapons)
	negativeIndicators := []string{
		"rifle", "gun", "weapon", "firearm", "assault", "military", "bullet",
		"ammunition", "shoot", "trigger", "barrel", "stock", "caliber",
	}
	
	// Check for negative indicators first
	for _, indicator := range negativeIndicators {
		if strings.Contains(content, indicator) {
			return false
		}
	}
	
	// Check for positive indicators
	positiveScore := 0
	for _, indicator := range positiveIndicators {
		if strings.Contains(content, indicator) {
			positiveScore++
		}
	}
	
	// Require at least one positive indicator for AKS mentions
	return positiveScore > 0
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
