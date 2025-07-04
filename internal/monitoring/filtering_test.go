package monitoring

import (
	"testing"

	"github.com/azure/aks-mentions-bot/internal/config"
	"github.com/azure/aks-mentions-bot/internal/models"
)

func TestImprovedAKSFiltering(t *testing.T) {
	// Create a service with default config for testing
	cfg := &config.Config{
		Keywords: []string{"aks", "azure kubernetes service"},
	}
	service := &Service{config: cfg}

	testCases := []struct {
		name     string
		mention  models.Mention
		expected bool
		reason   string
	}{
		{
			name: "Clear AKS - should accept",
			mention: models.Mention{
				Source:  "stackoverflow",
				Title:   "How to configure AKS cluster autoscaling",
				Content: "I'm trying to set up Azure Kubernetes Service cluster autoscaling with kubectl",
			},
			expected: true,
			reason:   "Has AKS + Azure context + Kubernetes context",
		},
		{
			name: "Gaming AKS - should reject",
			mention: models.Mention{
				Source:  "youtube",
				Title:   "Clean your AKS rifle maintenance",
				Content: "Best build of AKS-12 gaming shorts",
			},
			expected: false,
			reason:   "Gaming/rifle content should be filtered out",
		},
		{
			name: "Trading AKS - should reject", 
			mention: models.Mention{
				Source:  "youtube",
				Title:   "AKS trading software part 4",
				Content: "Forex trading bot with AKS algorithm",
			},
			expected: false,
			reason:   "Trading/forex content should be filtered out",
		},
		{
			name: "Makeup AKS - should reject",
			mention: models.Mention{
				Source:  "youtube", 
				Title:   "AKS makeup tutorial",
				Content: "Beauty and cosmetics AKS trending",
			},
			expected: false,
			reason:   "Beauty/makeup content should be filtered out",
		},
		{
			name: "Ambiguous AKS without context - should reject",
			mention: models.Mention{
				Source:  "youtube",
				Title:   "AKS part 2",
				Content: "This is about AKS but no other context",
			},
			expected: false,
			reason:   "AKS without Azure or Kubernetes context should be rejected",
		},
		{
			name: "AKS with Azure context but no K8s - should reject from YouTube",
			mention: models.Mention{
				Source:  "youtube",
				Title:   "AKS Microsoft Azure introduction", 
				Content: "Basic introduction to AKS on Microsoft Azure platform",
			},
			expected: false,
			reason:   "YouTube requires both Azure AND Kubernetes context for high confidence",
		},
		{
			name: "AKS with strong context - should accept from technical source",
			mention: models.Mention{
				Source:  "stackoverflow",
				Title:   "AKS pod deployment failing",
				Content: "My Azure AKS cluster pods are failing to deploy with kubectl",
			},
			expected: true,
			reason:   "Technical source with Azure and Kubernetes context",
		},
		{
			name: "Strong Azure Kubernetes Service indicator - should accept",
			mention: models.Mention{
				Source:  "medium",
				Title:   "Azure Kubernetes Service best practices",
				Content: "Guide for production AKS deployments",
			},
			expected: true,
			reason:   "Unambiguous Azure Kubernetes Service reference",
		},
		{
			name: "AWS EKS content - should reject",
			mention: models.Mention{
				Source:  "medium",
				Title:   "Migrating from AKS to EKS",
				Content: "Moving workloads from Azure to Amazon EKS kubernetes",
			},
			expected: false,
			reason:   "Contains AWS/EKS which is negative indicator",
		},
		{
			name: "Professional LinkedIn content - should accept",
			mention: models.Mention{
				Source:  "linkedin",
				Title:   "AKS cost optimization strategies",
				Content: "Sharing our experience optimizing Azure kubernetes costs with proper resource management",
			},
			expected: true,
			reason:   "Professional content with sufficient context",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := service.isRelevantMention(tc.mention)
			if result != tc.expected {
				t.Errorf("Test '%s' failed: expected %v, got %v. Reason: %s", 
					tc.name, tc.expected, result, tc.reason)
				t.Errorf("Source: %s, Title: %s, Content: %s", 
					tc.mention.Source, tc.mention.Title, tc.mention.Content)
			}
		})
	}
}

func TestSourceSpecificThresholds(t *testing.T) {
	cfg := &config.Config{
		Keywords: []string{"aks"},
	}
	service := &Service{config: cfg}

	// Test the same content across different sources with different thresholds
	baseMention := models.Mention{
		Title:   "AKS configuration",
		Content: "Setting up AKS with Azure and Kubernetes cluster management",
	}

	sources := []struct {
		source   string
		expected bool
		reason   string
	}{
		{"stackoverflow", true, "Technical platform with sufficient context"},
		{"reddit", true, "Technical platform with sufficient context"}, 
		{"medium", true, "Professional platform with strong context"},
		{"linkedin", true, "Professional platform with strong context"},
		{"youtube", true, "High-noise platform but has enough context (Azure + 2x K8s words)"},
		{"twitter", true, "Mixed platform with sufficient context"},
		{"unknown", true, "Unknown source with strong context"},
	}

	for _, sc := range sources {
		t.Run("Source_"+sc.source, func(t *testing.T) {
			mention := baseMention
			mention.Source = sc.source
			result := service.isRelevantMention(mention)
			if result != sc.expected {
				t.Errorf("Source '%s': expected %v, got %v. Reason: %s",
					sc.source, sc.expected, result, sc.reason)
			}
		})
	}
}

func TestUrgentFilteringWithContext(t *testing.T) {
	cfg := &config.Config{
		Keywords: []string{"aks", "azure kubernetes service"},
	}
	service := &Service{config: cfg}

	testCases := []struct {
		name           string
		mention        models.Mention
		expectedUrgent bool
		reason         string
	}{
		{
			name: "True urgent AKS security issue - should alert",
			mention: models.Mention{
				Source:  "hackernews",
				Title:   "Critical security vulnerability in AKS cluster management",
				Content: "Azure Kubernetes Service has a severe security breach affecting all clusters",
			},
			expectedUrgent: true,
			reason:         "Has both AKS context AND urgent security keywords",
		},
		{
			name: "False positive - software cracking with 'attack' - should NOT alert",
			mention: models.Mention{
				Source:  "hackernews", 
				Title:   "Ask HN: How do I prevent execs from obsessing over copy-protection?",
				Content: "The issue is that our present architecture means the attacker already has root and we cannot protect any key. While I want to encourage the org to eventually move to a client-server architecture we could protect, the need to provide the remote copies means all we can do is create puzzle boxes via security by obscurity",
			},
			expectedUrgent: false,
			reason:         "Contains 'attack' but no AKS/Azure context - should be filtered out",
		},
		{
			name: "Gaming AKS with security terms - should NOT alert",
			mention: models.Mention{
				Source:  "youtube",
				Title:   "AKS rifle security features and attack strategies",
				Content: "Best security attachments for AKS-74 against enemy attack",
			},
			expectedUrgent: false,
			reason:         "Gaming content should be filtered out despite urgent keywords",
		},
		{
			name: "Real AKS breaking change - should alert",
			mention: models.Mention{
				Source:  "medium",
				Title:   "Breaking change in Azure Kubernetes Service API",
				Content: "AKS users must upgrade immediately due to deprecated endpoints in Azure",
			},
			expectedUrgent: true,
			reason:         "Real AKS content with breaking change keywords",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test both context filtering and urgent filtering
			isRelevant := service.isRelevantMention(tc.mention)
			isUrgent := service.isUrgentMention(tc.mention)
			
			// Simulate the filterUrgentMentions logic
			wouldAlert := isRelevant && isUrgent
			
			if wouldAlert != tc.expectedUrgent {
				t.Errorf("Test '%s' failed: expected urgent alert %v, got %v. Reason: %s", 
					tc.name, tc.expectedUrgent, wouldAlert, tc.reason)
				t.Errorf("Details: isRelevant=%v, isUrgent=%v, Source: %s, Title: %s", 
					isRelevant, isUrgent, tc.mention.Source, tc.mention.Title)
			}
		})
	}
}
