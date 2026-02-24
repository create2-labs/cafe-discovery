package core

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/pkg/nats"

	"github.com/gofiber/fiber/v2"
	"github.com/spf13/viper"
)

// Setup initializes NATS and chain config. Call after initConfig/initLogging.
// Scanners do not use Postgres; persistence-service is the single writer.
func Setup(scannerType string) (*Deps, error) {
	natsConn, err := nats.New()
	if err != nil {
		return nil, err
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}
	chainConfig, err := config.LoadChainConfig(configPath)
	if err != nil {
		natsConn.Close()
		return nil, err
	}

	return &Deps{
		NATS:        natsConn,
		ChainConfig: chainConfig,
	}, nil
}

// RunResult holds the result of starting a runner (name, health checkers, shutdown func).
type RunResult struct {
	Name     string
	Checkers []HealthChecker
	Shutdown func()
}

// Run starts all runners, runs the health server and blocks until shutdown.
func Run(ctx context.Context, cancel context.CancelFunc, deps *Deps, runners []Runner) error {
	var results []RunResult
	for _, r := range runners {
		checkers, shutdown, err := r.Start(ctx, deps)
		if err != nil {
			return err
		}
		results = append(results, RunResult{Name: r.Name(), Checkers: checkers, Shutdown: shutdown})
	}

	log.Println("Scanners started successfully")

	healthPort := viper.GetString(config.ScannerHealthPort)
	if healthPort == "" {
		healthPort = "8081"
	}
	app := fiber.New(fiber.Config{AppName: "Cafe Discovery Scanner"})

	app.Get("/health", func(c *fiber.Ctx) error {
		natsConnected := deps.NATS.IsConnected()
		scanners := fiber.Map{}
		allOK := natsConnected
		for _, res := range results {
			running := true
			for _, h := range res.Checkers {
				if !h.IsRunning() {
					running = false
					allOK = false
					break
				}
			}
			scanners[res.Name] = fiber.Map{"running": running}
		}
		status := "ok"
		httpStatus := 200
		if !allOK {
			status = "degraded"
			httpStatus = 503
		}
		return c.Status(httpStatus).JSON(fiber.Map{
			"status":    status,
			"app_name":  "Cafe Discovery Scanner",
			"timestamp": time.Now().Format(time.RFC3339),
			"checks": fiber.Map{
				"nats":     fiber.Map{"connected": natsConnected},
				"scanners": scanners,
			},
		})
	})

	go func() {
		addr := "0.0.0.0:" + healthPort
		log.Printf("Starting health check server on %s", addr)
		if err := app.Listen(addr); err != nil {
			log.Printf("Failed to start health check server: %v", err)
		}
	}()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	<-sigint

	log.Println("Shutting down scanners...")
	for _, res := range results {
		if res.Shutdown != nil {
			res.Shutdown()
		}
	}
	cancel()

	if err := app.Shutdown(); err != nil {
		log.Printf("Error shutting down health check server: %v", err)
	}
	return nil
}
