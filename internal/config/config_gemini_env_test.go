package config

import (
	"os"
	"testing"
)

func TestLoadBindsGeminiImageAPIKeyFromEnv(t *testing.T) {
	t.Setenv("PLATFORM_GEMINI_IMAGE_API_KEY", "dummy-image-key")
	t.Setenv("PLATFORM_GEMINI_VISUAL_API_KEY", "dummy-visual-key")
	cfg, err := Load("config.local")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GeminiImage.APIKey != "dummy-image-key" {
		t.Fatalf("expected gemini image api key from env, got %q (env present=%v)", cfg.GeminiImage.APIKey, os.Getenv("PLATFORM_GEMINI_IMAGE_API_KEY") != "")
	}
	if cfg.GeminiVisual.APIKey != "dummy-visual-key" {
		t.Fatalf("expected gemini visual api key from env, got %q", cfg.GeminiVisual.APIKey)
	}
}
