package sources

import (
	"context"
	"time"

	"github.com/azure/aks-mentions-bot/internal/models"
)

// Source interface defines the contract for all data sources
type Source interface {
	GetName() string
	FetchMentions(ctx context.Context, keywords []string, since time.Duration) ([]models.Mention, error)
	IsEnabled() bool
}
