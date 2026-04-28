package main

import (
	"context"
	"errors"
	"testing"

	"platform-service/internal/app"
	"platform-service/internal/config"

	"github.com/gin-gonic/gin"
)

func TestParseServerConfigFile(t *testing.T) {
	env := func(string) string { return "" }
	configFile, err := parseServerConfigFile(nil, env)
	if err != nil || configFile != "config.local" {
		t.Fatalf("expected default config.local, got config=%s err=%v", configFile, err)
	}
	env = func(string) string { return "config.from.env" }
	configFile, err = parseServerConfigFile(nil, env)
	if err != nil || configFile != "config.from.env" {
		t.Fatalf("expected env config, got config=%s err=%v", configFile, err)
	}
	configFile, err = parseServerConfigFile([]string{"-config", "config.from.flag"}, env)
	if err != nil || configFile != "config.from.flag" {
		t.Fatalf("expected flag to override env, got config=%s err=%v", configFile, err)
	}
}

func TestRunServer(t *testing.T) {
	t.Run("parse error", func(t *testing.T) {
		err := runServer([]string{"-badflag"}, func(string) string { return "" }, nil, nil)
		if err == nil {
			t.Fatalf("expected parse error")
		}
	})

	t.Run("new app error", func(t *testing.T) {
		err := runServer(nil, func(string) string { return "" }, func(string) (*app.App, error) {
			return nil, errors.New("boom")
		}, nil)
		if err == nil {
			t.Fatalf("expected new app error")
		}
	})

	t.Run("run error and shutdown", func(t *testing.T) {
		shutdownCalled := false
		runCalled := false
		err := runServer(nil, func(string) string { return "" }, func(string) (*app.App, error) {
			return &app.App{
				Config: config.Config{Host: "127.0.0.1", Port: 18080},
				Router: gin.New(),
				Shutdown: func(context.Context) error {
					shutdownCalled = true
					return nil
				},
			}, nil
		}, func(_ *app.App, addr string) error {
			runCalled = true
			if addr != "127.0.0.1:18080" {
				t.Fatalf("unexpected addr: %s", addr)
			}
			return errors.New("run failed")
		})
		if err == nil || !runCalled || !shutdownCalled {
			t.Fatalf("expected run error with shutdown, err=%v runCalled=%v shutdownCalled=%v", err, runCalled, shutdownCalled)
		}
	})

	t.Run("success", func(t *testing.T) {
		err := runServer(nil, func(string) string { return "" }, func(string) (*app.App, error) {
			return &app.App{
				Config: config.Config{Host: "0.0.0.0", Port: 18081},
				Router: gin.New(),
			}, nil
		}, func(_ *app.App, addr string) error {
			if addr != "0.0.0.0:18081" {
				t.Fatalf("unexpected addr: %s", addr)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})
}
