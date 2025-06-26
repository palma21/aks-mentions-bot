package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/azure/aks-mentions-bot/internal/config"
	"github.com/azure/aks-mentions-bot/internal/models"
	"github.com/azure/aks-mentions-bot/internal/monitoring"
	"github.com/azure/aks-mentions-bot/internal/sources"
	"github.com/joho/godotenv"
)

// SimpleTestStorage for local testing
type SimpleTestStorage struct{}

func (s *SimpleTestStorage) Store(filename string, data []byte) error {
	fmt.Printf("📁 Would store %d bytes to %s\n", len(data), filename)
	return nil
}
func (s *SimpleTestStorage) Retrieve(filename string) ([]byte, error) { return nil, nil }
func (s *SimpleTestStorage) List(prefix string) ([]string, error)     { return nil, nil }
func (s *SimpleTestStorage) Delete(filename string) error             { return nil }

// SimpleTestNotification for local testing
type SimpleTestNotification struct{}

func (s *SimpleTestNotification) SendReport(report *models.Report) error {
	fmt.Println("\n🎉 REPORT GENERATED!")
	fmt.Printf("📊 Total Mentions: %d\n", report.TotalMentions)
	
	if sources, ok := report.Summary["sources"].(map[string]int); ok {
		fmt.Println("📍 Sources:")
		for source, count := range sources {
			fmt.Printf("   • %s: %d mentions\n", source, count)
		}
	}
	
	if report.TotalMentions > 0 {
		fmt.Println("📝 Sample Mentions:")
		for i, mention := range report.Mentions {
			if i >= 3 { break }
			fmt.Printf("   %d. [%s] %s\n", i+1, mention.Platform, mention.Title)
		}
	}
	
	return nil
}

func (s *SimpleTestNotification) SendAlert(alert *models.Alert) error {
	fmt.Printf("🚨 ALERT: %s\n", alert.Message)
	return nil
}

func main() {
	fmt.Println("🧪 AKS Mentions Bot - Local Integration Test")
	fmt.Println("============================================")
	
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}
	
	// Create basic config for testing
	cfg := &config.Config{
		ReportSchedule: "daily",
		Keywords:       []string{"aks", "azure kubernetes service", "kubefleet", "kaito"},
		Port:          "8080",
	}
	
	// Create test services
	storage := &SimpleTestStorage{}
	notifications := &SimpleTestNotification{}
	
	// Create monitoring service
	service := monitoring.NewService(cfg, storage, notifications)
	
	fmt.Println("🔍 Running full monitoring cycle...")
	fmt.Println("⏱️  This will test real APIs and may take 30-60 seconds...")
	
	// Run monitoring (this will test real APIs)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	// Test each source individually to see results
	sources := []struct {
		name   string
		source sources.Source
	}{
		{"Stack Overflow", sources.NewStackOverflowSource()},
		{"Hacker News", sources.NewHackerNewsSource()},
		{"Medium", sources.NewMediumSource()},
		{"LinkedIn", sources.NewLinkedInSource()},
	}
	
	var allMentions []models.Mention
	
	for _, src := range sources {
		fmt.Printf("\n🔸 Testing %s...\n", src.name)
		
		if !src.source.IsEnabled() {
			fmt.Printf("   ⚠️  Skipped (disabled)\n")
			continue
		}
		
		mentions, err := src.source.FetchMentions(ctx, cfg.Keywords, 24*time.Hour)
		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
			continue
		}
		
		fmt.Printf("   ✅ Found %d mentions\n", len(mentions))
		
		if len(mentions) > 0 {
			fmt.Printf("   📝 Sample: \"%s\"\n", mentions[0].Title)
			allMentions = append(allMentions, mentions...)
		}
	}
	
	// Generate test report
	fmt.Printf("\n📊 Generating report with %d total mentions...\n", len(allMentions))
	
	if len(allMentions) > 0 {
		report := service.GenerateTestReport(allMentions)
		notifications.SendReport(report)
	} else {
		fmt.Println("ℹ️  No mentions found. This is normal for a quick test.")
		fmt.Println("💡 Try adding API keys for Reddit, Twitter, or YouTube for more results.")
	}
	
	fmt.Println("\n✅ Local integration test completed!")
	fmt.Println("\n🚀 Ready for deployment:")
	fmt.Println("   • Add more API keys to .env for additional sources")
	fmt.Println("   • Deploy to AKS: kubectl apply -f k8s/deployment.yaml")
	fmt.Println("   • Or deploy to ACA: make azd-up")
}
