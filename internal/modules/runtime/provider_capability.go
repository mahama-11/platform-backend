package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type ProviderUploadFileInput struct {
	Kind     string
	Filename string
	Reader   io.Reader
}

type ProviderUploadURLInput struct {
	FileURL string `json:"file_url" binding:"required"`
}

type ProviderActionInput struct {
	Payload map[string]any `json:"payload"`
}

func (s *Service) ProviderBalance(ctx context.Context, providerCode string) (map[string]any, error) {
	provider, err := s.providerCapabilities(providerCode)
	if err != nil {
		return nil, err
	}
	return provider.Balance(ctx)
}

func (s *Service) ProviderTTSVoices(ctx context.Context, providerCode string) (map[string]any, error) {
	provider, err := s.providerCapabilities(providerCode)
	if err != nil {
		return nil, err
	}
	return provider.TTSVoices(ctx)
}

func (s *Service) ProviderUploadFile(ctx context.Context, providerCode string, input ProviderUploadFileInput) (map[string]any, error) {
	provider, err := s.providerCapabilities(providerCode)
	if err != nil {
		return nil, err
	}
	return provider.UploadFile(ctx, input.Kind, input.Filename, input.Reader)
}

func (s *Service) ProviderUploadURL(ctx context.Context, providerCode, fileURL string) (map[string]any, error) {
	provider, err := s.providerCapabilities(providerCode)
	if err != nil {
		return nil, err
	}
	return provider.UploadURL(ctx, fileURL)
}

func (s *Service) ProviderAction(ctx context.Context, providerCode, action string, payload map[string]any) (map[string]any, error) {
	provider, err := s.providerCapabilities(providerCode)
	if err != nil {
		return nil, err
	}
	return provider.ProviderAction(ctx, action, payload)
}

func (s *Service) providerCapabilities(providerCode string) (VideoProviderCapabilities, error) {
	if s.registry == nil {
		return nil, errors.New("runtime provider registry is not configured")
	}
	provider, err := s.registry.Get(providerCode)
	if err != nil {
		return nil, err
	}
	capabilities, ok := provider.(VideoProviderCapabilities)
	if !ok {
		return nil, fmt.Errorf("provider %s does not expose video capabilities", providerCode)
	}
	return capabilities, nil
}
