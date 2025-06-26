package monitoring

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/azure/aks-mentions-bot/internal/config"
	"github.com/azure/aks-mentions-bot/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockFileStorage implements StorageInterface for testing
type MockFileStorage struct {
	data map[string][]byte
}

func NewMockFileStorage() *MockFileStorage {
	return &MockFileStorage{
		data: make(map[string][]byte),
	}
}

func (m *MockFileStorage) Store(filename string, data []byte) error {
	m.data[filename] = data
	return nil
}

func (m *MockFileStorage) Retrieve(filename string) ([]byte, error) {
	if data, exists := m.data[filename]; exists {
		return data, nil
	}
	return nil, fmt.Errorf("file not found: %s", filename)
}

func (m *MockFileStorage) List(prefix string) ([]string, error) {
	var files []string
	for filename := range m.data {
		files = append(files, filename)
	}
	return files, nil
}

func (m *MockFileStorage) Delete(filename string) error {
	delete(m.data, filename)
	return nil
}

// MockFileNotificationService for file output testing
type MockFileNotificationService struct {
	reports []models.Report
	alerts  []models.Alert
}

func NewMockFileNotificationService() *MockFileNotificationService {
	return &MockFileNotificationService{
		reports: make([]models.Report, 0),
		alerts:  make([]models.Alert, 0),
	}
}

func (m *MockFileNotificationService) SendReport(report *models.Report) error {
	m.reports = append(m.reports, *report)
	
	// Output to terminal
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 AKS MENTIONS REPORT GENERATED")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Period: %s\n", report.Period)
	fmt.Printf("Generated: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("Total Mentions: %d\n", report.TotalMentions)
	
	if sourceStats, ok := report.Summary["sources"].(map[string]int); ok {
		fmt.Println("\n📈 Sources:")
		for source, count := range sourceStats {
			fmt.Printf("  • %s: %d mentions\n", source, count)
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
			fmt.Printf("  %s %s: %d mentions\n", emoji, sentiment, count)
		}
	}
	
	fmt.Println("\n📝 Sample Mentions:")
	for i, mention := range report.Mentions {
		if i >= 3 { // Show only first 3 mentions
			fmt.Printf("  ... and %d more mentions\n", len(report.Mentions)-3)
			break
		}
		fmt.Printf("  %d. [%s] %s\n", i+1, mention.Source, mention.Title)
		fmt.Printf("     URL: %s\n", mention.URL)
		fmt.Printf("     Sentiment: %s | Score: %d\n", mention.Sentiment, mention.Score)
		fmt.Println()
	}
	
	// Save to file
	if err := m.saveReportToFile(report); err != nil {
		fmt.Printf("Warning: Failed to save report to file: %v\n", err)
	}
	
	fmt.Println(strings.Repeat("=", 60))
	return nil
}

func (m *MockFileNotificationService) SendAlert(alert *models.Alert) error {
	m.alerts = append(m.alerts, *alert)
	
	fmt.Println("\n🚨 ALERT TRIGGERED")
	fmt.Printf("Type: %s\n", alert.Type)
	fmt.Printf("Title: %s\n", alert.Title)
	fmt.Printf("Message: %s\n", alert.Message)
	fmt.Printf("Timestamp: %s\n", alert.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
	
	return nil
}

func (m *MockFileNotificationService) saveReportToFile(report *models.Report) error {
	// Create reports directory if it doesn't exist
	reportsDir := "test_reports"
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return err
	}
	
	// Generate filename with timestamp
	timestamp := report.GeneratedAt.Format("2006-01-02_15-04-05")
	filename := filepath.Join(reportsDir, fmt.Sprintf("aks_mentions_report_%s.json", timestamp))
	
	// Convert report to JSON
	reportData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to file
	if err := os.WriteFile(filename, reportData, 0644); err != nil {
		return err
	}
	
	fmt.Printf("📁 Report saved to: %s\n", filename)
	return nil
}

// TestReportGeneration demonstrates full report generation with sample data
func TestReportGeneration(t *testing.T) {
	fmt.Println("\n🧪 Testing Report Generation with Sample Data...")
	
	// Create test configuration
	cfg := &config.Config{
		ReportSchedule: "weekly",
		Keywords:      []string{"aks", "azure kubernetes service", "kubefleet", "kaito"},
	}
	
	// Create mock services
	mockStorage := NewMockFileStorage()
	mockNotifications := NewMockFileNotificationService()
	
	// Create monitoring service
	service := NewService(cfg, mockStorage, mockNotifications)
	
	// Create sample mentions data
	sampleMentions := []models.Mention{
		{
			ID:          "reddit_1",
			Source:      "reddit",
			Platform:    "Reddit",
			Title:       "Help with AKS networking configuration",
			Content:     "I'm having trouble configuring networking in Azure Kubernetes Service. The pods can't communicate with external services.",
			Author:      "devops_engineer",
			URL:         "https://reddit.com/r/kubernetes/comments/sample1",
			CreatedAt:   time.Now().Add(-2 * time.Hour),
			Score:       15,
			Sentiment:   "negative",
			Keywords:    []string{"aks"},
		},
		{
			ID:          "stackoverflow_1",
			Source:      "stackoverflow",
			Platform:    "Stack Overflow",
			Title:       "Azure Kubernetes Service best practices for production",
			Content:     "What are the recommended best practices for running AKS in production? Looking for guidance on security, networking, and scaling.",
			Author:      "cloud_architect",
			URL:         "https://stackoverflow.com/questions/sample1",
			CreatedAt:   time.Now().Add(-4 * time.Hour),
			Score:       42,
			Sentiment:   "neutral",
			Keywords:    []string{"aks", "azure kubernetes service"},
		},
		{
			ID:          "twitter_1",
			Source:      "twitter",
			Platform:    "Twitter/X",
			Title:       "Just deployed my first app to AKS - amazing experience!",
			Content:     "Finally got my microservices running on Azure Kubernetes Service. The developer experience is fantastic! #AKS #Azure #Kubernetes",
			Author:      "startup_cto",
			URL:         "https://twitter.com/startup_cto/status/sample1",
			CreatedAt:   time.Now().Add(-6 * time.Hour),
			Score:       28,
			Sentiment:   "positive",
			Keywords:    []string{"aks"},
		},
		{
			ID:          "hackernews_1",
			Source:      "hackernews",
			Platform:    "Hacker News",
			Title:       "KubeFleet: Managing Multiple Kubernetes Clusters",
			Content:     "Interesting project from Microsoft for multi-cluster Kubernetes management. Has anyone tried KubeFleet in production?",
			Author:      "distributed_systems",
			URL:         "https://news.ycombinator.com/item?id=sample1",
			CreatedAt:   time.Now().Add(-8 * time.Hour),
			Score:       67,
			Sentiment:   "neutral",
			Keywords:    []string{"kubefleet"},
		},
		{
			ID:          "youtube_1",
			Source:      "youtube",
			Platform:    "YouTube",
			Title:       "KAITO: AI Model Deployment on Kubernetes",
			Content:     "Great tutorial on deploying AI models using KAITO on Azure Kubernetes Service. The automation capabilities are impressive!",
			Author:      "ai_engineer",
			URL:         "https://youtube.com/watch?v=sample1",
			CreatedAt:   time.Now().Add(-12 * time.Hour),
			Score:       156,
			Sentiment:   "positive",
			Keywords:    []string{"kaito", "aks"},
		},
	}
	
	// Generate and send report
	report := service.generateReport(sampleMentions)
	
	// Verify report structure
	assert.Equal(t, "weekly", report.Period)
	assert.Equal(t, 5, report.TotalMentions)
	assert.Equal(t, sampleMentions, report.Mentions)
	assert.NotNil(t, report.Summary)
	
	// Verify summary data
	sources := report.Summary["sources"].(map[string]int)
	assert.Equal(t, 1, sources["reddit"])
	assert.Equal(t, 1, sources["stackoverflow"])
	assert.Equal(t, 1, sources["twitter"])
	assert.Equal(t, 1, sources["hackernews"])
	assert.Equal(t, 1, sources["youtube"])
	
	sentiment := report.Summary["sentiment"].(map[string]int)
	assert.Equal(t, 2, sentiment["positive"])
	assert.Equal(t, 1, sentiment["negative"])
	assert.Equal(t, 2, sentiment["neutral"])
	
	// Send the report (this will output to terminal and save to file)
	err := mockNotifications.SendReport(report)
	require.NoError(t, err)
	
	// Verify the report was captured
	assert.Len(t, mockNotifications.reports, 1)
	capturedReport := mockNotifications.reports[0]
	assert.Equal(t, report.TotalMentions, capturedReport.TotalMentions)
	
	fmt.Println("✅ Report generation test completed successfully!")
}

// TestAlertGeneration demonstrates alert functionality
func TestAlertGeneration(t *testing.T) {
	fmt.Println("\n🚨 Testing Alert Generation...")
	
	mockNotifications := NewMockFileNotificationService()
	
	// Create sample alert
	alert := &models.Alert{
		ID:        "alert_1",
		Type:      "critical",
		Title:     "Multiple Negative Mentions Detected",
		Message:   "Multiple negative mentions detected for AKS networking issues",
		CreatedAt: time.Now(),
	}
	
	// Send alert
	err := mockNotifications.SendAlert(alert)
	require.NoError(t, err)
	
	// Verify alert was captured
	assert.Len(t, mockNotifications.alerts, 1)
	capturedAlert := mockNotifications.alerts[0]
	assert.Equal(t, "critical", capturedAlert.Type)
	assert.Equal(t, alert.Message, capturedAlert.Message)
	
	fmt.Println("✅ Alert generation test completed successfully!")
}

// TestFullWorkflow demonstrates the complete monitoring workflow
func TestFullWorkflow(t *testing.T) {
	fmt.Println("\n🔄 Testing Full Monitoring Workflow...")
	
	// This test demonstrates how the system would work end-to-end
	// Note: This doesn't make real API calls, but shows the complete flow
	
	cfg := &config.Config{
		ReportSchedule: "daily",
		Keywords:       []string{"aks", "azure kubernetes service", "kubefleet", "kaito"},
	}
	
	mockStorage := NewMockFileStorage()
	mockNotifications := NewMockFileNotificationService()
	service := NewService(cfg, mockStorage, mockNotifications)
	
	// Simulate the monitoring process with mock data
	fmt.Println("📊 Simulating monitoring workflow...")
	fmt.Println("  • Initializing data sources...")
	fmt.Println("  • Fetching mentions (simulated)...")
	fmt.Println("  • Applying context-aware filtering...")
	fmt.Println("  • Performing sentiment analysis...")
	fmt.Println("  • Storing data to Azure Blob Storage (simulated)...")
	fmt.Println("  • Generating report...")
	fmt.Println("  • Sending notifications...")
	
	// Create diverse sample data
	mentions := []models.Mention{
		{
			ID:        "integration_test_1",
			Source:    "reddit",
			Title:     "AKS vs EKS comparison",
			Content:   "Detailed comparison between Azure Kubernetes Service and Amazon EKS",
			Sentiment: "neutral",
			Score:     25,
			CreatedAt: time.Now(),
		},
		{
			ID:        "integration_test_2",
			Source:    "stackoverflow",
			Title:     "AKS RBAC configuration issues",
			Content:   "Having problems with RBAC setup in AKS cluster",
			Sentiment: "negative",
			Score:     8,
			CreatedAt: time.Now(),
		},
	}
	
	// Generate and process report
	report := service.generateReport(mentions)
	err := mockNotifications.SendReport(report)
	require.NoError(t, err)
	
	// Verify workflow completed
	assert.Len(t, mockNotifications.reports, 1)
	
	fmt.Println("✅ Full workflow test completed successfully!")
	fmt.Println("\n💡 To run this manually:")
	fmt.Println("   go test -v ./internal/monitoring -run TestReportGeneration")
	fmt.Println("   go test -v ./internal/monitoring -run TestFullWorkflow")
}
