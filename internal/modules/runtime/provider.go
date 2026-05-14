package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"platform-service/internal/config"
)

type ProviderJobRequest struct {
	RuntimeJobID   string
	TaskType       string
	ProductCode    string
	OrganizationID string
	UserID         string
	Provider       string
	CallbackURL    string
	Input          RuntimeInputManifest
	Metadata       map[string]any
}

type ProviderSourceAsset struct {
	StorageKey string `json:"storage_key"`
	ID         string `json:"id"`
	SourceURL  string `json:"source_url"`
	PreviewURL string `json:"preview_url"`
	MimeType   string `json:"mime_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type RuntimePromptSnapshot struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	SystemPrompt   string `json:"system_prompt"`
	StylePrompt    string `json:"style_prompt"`
	UserPrompt     string `json:"user_prompt"`
	PromptTemplate string `json:"prompt_template"`
}

type RuntimeInputManifest struct {
	InputMode         string                `json:"input_mode"`
	PromptSnapshot    RuntimePromptSnapshot `json:"prompt_snapshot"`
	ParamsSnapshot    map[string]any        `json:"params_snapshot"`
	SourceAssetIDs    []string              `json:"source_asset_ids"`
	SourceAssets      []ProviderSourceAsset `json:"source_assets"`
	RequestedVariants int                   `json:"requested_variants"`
}

type ProviderResultVariant struct {
	Index      int            `json:"index"`
	SourceURL  string         `json:"source_url"`
	PreviewURL string         `json:"preview_url"`
	InlineData string         `json:"inline_data,omitempty"`
	MimeType   string         `json:"mime_type"`
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	Metadata   map[string]any `json:"metadata"`
}

type ProviderCompletion struct {
	Status       string                  `json:"status"`
	Progress     int                     `json:"progress"`
	StageMessage string                  `json:"stage_message"`
	Variants     []ProviderResultVariant `json:"variants"`
	Metadata     map[string]any          `json:"metadata"`
}

type NormalizedProviderCallbackPayload struct {
	ProviderCode  string                  `json:"provider_code"`
	ProviderJobID string                  `json:"provider_job_id,omitempty"`
	Status        string                  `json:"status"`
	Stage         string                  `json:"stage,omitempty"`
	StageMessage  string                  `json:"stage_message,omitempty"`
	Progress      int                     `json:"progress"`
	ErrorClass    string                  `json:"error_class,omitempty"`
	ErrorCode     string                  `json:"error_code,omitempty"`
	ErrorMessage  string                  `json:"error_message,omitempty"`
	Completion    *ProviderCompletion     `json:"completion,omitempty"`
	Variants      []ProviderResultVariant `json:"variants,omitempty"`
	Metadata      map[string]any          `json:"metadata,omitempty"`
}

type RuntimeOutputAssetManifest struct {
	AssetType      string         `json:"asset_type"`
	SourceType     string         `json:"source_type"`
	StorageKey     string         `json:"storage_key,omitempty"`
	StorageAssetID string         `json:"storage_asset_id,omitempty"`
	SourceURL      string         `json:"source_url,omitempty"`
	PreviewURL     string         `json:"preview_url,omitempty"`
	MimeType       string         `json:"mime_type,omitempty"`
	FileSize       int64          `json:"file_size,omitempty"`
	Width          int            `json:"width,omitempty"`
	Height         int            `json:"height,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type RuntimeOutputVariantManifest struct {
	Index      int                        `json:"index"`
	Status     string                     `json:"status"`
	IsSelected bool                       `json:"is_selected,omitempty"`
	Asset      RuntimeOutputAssetManifest `json:"asset"`
}

type RuntimeOutputManifest struct {
	Contract     string                         `json:"contract"`
	RuntimeJobID string                         `json:"runtime_job_id"`
	ProductCode  string                         `json:"product_code"`
	TaskType     string                         `json:"task_type"`
	ProviderCode string                         `json:"provider_code"`
	Status       string                         `json:"status"`
	Progress     int                            `json:"progress"`
	StageMessage string                         `json:"stage_message,omitempty"`
	Storage      map[string]any                 `json:"storage,omitempty"`
	Variants     []RuntimeOutputVariantManifest `json:"variants"`
	ProviderMeta map[string]any                 `json:"provider_metadata,omitempty"`
}

type ProviderSubmission struct {
	ProviderJobID string              `json:"provider_job_id"`
	Stage         string              `json:"stage"`
	StageMessage  string              `json:"stage_message"`
	EtaSeconds    int                 `json:"eta_seconds"`
	Completion    *ProviderCompletion `json:"completion,omitempty"`
}

type ProviderPollResult struct {
	Status       string              `json:"status"`
	Stage        string              `json:"stage"`
	StageMessage string              `json:"stage_message"`
	Progress     int                 `json:"progress"`
	EtaSeconds   int                 `json:"eta_seconds"`
	ErrorClass   string              `json:"error_class,omitempty"`
	ErrorCode    string              `json:"error_code,omitempty"`
	ErrorMessage string              `json:"error_message,omitempty"`
	Completion   *ProviderCompletion `json:"completion,omitempty"`
}

type providerError struct {
	message   string
	retryable bool
}

func (e *providerError) Error() string   { return e.message }
func (e *providerError) Retryable() bool { return e.retryable }

func newRetryableProviderError(message string) error {
	return &providerError{message: message, retryable: true}
}

func newNonRetryableProviderError(message string) error {
	return &providerError{message: message, retryable: false}
}

func isRetryableProviderError(err error) bool {
	if err == nil {
		return false
	}
	type retryable interface{ Retryable() bool }
	if value, ok := err.(retryable); ok {
		return value.Retryable()
	}
	return true
}

type GenerationProvider interface {
	Name() string
	Submit(ctx context.Context, req ProviderJobRequest) (*ProviderSubmission, error)
	Poll(ctx context.Context, providerJobID string) (*ProviderPollResult, error)
	Cancel(ctx context.Context, providerJobID string) error
}

type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]GenerationProvider
}

func NewProviderRegistry(volcengineCfg config.VolcengineConfig, comfyCfg config.ComfyUIBridgeConfig) *ProviderRegistry {
	registry := &ProviderRegistry{providers: map[string]GenerationProvider{}}
	registry.Register(newManualProvider("manual"))
	registry.Register(newManualProvider("mock"))
	registry.Register(newVolcengineImageProvider("volcengine", volcengineCfg))
	if comfyCfg.Enabled {
		registry.Register(newComfyUIBridgeProvider("comfyui_bridge", comfyCfg))
	}
	return registry
}

func (r *ProviderRegistry) Register(provider GenerationProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

func (r *ProviderRegistry) Get(name string) (GenerationProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %s not registered", name)
	}
	return provider, nil
}

type manualProvider struct {
	name string
}

func newManualProvider(name string) GenerationProvider {
	return &manualProvider{name: name}
}

func (p *manualProvider) Name() string { return p.name }

func (p *manualProvider) Submit(_ context.Context, req ProviderJobRequest) (*ProviderSubmission, error) {
	return &ProviderSubmission{
		ProviderJobID: fmt.Sprintf("%s-%s-%d", p.name, req.RuntimeJobID, time.Now().UnixNano()),
		Stage:         "provider_accepted",
		StageMessage:  "Accepted by provider and waiting for processing results",
		EtaSeconds:    30,
	}, nil
}

func (p *manualProvider) Poll(_ context.Context, _ string) (*ProviderPollResult, error) {
	return &ProviderPollResult{
		Status:       "processing",
		Stage:        "provider_pending",
		StageMessage: "Waiting for manual provider completion",
		Progress:     0,
	}, nil
}

func (p *manualProvider) Cancel(_ context.Context, _ string) error { return nil }
