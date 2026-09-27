package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/joho/godotenv"

	"github.com/ingres/http-backend-go/internal/apierr"
	"github.com/ingres/http-backend-go/internal/cache"
	"github.com/ingres/http-backend-go/internal/config"
	"github.com/ingres/http-backend-go/internal/db"
	"github.com/ingres/http-backend-go/internal/models"
	"github.com/ingres/http-backend-go/internal/router"
)

const DefaultAgentURL = "http://localhost:9000"

func main() {
	// Initialize structured logger
	loggerOpts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	baseLogger := slog.New(slog.NewJSONHandler(os.Stdout, loggerOpts))
	slog.SetDefault(baseLogger)

	_ = godotenv.Load(".env")
	if err := godotenv.Load("../../.env"); err != nil {
		slog.Warn("Note: root .env not found, using local or system env")
	}

	cfg := config.LoadConfig()
	if cfg.AgentServiceURL == "http://localhost:9000" {
		cfg.AgentServiceURL = DefaultAgentURL
	}

	dbConn := db.Connect(cfg)
	if err := dbConn.AutoMigrate(&models.User{}, &models.Chat{}, &models.Message{}); err != nil {
		slog.Error("failed migration", "error", err)
		os.Exit(1)
	}

	app := fiber.New(fiber.Config{
		AppName: "Ingres HTTP Backend",
		// Centralized Error Handler
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			message := "Internal Server Error"

			// Use errors.As to check if it's our custom AppError
			var e *apierr.AppError
			if errors.As(err, &e) {
				code = e.Code
				message = e.Message
			} else {
				// Fallback for standard Fiber errors
				var fe *fiber.Error
				
				if errors.As(err, &fe) {
					code = fe.Code
					message = fe.Message
				}
			}

			slog.Error("request error", "code", code, "message", message, "details", err.Error())
			return c.Status(code).JSON(fiber.Map{
				"error": message,
			})
		},
	})
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowCredentials: true,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Requested-With",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Initialize Cache (L1: Local, L2: Redis)
	l1 := cache.NewLocalStore(1 * time.Minute)
	var cacheStore cache.Store = l1 // Default to L1 only if Redis fails

	if cfg.RedisURL != "" {
		l2, err := cache.NewRedisStore(cfg.RedisURL)
		if err != nil {
			slog.Error("failed to connect to redis, falling back to local only", "error", err)
		} else {
			slog.Info("connected to redis (L2 cache)")
			cacheStore = cache.NewHybridCache(l1, l2)
		}
	} else {
		slog.Warn("REDIS_URL not set, using local cache only")
	}

	router.SetupRoutes(app, dbConn, cfg, cacheStore)

	port := strconv.Itoa(cfg.HTTPBackendPort)

	// Prepare for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("http-backend starting", "port", port, "app", "IngresAI")
		if err := app.Listen(":" + port); err != nil {
			slog.Error("failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	<-quit // Block until a signal is received
	slog.Info("shutting down http-backend...")

	// Create a deadline for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
	}

	slog.Info("http-backend stopped")
}
