package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	"platform-service/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeQueue struct {
	dispatches []queuedTask
	polls      []queuedTask
	callbacks  []queuedTask
}

type queuedTask struct {
	RuntimeJobID string
	Delay        time.Duration
}

func (q *fakeQueue) EnqueueDispatch(runtimeJobID string, delay time.Duration) error {
	q.dispatches = append(q.dispatches, queuedTask{RuntimeJobID: runtimeJobID, Delay: delay})
	return nil
}

func (q *fakeQueue) EnqueuePoll(runtimeJobID string, delay time.Duration) error {
	q.polls = append(q.polls, queuedTask{RuntimeJobID: runtimeJobID, Delay: delay})
	return nil
}

func (q *fakeQueue) EnqueueCallback(deliveryID string, delay time.Duration) error {
	q.callbacks = append(q.callbacks, queuedTask{RuntimeJobID: deliveryID, Delay: delay})
	return nil
}

type fakeProvider struct {
	name         string
	submitFn     func(ProviderJobRequest) (*ProviderSubmission, error)
	pollFn       func(string) (*ProviderPollResult, error)
	submitCtx    context.Context
	pollCtx      context.Context
	cancelCalled bool
}

func (p *fakeProvider) Name() string { return p.name }

func (p *fakeProvider) Submit(ctx context.Context, req ProviderJobRequest) (*ProviderSubmission, error) {
	p.submitCtx = ctx
	if p.submitFn != nil {
		return p.submitFn(req)
	}
	return &ProviderSubmission{ProviderJobID: "provider-job", Stage: "provider_accepted", StageMessage: "accepted"}, nil
}

func (p *fakeProvider) Poll(ctx context.Context, providerJobID string) (*ProviderPollResult, error) {
	p.pollCtx = ctx
	if p.pollFn != nil {
		return p.pollFn(providerJobID)
	}
	return &ProviderPollResult{Status: "processing", Stage: "provider_running", Progress: 10}, nil
}

func (p *fakeProvider) Cancel(_ context.Context, _ string) error {
	p.cancelCalled = true
	return nil
}

func newRuntimeFullTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.RuntimeProviderDefinition{},
		&models.RuntimeProductEndpoint{},
		&models.RuntimeProviderBinding{},
		&models.StorageBinding{},
		&models.StorageAsset{},
		&models.RuntimeJob{},
		&models.RuntimeAttempt{},
		&models.ChargeSession{},
		&models.RuntimeCallbackDelivery{},
		&models.Product{},
		&models.BillableItem{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func newRuntimeServiceForTest(t *testing.T) (*Service, *repository.RuntimeRepository, *fakeQueue) {
	t.Helper()
	db := newRuntimeFullTestDB(t)
	repo := repository.NewRuntimeRepository(db)
	queue := &fakeQueue{}
	service := NewService(repo, runtimeConfigForTest(), runtimeSecurityForTest(), runtimeComfyForTest())
	service.UseRuntime(queue, nil)
	return service, repo, queue
}

func runtimeConfigForTest() config.RuntimeConfig {
	return config.RuntimeConfig{
		ExecutionTimeout:   5 * time.Minute,
		RetryBackoff:       5 * time.Second,
		PollInitialBackoff: 1 * time.Second,
		PollBackoff:        2 * time.Second,
		PollTimeout:        5 * time.Minute,
		MaxAttempts:        3,
	}
}

func runtimeSecurityForTest() config.SecurityConfig {
	return config.SecurityConfig{EncryptionKey: "test-secret"}
}

func runtimeComfyForTest() config.ComfyUIBridgeConfig {
	return config.ComfyUIBridgeConfig{}
}
