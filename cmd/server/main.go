package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

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
	if err := runServer(os.Args[1:], os.Getenv, app.New, func(application *app.App, addr string) error {
		return application.Router.Run(addr)
	}); err != nil {
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
	if err := runFn(application, addr); err != nil {
		return err
	}
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
