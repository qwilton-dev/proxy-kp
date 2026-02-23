package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"proxy-kp/internal/config"
	"proxy-kp/internal/proxy"
	"proxy-kp/pkg/logger"

	"go.uber.org/zap"
)

const version = "1.0.0"

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Go Proxy Load Balancer v%s\n", version)
		os.Exit(0)
	}

	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Config file not found: %s\n", *configPath)
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Go Proxy Load Balancer",
		zap.String("version", version),
		zap.String("config", *configPath))

	server, err := proxy.NewServer(cfg, log)
	if err != nil {
		log.Fatal("Failed to create server", zap.Error(err))
		os.Exit(2)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)

	go func() {
		errCh <- server.Start(ctx)
	}()

	select {
	case sig := <-sigCh:
		log.Info("Received signal, shutting down",
			zap.String("signal", sig.String()))
		cancel()

		if err := server.Shutdown(); err != nil {
			log.Error("Shutdown error", zap.Error(err))
			os.Exit(2)
		}

		log.Info("Server stopped gracefully")
		os.Exit(0)

	case err := <-errCh:
		if err != nil {
			log.Error("Server error", zap.Error(err))
			os.Exit(2)
		}
	}
}
