package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ad-delivery-simulator/api"
	"github.com/ad-delivery-simulator/config"
	"github.com/ad-delivery-simulator/internal/auction"
	"github.com/ad-delivery-simulator/internal/campaign"
	"github.com/ad-delivery-simulator/internal/tracking"
	kafkapkg "github.com/ad-delivery-simulator/pkg/kafka"
	redispkg "github.com/ad-delivery-simulator/pkg/redis"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg, err := config.Load(".")
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	logger := setupLogger(cfg.Logging)
	logger.Info("Starting Ad Delivery Simulator")

	db, err := setupDatabase(cfg.Database)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	if err := runMigrations(db); err != nil {
		logger.WithError(err).Fatal("Failed to run database migrations")
	}

	redisClient, err := redispkg.NewClient(
		cfg.Redis.Address(),
		cfg.Redis.Password,
		cfg.Redis.DB,
		logger,
	)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to Redis")
	}
	defer redisClient.Close()

	kafkaProducer := kafkapkg.NewProducer(cfg.Kafka.Brokers, logger)
	defer kafkaProducer.Close()

	kafkaConsumer := kafkapkg.NewConsumer(logger)
	defer kafkaConsumer.Close()

	campaignService := campaign.NewService(db, redisClient, kafkaProducer, cfg.Kafka.Brokers, logger)
	trackingService := tracking.NewService(db, redisClient, kafkaProducer, campaignService, cfg.Kafka.Brokers, logger)
	auctionEngine := auction.NewEngine(campaignService, redisClient, kafkaProducer, cfg.Kafka.Brokers, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trackingService.Start(ctx)
	defer trackingService.Stop()

	go startDailyBudgetResetScheduler(ctx, campaignService, logger)

	go startKafkaConsumers(ctx, kafkaConsumer, cfg.Kafka, logger)

	handlers := api.NewHandlers(auctionEngine, campaignService, trackingService, logger)
	
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	
	router := api.SetupRouter(handlers, logger)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.WithField("address", srv.Addr).Info("Starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start HTTP server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("Failed to gracefully shutdown server")
	}

	logger.Info("Server shutdown complete")
}

func setupLogger(cfg config.LoggingConfig) *logrus.Logger {
	logger := logrus.New()

	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	if cfg.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	if cfg.Output == "stdout" {
		logger.SetOutput(os.Stdout)
	} else {
		file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			logger.SetOutput(file)
		}
	}

	return logger
}

func setupDatabase(cfg config.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS campaigns (
			id UUID PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			advertiser_id VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL,
			budget_daily DECIMAL(10, 2) NOT NULL,
			budget_total DECIMAL(10, 2) NOT NULL,
			spent_daily DECIMAL(10, 2) DEFAULT 0,
			spent_total DECIMAL(10, 2) DEFAULT 0,
			bid_type VARCHAR(20) NOT NULL,
			bid_amount DECIMAL(10, 4) NOT NULL,
			targeting_rules JSONB,
			frequency_capping JSONB,
			start_date TIMESTAMP NOT NULL,
			end_date TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status)`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_advertiser ON campaigns(advertiser_id)`,
		`CREATE TABLE IF NOT EXISTS tracking_events (
			id UUID PRIMARY KEY,
			type VARCHAR(50) NOT NULL,
			campaign_id UUID NOT NULL,
			creative_id UUID,
			user_id VARCHAR(255),
			session_id VARCHAR(255),
			ip VARCHAR(45),
			user_agent TEXT,
			referrer TEXT,
			price DECIMAL(10, 4),
			timestamp TIMESTAMP DEFAULT NOW(),
			processed_at TIMESTAMP,
			metadata JSONB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tracking_campaign ON tracking_events(campaign_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tracking_type ON tracking_events(type)`,
		`CREATE INDEX IF NOT EXISTS idx_tracking_timestamp ON tracking_events(timestamp)`,
		`CREATE TABLE IF NOT EXISTS ad_creatives (
			id UUID PRIMARY KEY,
			campaign_id UUID NOT NULL REFERENCES campaigns(id),
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			format VARCHAR(50),
			width INT,
			height INT,
			asset_url TEXT,
			click_url TEXT,
			impression_url TEXT,
			html TEXT,
			status VARCHAR(50),
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_creatives_campaign ON ad_creatives(campaign_id)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("failed to run migration: %w", err)
		}
	}

	return nil
}

func startDailyBudgetResetScheduler(ctx context.Context, campaignService *campaign.Service, logger *logrus.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	nextReset := time.Now().Truncate(24*time.Hour).Add(24 * time.Hour)
	time.Sleep(time.Until(nextReset))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logger.Info("Resetting daily campaign budgets")
			if err := campaignService.ResetDailyBudgets(ctx); err != nil {
				logger.WithError(err).Error("Failed to reset daily budgets")
			}
		}
	}
}

func startKafkaConsumers(ctx context.Context, consumer *kafkapkg.Consumer, cfg config.KafkaConfig, logger *logrus.Logger) {
	logger.Info("Starting Kafka consumers")

	go consumer.ConsumeFromTopic(ctx, "bid-requests", cfg.Brokers, cfg.ConsumerGroup,
		func(ctx context.Context, message []byte) error {
			logger.Debug("Received bid request event")
			return nil
		})

	go consumer.ConsumeFromTopic(ctx, "impressions", cfg.Brokers, cfg.ConsumerGroup,
		func(ctx context.Context, message []byte) error {
			logger.Debug("Received impression event")
			return nil
		})

	go consumer.ConsumeFromTopic(ctx, "clicks", cfg.Brokers, cfg.ConsumerGroup,
		func(ctx context.Context, message []byte) error {
			logger.Debug("Received click event")
			return nil
		})
}