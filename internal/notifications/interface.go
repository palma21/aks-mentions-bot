package notifications

import "github.com/azure/aks-mentions-bot/internal/models"

// NotificationInterface defines the contract for notification services
type NotificationInterface interface {
	SendReport(report *models.Report) error
	SendAlert(alert *models.Alert) error
}
