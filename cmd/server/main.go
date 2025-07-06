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

	"https://github.com/soosho/bixor-engine/internal/matching"
	"https://github.com/soosho/bixor-engine/pkg/api"
	"https://github.com/soosho/bixor-engine/pkg/cache"
	"https://github.com/soosho/bixor-engine/pkg/config"
	"https://github.com/soosho/bixor-engine/pkg/database"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logging
	setupLogging(cfg)

	logrus.Info("Starting Bixor Engine...")

	// Initialize database
	if err := database.Initialize(cfg); err != nil {
		logrus.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Run database migrations
	if err := database.AutoMigrate(); err != nil {
		logrus.Fatalf("Failed to run database migrations: %v", err)
	}

	// Seed initial data
	if cfg.IsDevelopment() {
		if err := database.SeedData(); err != nil {
			logrus.Fatalf("Failed to seed database: %v", err)
		}
	}

	// Initialize Redis cache
	if err := cache.Initialize(cfg); err != nil {
		logrus.Fatalf("Failed to initialize Redis: %v", err)
	}
	defer cache.Close()

	// Initialize matching engine
	publishTrader := matching.NewMemoryPublishTrader()
	engine := matching.NewMatchingEngine(publishTrader)

	// Setup HTTP server
	if !cfg.IsDevelopment() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// CORS configuration
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	router.Use(cors.New(corsConfig))

	// Initialize API routes
	api.SetupRoutes(router, engine, cfg)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		logrus.Infof("Bixor Engine server starting on port %s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down Bixor Engine...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.Errorf("Server forced to shutdown: %v", err)
	}

	logrus.Info("Bixor Engine stopped successfully")
}

func setupLogging(cfg *config.Config) {
	// Set log format
	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	// Set log level
	if cfg.IsDevelopment() {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	logrus.Info("Logging initialized")
} 