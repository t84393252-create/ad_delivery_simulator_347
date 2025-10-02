package api

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func SetupRouter(handlers *Handlers, logger *logrus.Logger) *gin.Engine {
	if gin.Mode() == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	
	router.Use(gin.Recovery())
	router.Use(LoggerMiddleware(logger))
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/health", handlers.HealthCheck)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := router.Group("/api/v1")
	{
		api.POST("/bid-request", RateLimitMiddleware(1000), handlers.HandleBidRequest)

		campaigns := api.Group("/campaigns")
		{
			campaigns.POST("", handlers.CreateCampaign)
			campaigns.GET("", handlers.ListCampaigns)
			campaigns.GET("/:id", handlers.GetCampaign)
			campaigns.PUT("/:id", handlers.UpdateCampaign)
			campaigns.GET("/:id/performance", handlers.GetCampaignPerformance)
			campaigns.GET("/:id/stats", handlers.GetEventStats)
			campaigns.GET("/:id/metrics", handlers.GetRealTimeMetrics)
		}

		tracking := api.Group("/track")
		{
			tracking.POST("/impression", RateLimitMiddleware(10000), handlers.TrackImpression)
			tracking.POST("/click", RateLimitMiddleware(5000), handlers.TrackClick)
			tracking.POST("/conversion", RateLimitMiddleware(1000), handlers.TrackConversion)
		}
	}

	return router
}

func LoggerMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		duration := time.Since(startTime)
		
		entry := logger.WithFields(logrus.Fields{
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"status":     c.Writer.Status(),
			"duration":   duration.Milliseconds(),
			"client_ip":  c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		})

		if c.Writer.Status() >= 500 {
			entry.Error("Server error")
		} else if c.Writer.Status() >= 400 {
			entry.Warn("Client error")
		} else {
			entry.Info("Request processed")
		}
	}
}

func RateLimitMiddleware(requestsPerSecond int) gin.HandlerFunc {
	ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
	
	return func(c *gin.Context) {
		select {
		case <-ticker.C:
			c.Next()
		default:
			c.JSON(429, gin.H{"error": "Rate limit exceeded"})
			c.Abort()
		}
	}
}