package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/azure/aks-mentions-bot/internal/config"
	"github.com/azure/aks-mentions-bot/internal/models"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
)

// Service handles sending notifications via various channels
type Service struct {
	config *config.Config
	client *resty.Client
}

// Ensure Service implements NotificationInterface
var _ NotificationInterface = (*Service)(nil)

// TeamsMessage represents a Microsoft Teams webhook message (legacy format)
type TeamsMessage struct {
	Type     string         `json:"@type"`
	Context  string         `json:"@context"`
	Title    string         `json:"title"`
	Text     string         `json:"text"`
	Sections []TeamsSection `json:"sections,omitempty"`
}

type TeamsSection struct {
	ActivityTitle    string      `json:"activityTitle,omitempty"`
	ActivitySubtitle string      `json:"activitySubtitle,omitempty"`
	ActivityText     string      `json:"activityText,omitempty"`
	Facts            []TeamsFact `json:"facts,omitempty"`
	Markdown         bool        `json:"markdown,omitempty"`
}

type TeamsFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// LogicAppMessage represents a message for Azure Logic Apps
type LogicAppMessage struct {
	Title    string            `json:"title"`
	Summary  string            `json:"summary"`
	Mentions []LogicAppMention `json:"mentions"`
}

type LogicAppMention struct {
	Source    string `json:"source"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Snippet   string `json:"snippet"`
	Timestamp string `json:"timestamp"`
}

// NewService creates a new notification service
func NewService(cfg *config.Config) *Service {
	return &Service{
		config: cfg,
		client: resty.New().SetTimeout(30 * time.Second),
	}
}

// SendReport sends a report via configured notification channels
func (s *Service) SendReport(report *models.Report) error {
	var errors []string

	// Send to Teams if configured
	if s.config.TeamsWebhookURL != "" {
		if err := s.sendToTeams(report); err != nil {
			logrus.Errorf("Failed to send Teams notification: %v", err)
			errors = append(errors, fmt.Sprintf("Teams: %v", err))
		} else {
			logrus.Info("Successfully sent report to Teams")
		}
	}

	// Send via email if configured
	if s.config.NotificationEmail != "" {
		if err := s.sendEmail(report); err != nil {
			logrus.Errorf("Failed to send email notification: %v", err)
			errors = append(errors, fmt.Sprintf("Email: %v", err))
		} else {
			logrus.Info("Successfully sent report via email")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("notification errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

func (s *Service) sendToTeams(report *models.Report) error {
	// Detect if this is a Logic Apps endpoint or traditional Teams webhook
	if s.isLogicAppsEndpoint() {
		logrus.Info("Sending notification to Logic Apps endpoint")
		return s.sendToLogicApps(report)
	} else {
		logrus.Info("Sending notification to Teams webhook endpoint")
		message := s.buildTeamsMessage(report)
		return s.sendSingleMessage(message)
	}
}

func (s *Service) sendToLogicApps(report *models.Report) error {
	message := s.buildLogicAppMessage(report)
	
	// Check payload size
	payloadBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	payloadSize := len(payloadBytes)
	logrus.Infof("Payload size: %d bytes, mentions: %d", payloadSize, len(message.Mentions))
	
	// If payload is too large (> 500KB), send in batches
	if payloadSize > 500000 {
		logrus.Warn("Payload too large, sending in batches")
		return s.sendLogicAppsInBatches(report)
	}
	
	// Log a truncated version of the payload for debugging
	if payloadSize > 2000 {
		logrus.Infof("Large payload being sent (%d bytes) - truncated preview: %s...", payloadSize, string(payloadBytes[:2000]))
	} else {
		logrus.Infof("Payload being sent: %s", string(payloadBytes))
	}
	
	return s.sendSingleMessage(message)
}

func (s *Service) sendLogicAppsInBatches(report *models.Report) error {
	batchSize := 20 // Send 20 mentions per batch
	totalBatches := (len(report.Mentions) + batchSize - 1) / batchSize
	
	for i := 0; i < len(report.Mentions); i += batchSize {
		end := i + batchSize
		if end > len(report.Mentions) {
			end = len(report.Mentions)
		}
		
		// Create a batch report
		batchReport := &models.Report{
			GeneratedAt:    report.GeneratedAt,
			Period:         report.Period,
			TotalMentions:  len(report.Mentions), // Keep total count
			Summary:        report.Summary,
			Mentions:       report.Mentions[i:end],
		}
		
		batchNum := (i / batchSize) + 1
		message := s.buildLogicAppMessage(batchReport)
		
		// Update title to indicate batch
		message.Title = fmt.Sprintf("%s - Batch %d/%d", message.Title, batchNum, totalBatches)
		message.Summary = fmt.Sprintf("Batch %d of %d - %d mentions in this batch (Total: %d)", 
			batchNum, totalBatches, len(batchReport.Mentions), report.TotalMentions)
		
		err := s.sendSingleMessage(message)
		if err != nil {
			return fmt.Errorf("failed to send batch %d: %w", batchNum, err)
		}
		
		logrus.Infof("Successfully sent batch %d/%d to Teams", batchNum, totalBatches)
		
		// Small delay between batches to avoid overwhelming the Logic App
		if batchNum < totalBatches {
			time.Sleep(2 * time.Second)
		}
	}
	
	logrus.Infof("Successfully sent all %d batches to Teams", totalBatches)
	return nil
}

func (s *Service) sendSingleMessage(message interface{}) error {
	resp, err := s.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(message).
		Post(s.config.TeamsWebhookURL)

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 202 {
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	logrus.Info("Successfully sent report to Teams")
	return nil
}

// isLogicAppsEndpoint detects if the webhook URL is a Logic Apps endpoint
func (s *Service) isLogicAppsEndpoint() bool {
	url := s.config.TeamsWebhookURL
	logrus.Infof("Checking if URL is Logic Apps endpoint: %s", url)
	
	isLogicApps := strings.Contains(url, "logic.azure.com") ||
		strings.Contains(url, "workflows") ||
		strings.Contains(url, "azurewebsites.net")
	
	logrus.Infof("URL contains logic.azure.com: %v", strings.Contains(url, "logic.azure.com"))
	logrus.Infof("URL contains workflows: %v", strings.Contains(url, "workflows"))
	logrus.Infof("URL contains azurewebsites.net: %v", strings.Contains(url, "azurewebsites.net"))
	logrus.Infof("Is Logic Apps endpoint: %v", isLogicApps)
	
	return isLogicApps
}

// buildLogicAppMessage creates a message for Azure Logic Apps
func (s *Service) buildLogicAppMessage(report *models.Report) *LogicAppMessage {
	// Create a more descriptive title with date range
	var title string
	if report.Period == "weekly" {
		// For weekly reports, show the week ending date
		endDate := report.GeneratedAt.Format("Jan 2, 2006")
		startDate := report.GeneratedAt.AddDate(0, 0, -7).Format("Jan 2")
		title = fmt.Sprintf("AKS Mentions Report - Weekly (%s - %s)", startDate, endDate)
	} else if report.Period == "daily" {
		// For daily reports, show the specific date
		date := report.GeneratedAt.Format("Jan 2, 2006")
		title = fmt.Sprintf("AKS Mentions Report - Daily (%s)", date)
	} else {
		// Fallback for other periods
		date := report.GeneratedAt.Format("Jan 2, 2006")
		title = fmt.Sprintf("AKS Mentions Report - %s (%s)", strings.Title(report.Period), date)
	}

	message := &LogicAppMessage{
		Title:    title,
		Summary:  fmt.Sprintf("Found %d mentions in the last %s", report.TotalMentions, report.Period),
		Mentions: make([]LogicAppMention, 0, len(report.Mentions)),
	}

	// Convert mentions to Logic App format with content truncation
	for _, mention := range report.Mentions {
		logicAppMention := LogicAppMention{
			Source:    mention.Source,
			Title:     s.truncateString(mention.Title, 150),
			URL:       mention.URL,
			Snippet:   s.truncateString(mention.Content, 300), // Limit snippet to 300 chars
			Timestamp: mention.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		}
		message.Mentions = append(message.Mentions, logicAppMention)
	}

	return message
}

func (s *Service) buildTeamsMessage(report *models.Report) *TeamsMessage {
	// Create a more descriptive title with date range
	var title string
	if report.Period == "weekly" {
		// For weekly reports, show the week ending date
		endDate := report.GeneratedAt.Format("Jan 2, 2006")
		startDate := report.GeneratedAt.AddDate(0, 0, -7).Format("Jan 2")
		title = fmt.Sprintf("AKS Mentions Report - Weekly (%s - %s)", startDate, endDate)
	} else if report.Period == "daily" {
		// For daily reports, show the specific date
		date := report.GeneratedAt.Format("Jan 2, 2006")
		title = fmt.Sprintf("AKS Mentions Report - Daily (%s)", date)
	} else {
		// Fallback for other periods
		date := report.GeneratedAt.Format("Jan 2, 2006")
		title = fmt.Sprintf("AKS Mentions Report - %s (%s)", strings.Title(report.Period), date)
	}

	message := &TeamsMessage{
		Type:    "MessageCard",
		Context: "https://schema.org/extensions",
		Title:   title,
		Text:    fmt.Sprintf("Found %d mentions in the last %s", report.TotalMentions, report.Period),
	}

	// Add summary section
	if summary, ok := report.Summary["sentiment"].(map[string]int); ok {
		facts := []TeamsFact{
			{Name: "Total Mentions", Value: fmt.Sprintf("%d", report.TotalMentions)},
			{Name: "Generated", Value: report.GeneratedAt.Format("2006-01-02 15:04:05 UTC")},
		}

		for sentiment, count := range summary {
			facts = append(facts, TeamsFact{
				Name:  fmt.Sprintf("%s Mentions", strings.Title(sentiment)),
				Value: fmt.Sprintf("%d", count),
			})
		}

		message.Sections = append(message.Sections, TeamsSection{
			ActivityTitle: "Summary",
			Facts:         facts,
			Markdown:      true,
		})
	}

	// Add top mentions section
	if len(report.Mentions) > 0 {
		var topMentions []string
		limit := 5
		if len(report.Mentions) < limit {
			limit = len(report.Mentions)
		}

		for i := 0; i < limit; i++ {
			mention := report.Mentions[i]
			mentionText := fmt.Sprintf("**[%s](%s)** - %s (%s)",
				mention.Title, mention.URL, mention.Source, mention.CreatedAt.Format("Jan 2"))
			topMentions = append(topMentions, mentionText)
		}

		message.Sections = append(message.Sections, TeamsSection{
			ActivityTitle: "Recent Mentions",
			ActivityText:  strings.Join(topMentions, "\n\n"),
			Markdown:      true,
		})
	}

	return message
}

func (s *Service) sendEmail(report *models.Report) error {
	subject := fmt.Sprintf("AKS Mentions Report - %s (%d mentions)",
		strings.Title(report.Period), report.TotalMentions)

	htmlBody, err := s.buildEmailHTML(report)
	if err != nil {
		return fmt.Errorf("failed to build email HTML: %w", err)
	}

	textBody := s.buildEmailText(report)

	// Create message
	m := gomail.NewMessage()
	m.SetHeader("From", s.config.SMTPUsername)
	m.SetHeader("To", s.config.NotificationEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", textBody)
	m.AddAlternative("text/html", htmlBody)

	// Send email
	d := gomail.NewDialer(s.config.SMTPHost, s.config.SMTPPort, s.config.SMTPUsername, s.config.SMTPPassword)

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *Service) buildEmailHTML(report *models.Report) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>AKS Mentions Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background-color: #0078d4; color: white; padding: 20px; border-radius: 5px; }
        .summary { background-color: #f5f5f5; padding: 15px; margin: 20px 0; border-radius: 5px; }
        .mention { border-left: 4px solid #0078d4; padding: 10px; margin: 10px 0; background-color: #fafafa; }
        .mention-title { font-weight: bold; margin-bottom: 5px; }
        .mention-meta { color: #666; font-size: 0.9em; }
        .positive { border-left-color: #107c10; }
        .negative { border-left-color: #d13438; }
        .neutral { border-left-color: #605e5c; }
    </style>
</head>
<body>
    <div class="header">
        <h1>AKS Mentions Report</h1>
        <p>{{.Period}} report generated on {{.GeneratedAt.Format "January 2, 2006 at 3:04 PM UTC"}}</p>
    </div>

    <div class="summary">
        <h2>Summary</h2>
        <p><strong>Total Mentions:</strong> {{.TotalMentions}}</p>
        {{if .Summary.sentiment}}
            {{range $sentiment, $count := .Summary.sentiment}}
                <p><strong>{{$sentiment | title}} Mentions:</strong> {{$count}}</p>
            {{end}}
        {{end}}
    </div>

    {{if .Mentions}}
    <h2>Recent Mentions</h2>
    {{range $index, $mention := .Mentions}}
        {{if lt $index 10}}
        <div class="mention {{$mention.Sentiment}}">
            <div class="mention-title">
                <a href="{{$mention.URL}}" target="_blank">{{$mention.Title}}</a>
            </div>
            <div class="mention-meta">
                By {{$mention.Author}} on {{$mention.Source}} | {{$mention.CreatedAt.Format "Jan 2, 2006"}}
                {{if $mention.Score}} | Score: {{printf "%d" $mention.Score}}{{end}}
            </div>
            {{if $mention.Content}}
            <p>{{$mention.Content | truncate 200}}</p>
            {{end}}
        </div>
        {{end}}
    {{end}}
    {{end}}

    <hr>
    <p><small>This report was generated automatically by the AKS Mentions Bot.</small></p>
</body>
</html>
`

	// Create template with custom functions
	t := template.New("email").Funcs(template.FuncMap{
		"title": strings.Title,
		"printf": fmt.Sprintf,
		"truncate": func(s string, length int) string {
			if len(s) <= length {
				return s
			}
			return s[:length] + "..."
		},
	})

	t, err := t.Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, report); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (s *Service) buildEmailText(report *models.Report) string {
	var text strings.Builder

	text.WriteString(fmt.Sprintf("AKS Mentions Report - %s\n", strings.Title(report.Period)))
	text.WriteString(fmt.Sprintf("Generated: %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))

	text.WriteString("SUMMARY\n")
	text.WriteString("=======\n")
	text.WriteString(fmt.Sprintf("Total Mentions: %d\n", report.TotalMentions))

	if summary, ok := report.Summary["sentiment"].(map[string]int); ok {
		for sentiment, count := range summary {
			text.WriteString(fmt.Sprintf("%s Mentions: %d\n", strings.Title(sentiment), count))
		}
	}

	if len(report.Mentions) > 0 {
		text.WriteString("\nRECENT MENTIONS\n")
		text.WriteString("===============\n")

		limit := 10
		if len(report.Mentions) < limit {
			limit = len(report.Mentions)
		}

		for i := 0; i < limit; i++ {
			mention := report.Mentions[i]
			text.WriteString(fmt.Sprintf("\n%d. %s\n", i+1, mention.Title))
			text.WriteString(fmt.Sprintf("   Source: %s | Author: %s | Date: %s\n",
				mention.Source, mention.Author, mention.CreatedAt.Format("Jan 2, 2006")))
			text.WriteString(fmt.Sprintf("   URL: %s\n", mention.URL))
			if mention.Content != "" {
				content := mention.Content
				if len(content) > 200 {
					content = content[:200] + "..."
				}
				text.WriteString(fmt.Sprintf("   Content: %s\n", content))
			}
		}
	}

	text.WriteString("\n---\nThis report was generated automatically by the AKS Mentions Bot.\n")

	return text.String()
}

// SendAlert sends an urgent alert notification
func (s *Service) SendAlert(alert *models.Alert) error {
	// Implementation for urgent alerts
	// This could send immediate notifications for critical issues
	logrus.Infof("Alert would be sent: %s - %s", alert.Type, alert.Title)
	return nil
}

// truncateString truncates a string to maxLength and adds "..." if truncated
func (s *Service) truncateString(str string, maxLength int) string {
	if len(str) <= maxLength {
		return str
	}
	
	if maxLength <= 3 {
		return str[:maxLength]
	}
	
	return str[:maxLength-3] + "..."
}
