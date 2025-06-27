package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the application
type Config struct {
	// Server configuration
	Port  string
	Debug bool

	// Schedule configuration
	ReportSchedule string // "daily" or "weekly"
	TimeZone       string

	// Azure Storage configuration
	StorageAccount   string
	StorageContainer string

	// Notification configuration
	TeamsWebhookURL   string
	NotificationEmail string
	SMTPHost          string
	SMTPPort          int
	SMTPUsername      string
	SMTPPassword      string

	// API Keys and credentials
	RedditClientID     string
	RedditClientSecret string
	TwitterBearerToken string
	YouTubeAPIKey      string

	// Keywords to monitor
	Keywords []string

	// Context filtering
	EnableContextFiltering bool
	ContextThreshold       float64

	// Sentiment analysis
	EnableSentimentAnalysis bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		Debug:          getBoolEnv("DEBUG", false),
		ReportSchedule: getEnv("REPORT_SCHEDULE", "weekly"),
		TimeZone:       getEnv("TIMEZONE", "UTC"),

		StorageAccount:   getEnv("AZURE_STORAGE_ACCOUNT", ""),
		StorageContainer: getEnv("AZURE_STORAGE_CONTAINER", "mentions"),

		TeamsWebhookURL:   getEnv("TEAMS_WEBHOOK_URL", ""),
		NotificationEmail: getEnv("NOTIFICATION_EMAIL", ""),
		SMTPHost:          getEnv("SMTP_HOST", ""),
		SMTPPort:          getIntEnv("SMTP_PORT", 587),
		SMTPUsername:      getEnv("SMTP_USERNAME", ""),
		SMTPPassword:      getEnv("SMTP_PASSWORD", ""),

		RedditClientID:     getEnv("REDDIT_CLIENT_ID", ""),
		RedditClientSecret: getEnv("REDDIT_CLIENT_SECRET", ""),
		TwitterBearerToken: getEnv("TWITTER_BEARER_TOKEN", ""),
		YouTubeAPIKey:      getEnv("YOUTUBE_API_KEY", ""),

		Keywords: getSliceEnv("KEYWORDS", []string{
			"Azure Kubernetes Service",
			"AKS",
			// Commented out for now to reduce noise and focus on core AKS mentions
			// "Azure Kubernetes Fleet Manager",
			// "KubeFleet", 
			// "KAITO",
			// "Azure Container Service",
		}),

		EnableContextFiltering:  getBoolEnv("ENABLE_CONTEXT_FILTERING", true),
		ContextThreshold:        getFloatEnv("CONTEXT_THRESHOLD", 0.7),
		EnableSentimentAnalysis: getBoolEnv("ENABLE_SENTIMENT_ANALYSIS", true),
	}

	// Validate required configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.ReportSchedule != "daily" && c.ReportSchedule != "weekly" {
		return fmt.Errorf("REPORT_SCHEDULE must be 'daily' or 'weekly'")
	}

	if c.TeamsWebhookURL == "" && c.NotificationEmail == "" {
		return fmt.Errorf("at least one notification method must be configured (TEAMS_WEBHOOK_URL or NOTIFICATION_EMAIL)")
	}

	if c.NotificationEmail != "" {
		if c.SMTPHost == "" || c.SMTPUsername == "" || c.SMTPPassword == "" {
			return fmt.Errorf("SMTP configuration is required when NOTIFICATION_EMAIL is set")
		}
	}

	return nil
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getFloatEnv(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getSliceEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
