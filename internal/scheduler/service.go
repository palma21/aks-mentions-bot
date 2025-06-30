package scheduler

import (
	"github.com/azure/aks-mentions-bot/internal/config"
	"github.com/azure/aks-mentions-bot/internal/monitoring"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// Service handles scheduling of monitoring tasks
type Service struct {
	config            *config.Config
	monitoringService *monitoring.Service
	cron              *cron.Cron
}

// NewService creates a new scheduler service
func NewService(cfg *config.Config, monitoringService *monitoring.Service) *Service {
	return &Service{
		config:            cfg,
		monitoringService: monitoringService,
		cron:              cron.New(cron.WithSeconds()),
	}
}

// Start begins the scheduled monitoring
func (s *Service) Start() error {
	var cronExpression string

	switch s.config.ReportSchedule {
	case "daily":
		// Run daily at 9 AM UTC
		cronExpression = "0 0 9 * * *"
	case "weekly":
		// Run weekly on Monday at 9 AM UTC
		cronExpression = "0 0 9 * * MON"
	default:
		// Default to weekly
		cronExpression = "0 0 9 * * MON"
	}

	_, err := s.cron.AddFunc(cronExpression, func() {
		logrus.Info("Starting scheduled monitoring run")
		if err := s.monitoringService.RunMonitoring(); err != nil {
			logrus.Errorf("Scheduled monitoring run failed: %v", err)
		}
	})

	if err != nil {
		return err
	}

	// Also add a more frequent check for critical issues (every 4 hours)
	_, err = s.cron.AddFunc("0 0 */4 * * *", func() {
		logrus.Info("Starting urgent mentions check (4-hour frequency)")
		if err := s.monitoringService.RunUrgentCheck(); err != nil {
			logrus.Errorf("Urgent mentions check failed: %v", err)
		}
	})

	if err != nil {
		return err
	}

	s.cron.Start()
	logrus.Infof("Scheduler started with %s schedule (plus urgent checks every 4 hours)", s.config.ReportSchedule)
	return nil
}

// Stop stops the scheduler
func (s *Service) Stop() {
	if s.cron != nil {
		s.cron.Stop()
		logrus.Info("Scheduler stopped")
	}
}
