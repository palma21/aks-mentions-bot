package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/azure/aks-mentions-bot/internal/config"
	"github.com/azure/aks-mentions-bot/internal/sources"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("🔍 AKS Mentions Bot - API Connectivity Test")
	fmt.Println("==========================================")
	
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}
	
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	// Test keywords
	keywords := []string{"aks", "azure kubernetes service"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	fmt.Println("\n📡 Testing API Sources...")
	fmt.Println(strings.Repeat("-", 40))
	
	// Test each source
	testSource("Reddit", sources.NewRedditSource(cfg.RedditClientID, cfg.RedditClientSecret), keywords, ctx)
	testSource("Stack Overflow", sources.NewStackOverflowSource(), keywords, ctx)
	testSource("Hacker News", sources.NewHackerNewsSource(), keywords, ctx)
	testSource("Twitter/X", sources.NewTwitterSource(cfg.TwitterBearerToken), keywords, ctx)
	testSource("YouTube", sources.NewYouTubeSource(cfg.YouTubeAPIKey), keywords, ctx)
	testSource("Medium", sources.NewMediumSource(), keywords, ctx)
	testSource("LinkedIn", sources.NewLinkedInSource(), keywords, ctx)
	
	fmt.Println("\n✅ API connectivity test completed!")
	fmt.Println("\n💡 Next steps:")
	fmt.Println("   • Configure missing API keys in .env file")
	fmt.Println("   • Run full bot with: make run")
	fmt.Println("   • Deploy to your preferred platform")
}

func testSource(name string, source sources.Source, keywords []string, ctx context.Context) {
	fmt.Printf("🔸 Testing %s... ", name)
	
	if !source.IsEnabled() {
		fmt.Printf("⚠️  DISABLED (missing API key)\n")
		return
	}
	
	mentions, err := source.FetchMentions(ctx, keywords, 24*time.Hour)
	if err != nil {
		fmt.Printf("❌ ERROR: %v\n", err)
		return
	}
	
	fmt.Printf("✅ SUCCESS (%d mentions found)\n", len(mentions))
	
	// Show sample mentions
	if len(mentions) > 0 {
		fmt.Printf("   📝 Sample: \"%s\"\n", mentions[0].Title)
	}
}
