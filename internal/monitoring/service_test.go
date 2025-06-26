package monitoring

import (
	"testing"

	"github.com/azure/aks-mentions-bot/internal/config"
	"github.com/azure/aks-mentions-bot/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStorage is a mock implementation of the storage interface
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Store(filename string, data []byte) error {
	args := m.Called(filename, data)
	return args.Error(0)
}

func (m *MockStorage) Retrieve(filename string) ([]byte, error) {
	args := m.Called(filename)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockStorage) List(prefix string) ([]string, error) {
	args := m.Called(prefix)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockStorage) Delete(filename string) error {
	args := m.Called(filename)
	return args.Error(0)
}

// MockNotificationService is a mock implementation of the notification service
type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) SendReport(report *models.Report) error {
	args := m.Called(report)
	return args.Error(0)
}

func (m *MockNotificationService) SendAlert(alert *models.Alert) error {
	args := m.Called(alert)
	return args.Error(0)
}

func TestService_isRelevantMention(t *testing.T) {
	cfg := &config.Config{}
	mockStorage := &MockStorage{}
	mockNotifications := &MockNotificationService{}
	
	service := NewService(cfg, mockStorage, mockNotifications)

	tests := []struct {
		name     string
		mention  models.Mention
		expected bool
	}{
		{
			name: "Valid AKS mention",
			mention: models.Mention{
				Title:   "How to deploy to Azure Kubernetes Service",
				Content: "I'm trying to deploy my application to AKS using kubectl",
			},
			expected: true,
		},
		{
			name: "Weapon-related mention",
			mention: models.Mention{
				Title:   "AK47 rifle specifications",
				Content: "Looking for information about AK assault rifles",
			},
			expected: false,
		},
		{
			name: "Valid KubeFleet mention",
			mention: models.Mention{
				Title:   "KubeFleet multi-cluster management",
				Content: "Using KubeFleet to manage multiple Kubernetes clusters",
			},
			expected: true,
		},
		{
			name: "Valid KAITO mention",
			mention: models.Mention{
				Title:   "KAITO AI inference on Kubernetes",
				Content: "Deploying AI models with KAITO on Azure Kubernetes",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isRelevantMention(tt.mention)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestService_basicSentimentAnalysis(t *testing.T) {
	cfg := &config.Config{}
	mockStorage := &MockStorage{}
	mockNotifications := &MockNotificationService{}
	
	service := NewService(cfg, mockStorage, mockNotifications)

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Positive content",
			content:  "This is a great solution that works perfectly",
			expected: "positive",
		},
		{
			name:     "Negative content",
			content:  "This is terrible and broken, hate it",
			expected: "negative",
		},
		{
			name:     "Neutral content",
			content:  "This is a documentation page about AKS",
			expected: "neutral",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.basicSentimentAnalysis(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestService_generateReport(t *testing.T) {
	cfg := &config.Config{
		ReportSchedule: "weekly",
	}
	mockStorage := &MockStorage{}
	mockNotifications := &MockNotificationService{}
	
	service := NewService(cfg, mockStorage, mockNotifications)

	mentions := []models.Mention{
		{
			ID:        "1",
			Source:    "reddit",
			Sentiment: "positive",
		},
		{
			ID:        "2",
			Source:    "stackoverflow",
			Sentiment: "negative",
		},
		{
			ID:        "3",
			Source:    "reddit",
			Sentiment: "neutral",
		},
	}

	report := service.generateReport(mentions)

	assert.Equal(t, "weekly", report.Period)
	assert.Equal(t, 3, report.TotalMentions)
	assert.Equal(t, mentions, report.Mentions)
	
	// Check summary
	sources := report.Summary["sources"].(map[string]int)
	assert.Equal(t, 2, sources["reddit"])
	assert.Equal(t, 1, sources["stackoverflow"])
	
	sentiment := report.Summary["sentiment"].(map[string]int)
	assert.Equal(t, 1, sentiment["positive"])
	assert.Equal(t, 1, sentiment["negative"])
	assert.Equal(t, 1, sentiment["neutral"])
}
