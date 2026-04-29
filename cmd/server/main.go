package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"platform-service/internal/app"
)

// @title Platform Service Internal API
// @version 0.1.0
// @description Internal service APIs for metering, controls, wallet, incentives, and commercial routing.
// @description This spec is intended for product backends such as v-menu-backend.
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey InternalServiceSignature
// @in header
// @name X-Internal-Signature
// @securityDefinitions.apikey InternalServiceName
// @in header
// @name X-Internal-Service
// @securityDefinitions.apikey InternalServiceTimestamp
// @in header
// @name X-Internal-Timestamp
func main() {
	if err := runServer(os.Args[1:], os.Getenv, app.New, nil); err != nil {
		log.Fatalf("platform service exited with error: %v", err)
	}
}

func runServer(args []string, getenvFn func(string) string, newAppFn func(string) (*app.App, error), runFn func(*app.App, string) error) error {
	configFile, err := parseServerConfigFile(args, getenvFn)
	if err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	application, err := newAppFn(configFile)
	if err != nil {
		return fmt.Errorf("failed to initialize platform service: %w", err)
	}
	if application.Shutdown != nil {
		defer func() {
			_ = application.Shutdown(context.Background())
		}()
	}
	addr := fmt.Sprintf("%s:%d", application.Config.Host, application.Config.Port)

	// 支持测试注入自定义 runFn
	if runFn != nil {
		return runFn(application, addr)
	}

	// 生产模式：使用 http.Server + 优雅停服
	srv := &http.Server{
		Addr:    addr,
		Handler: application.Router,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("platform service listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server listen error: %w", err)
	case sig := <-quit:
		log.Printf("received signal %v, shutting down gracefully...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}
	log.Println("platform service stopped gracefully")
	return nil
}

func parseServerConfigFile(args []string, getenvFn func(string) string) (string, error) {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	configFile := fs.String("config", defaultEnv(getenvFn("PLATFORM_CONFIG_FILE"), "config.local"), "config file name without .yaml suffix")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	return *configFile, nil
}

func defaultEnv(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
