package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/azure/aks-mentions-bot/internal/config"
	"github.com/azure/aks-mentions-bot/internal/models"
	"github.com/azure/aks-mentions-bot/internal/monitoring"
)

// TestStorage implements simple file-based storage for testing
type TestStorage struct{}

func (t *TestStorage) Store(filename string, data []byte) error {
	dir := "test_output"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, filename), data, 0644)
}

func (t *TestStorage) Retrieve(filename string) ([]byte, error) {
	return os.ReadFile(filepath.Join("test_output", filename))
}

func (t *TestStorage) List(prefix string) ([]string, error) {
	return []string{}, nil
}

func (t *TestStorage) Delete(filename string) error {
	return os.Remove(filepath.Join("test_output", filename))
}

// TestNotificationService outputs reports to terminal and files
type TestNotificationService struct{}

func (t *TestNotificationService) SendReport(report *models.Report) error {
	// Print to terminal
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("📊 AKS MENTIONS REPORT")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("📅 Period: %s\n", report.Period)
	fmt.Printf("🕒 Generated: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("📈 Total Mentions: %d\n", report.TotalMentions)
	
	if sourceStats, ok := report.Summary["sources"].(map[string]int); ok {
		fmt.Println("\n📍 Sources:")
		for source, count := range sourceStats {
			fmt.Printf("   • %-15s %d mentions\n", source+":", count)
		}
	}
	
	if sentimentStats, ok := report.Summary["sentiment"].(map[string]int); ok {
		fmt.Println("\n💭 Sentiment Analysis:")
		for sentiment, count := range sentimentStats {
			emoji := "😐"
			switch sentiment {
			case "positive":
				emoji = "😊"
			case "negative":
				emoji = "😞"
			}
			fmt.Printf("   %s %-10s %d mentions\n", emoji, sentiment+":", count)
		}
	}
	
	fmt.Println("\n📝 Recent Mentions:")
	for i, mention := range report.Mentions {
		if i >= 5 { // Show first 5 mentions
			fmt.Printf("   ... and %d more mentions\n", len(report.Mentions)-5)
			break
		}
		fmt.Printf("\n   %d. [%s] %s\n", i+1, mention.Platform, mention.Title)
		if mention.Author != "" {
			fmt.Printf("      👤 Author: %s\n", mention.Author)
		}
		fmt.Printf("      🔗 URL: %s\n", mention.URL)
		fmt.Printf("      💭 Sentiment: %s | ⭐ Score: %d\n", mention.Sentiment, mention.Score)
		fmt.Printf("      🕒 Posted: %s\n", mention.CreatedAt.Format("2006-01-02 15:04"))
	}
	
	// Save to JSON file
	if err := t.saveReportToFile(report); err != nil {
		fmt.Printf("\n⚠️  Warning: Could not save to file: %v\n", err)
	}
	
	fmt.Println("\n" + strings.Repeat("=", 70))
	return nil
}

func (t *TestNotificationService) SendAlert(alert *models.Alert) error {
	fmt.Println("\n🚨 ALERT")
	fmt.Printf("Type: %s\n", alert.Type)
	fmt.Printf("Message: %s\n", alert.Message)
	return nil
}

func (t *TestNotificationService) saveReportToFile(report *models.Report) error {
	dir := "test_output"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	timestamp := report.GeneratedAt.Format("2006-01-02_15-04-05")
	filename := filepath.Join(dir, fmt.Sprintf("aks_mentions_report_%s.json", timestamp))
	
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}
	
	fmt.Printf("\n💾 Report saved to: %s\n", filename)
	return nil
}

func main() {
	fmt.Println("🤖 AKS Mentions Bot - Test Report Generator")
	fmt.Println("==========================================")
	
	// Create test configuration
	cfg := &config.Config{
		ReportSchedule: "weekly",
		Keywords:       []string{"aks", "azure kubernetes service", "kubefleet", "kaito"},
	}
	
	// Create test services
	storage := &TestStorage{}
	notifications := &TestNotificationService{}
	
	// Create monitoring service
	service := monitoring.NewService(cfg, storage, notifications)
	
	// Generate sample mentions data
	sampleMentions := []models.Mention{
		{
			ID:           "test_reddit_1",
			Source:       "reddit",
			Platform:     "Reddit",
			Title:        "Need help with AKS cluster networking",
			Content:      "I'm experiencing connectivity issues between pods in my AKS cluster. The application can't reach the database.",
			Author:       "devops_rookie",
			URL:          "https://reddit.com/r/AZURE/comments/example1",
			CreatedAt:    time.Now().Add(-3 * time.Hour),
			Score:        12,
			CommentCount: 5,
			Sentiment:    "negative",
			Keywords:     []string{"aks"},
		},
		{
			ID:           "test_stackoverflow_1",
			Source:       "stackoverflow",
			Platform:     "Stack Overflow",
			Title:        "Azure Kubernetes Service production readiness checklist",
			Content:      "What are the essential steps to make an AKS cluster production-ready? Looking for security, monitoring, and scaling best practices.",
			Author:       "enterprise_architect",
			URL:          "https://stackoverflow.com/questions/example1",
			CreatedAt:    time.Now().Add(-5 * time.Hour),
			Score:        89,
			CommentCount: 15,
			Sentiment:    "neutral",
			Keywords:     []string{"aks", "azure kubernetes service"},
		},
		{
			ID:           "test_twitter_1",
			Source:       "twitter",
			Platform:     "Twitter/X",
			Title:        "Just migrated from EKS to AKS - loving the Azure integration!",
			Content:      "The seamless integration between AKS and other Azure services is incredible. Monitoring with Application Insights is a game-changer! #AKS #Azure",
			Author:       "cloud_enthusiast",
			URL:          "https://twitter.com/cloud_enthusiast/status/example1",
			CreatedAt:    time.Now().Add(-8 * time.Hour),
			Score:        47,
			CommentCount: 12,
			Sentiment:    "positive",
			Keywords:     []string{"aks"},
		},
		{
			ID:           "test_hackernews_1",
			Source:       "hackernews",
			Platform:     "Hacker News",
			Title:        "KubeFleet: Microsoft's Multi-Cluster Kubernetes Management",
			Content:      "Interesting approach to managing multiple Kubernetes clusters. Has anyone compared KubeFleet with Rancher or other multi-cluster solutions?",
			Author:       "distributed_systems_expert",
			URL:          "https://news.ycombinator.com/item?id=example1",
			CreatedAt:    time.Now().Add(-12 * time.Hour),
			Score:        156,
			CommentCount: 43,
			Sentiment:    "neutral",
			Keywords:     []string{"kubefleet"},
		},
		{
			ID:           "test_youtube_1",
			Source:       "youtube",
			Platform:     "YouTube",
			Title:        "KAITO Tutorial: Deploying AI Models on Kubernetes Made Easy",
			Content:      "Excellent walkthrough of KAITO for AI model deployment. The automatic scaling and GPU management features are impressive. Great work by the Microsoft team!",
			Author:       "ml_engineer_pro",
			URL:          "https://youtube.com/watch?v=example1",
			CreatedAt:    time.Now().Add(-24 * time.Hour),
			Score:        234,
			CommentCount: 28,
			Sentiment:    "positive",
			Keywords:     []string{"kaito"},
		},
		{
			ID:           "test_medium_1",
			Source:       "medium",
			Platform:     "Medium",
			Title:        "Cost Optimization Strategies for Azure Kubernetes Service",
			Content:      "Comprehensive guide to reducing AKS costs through proper resource sizing, spot instances, and cluster optimization techniques.",
			Author:       "cloud_cost_optimizer",
			URL:          "https://medium.com/@cloudexpert/aks-cost-optimization-example1",
			CreatedAt:    time.Now().Add(-36 * time.Hour),
			Score:        67,
			CommentCount: 8,
			Sentiment:    "positive",
			Keywords:     []string{"aks", "azure kubernetes service"},
		},
	}
	
	fmt.Printf("\n📊 Generating report with %d sample mentions...\n", len(sampleMentions))
	
	// Generate report using the monitoring service
	report := service.GenerateTestReport(sampleMentions)
	
	// Send the report (outputs to terminal and saves to file)
	if err := notifications.SendReport(report); err != nil {
		fmt.Printf("❌ Error sending report: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("\n✅ Test report generation completed!")
	fmt.Println("\n💡 Next steps:")
	fmt.Println("   • Check the 'test_output' directory for saved JSON report")
	fmt.Println("   • Run 'go test ./internal/monitoring -v' for more detailed tests")
	fmt.Println("   • Configure real API keys and run the full bot with 'go run cmd/bot/main.go'")
}
