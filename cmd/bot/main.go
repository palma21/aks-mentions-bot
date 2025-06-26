package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/azure/aks-mentions-bot/internal/config"
	"github.com/azure/aks-mentions-bot/internal/monitoring"
	"github.com/azure/aks-mentions-bot/internal/notifications"
	"github.com/azure/aks-mentions-bot/internal/scheduler"
	"github.com/azure/aks-mentions-bot/internal/storage"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil {
		logrus.Info("No .env file found, using environment variables")
	}

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set up logging
	logrus.SetLevel(logrus.InfoLevel)
	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logrus.SetFormatter(&logrus.JSONFormatter{})

	logrus.Info("Starting AKS Mentions Bot")

	// Initialize Azure storage
	storageClient, err := storage.NewAzureStorage(cfg.StorageAccount, cfg.StorageContainer)
	if err != nil {
		logrus.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize notification services
	notificationService := notifications.NewService(cfg)

	// Initialize monitoring service
	monitoringService := monitoring.NewService(cfg, storageClient, notificationService)

	// Initialize scheduler
	schedulerService := scheduler.NewService(cfg, monitoringService)

	// Start scheduler
	if err := schedulerService.Start(); err != nil {
		logrus.Fatalf("Failed to start scheduler: %v", err)
	}
	defer schedulerService.Stop()

	// Set up HTTP server for health checks and webhooks
	router := mux.NewRouter()
	
	// Health check endpoint
	router.HandleFunc("/health", healthCheckHandler).Methods("GET")
	
	// Metrics endpoint
	router.HandleFunc("/metrics", metricsHandler(monitoringService)).Methods("GET")
	
	// Manual trigger endpoint (for testing)
	router.HandleFunc("/trigger", triggerHandler(monitoringService)).Methods("POST")

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in a goroutine
	go func() {
		logrus.Infof("HTTP server starting on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logrus.Errorf("Server forced to shutdown: %v", err)
	}

	logrus.Info("Server exited")
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

func metricsHandler(monitoringService *monitoring.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := monitoringService.GetMetrics()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(metrics))
	}
}

func triggerHandler(monitoringService *monitoring.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		go func() {
			if err := monitoringService.RunMonitoring(); err != nil {
				logrus.Errorf("Manual monitoring trigger failed: %v", err)
			}
		}()
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Monitoring triggered successfully"}`))
	}
}
