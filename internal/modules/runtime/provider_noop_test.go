package runtime

import (
	"testing"

	"platform-service/internal/config"
)

func TestRuntimeProviderNoopAndAsynqConstructorGuards(t *testing.T) {
	manual := newManualProvider("manual-test")
	if manual.Name() != "manual-test" {
		t.Fatalf("unexpected manual provider name: %s", manual.Name())
	}
	submission, err := manual.Submit(nil, ProviderJobRequest{RuntimeJobID: "runtime-provider-noop"})
	if err != nil || submission == nil || submission.ProviderJobID == "" {
		t.Fatalf("manual Submit: %+v err=%v", submission, err)
	}
	poll, err := manual.Poll(nil, submission.ProviderJobID)
	if err != nil || poll == nil || poll.Status != "processing" {
		t.Fatalf("manual Poll: %+v err=%v", poll, err)
	}
	if err := manual.Cancel(nil, submission.ProviderJobID); err != nil {
		t.Fatalf("manual Cancel: %v", err)
	}
	if _, err := NewAsynqRuntime(config.RedisConfig{Enabled: false}, config.RuntimeConfig{}, nil); err == nil {
		t.Fatalf("expected disabled redis to reject asynq runtime construction")
	}
	withDefaults := defaultRuntimeConfig(config.RuntimeConfig{})
	if withDefaults.WorkerConcurrency <= 0 || withDefaults.QueueName == "" || withDefaults.MaxAttempts <= 0 {
		t.Fatalf("defaultRuntimeConfig did not populate defaults: %+v", withDefaults)
	}
	client := &fakeAsynqClient{}
	runtime := &AsynqRuntime{client: client}
	runtime.Shutdown()
}
