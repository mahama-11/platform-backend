package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"platform-service/internal/config"
	"platform-service/pkg/logger"

	"github.com/hibiken/asynq"
)

const (
	taskTypeDispatch = "runtime:dispatch"
	taskTypePoll     = "runtime:poll"
	taskTypeCallback = "runtime:callback"
)

type taskPayload struct {
	RuntimeJobID string `json:"runtime_job_id"`
	DeliveryID   string `json:"delivery_id,omitempty"`
}

type asynqClient interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
	Close() error
}

type AsynqRuntime struct {
	client    asynqClient
	server    *asynq.Server
	mux       *asynq.ServeMux
	queueName string
}

func NewAsynqRuntime(redisCfg config.RedisConfig, runtimeCfg config.RuntimeConfig, service *Service) (*AsynqRuntime, error) {
	if !redisCfg.Enabled {
		return nil, fmt.Errorf("runtime queue requires redis.enabled=true")
	}
	runtimeCfg = defaultRuntimeConfig(runtimeCfg)
	redisOpt := asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port),
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	}
	queueName := runtimeCfg.QueueName
	if queueName == "" {
		queueName = "runtime:default"
	}
	mux := asynq.NewServeMux()
	mux.HandleFunc(taskTypeDispatch, func(ctx context.Context, task *asynq.Task) error {
		return handleRuntimeAsynqTask(ctx, service, taskTypeDispatch, task)
	})
	mux.HandleFunc(taskTypePoll, func(ctx context.Context, task *asynq.Task) error {
		return handleRuntimeAsynqTask(ctx, service, taskTypePoll, task)
	})
	mux.HandleFunc(taskTypeCallback, func(ctx context.Context, task *asynq.Task) error {
		return handleRuntimeAsynqTask(ctx, service, taskTypeCallback, task)
	})
	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: runtimeCfg.WorkerConcurrency,
		Queues: map[string]int{
			queueName: 1,
		},
		RetryDelayFunc: func(n int, _ error, _ *asynq.Task) time.Duration {
			return runtimeCfg.RetryBackoff
		},
	})
	return &AsynqRuntime{
		client:    asynq.NewClient(redisOpt),
		server:    server,
		mux:       mux,
		queueName: queueName,
	}, nil
}

func (r *AsynqRuntime) EnqueueDispatch(runtimeJobID string, delay time.Duration) error {
	return r.enqueue(taskTypeDispatch, runtimeJobID, "", delay)
}

func (r *AsynqRuntime) EnqueuePoll(runtimeJobID string, delay time.Duration) error {
	return r.enqueue(taskTypePoll, runtimeJobID, "", delay)
}

func (r *AsynqRuntime) EnqueueCallback(deliveryID string, delay time.Duration) error {
	return r.enqueue(taskTypeCallback, "", deliveryID, delay)
}

func (r *AsynqRuntime) enqueue(taskType, runtimeJobID, deliveryID string, delay time.Duration) error {
	payload, _ := json.Marshal(taskPayload{RuntimeJobID: runtimeJobID, DeliveryID: deliveryID})
	task := asynq.NewTask(taskType, payload)
	options := []asynq.Option{
		asynq.Queue(r.queueName),
		asynq.MaxRetry(0),
	}
	if delay > 0 {
		options = append(options, asynq.ProcessIn(delay))
	}
	_, err := r.client.Enqueue(task, options...)
	return err
}

func (r *AsynqRuntime) Start() error {
	// 使用 channel 传播 server.Run() 的初始化错误
	errCh := make(chan error, 1)
	go func() {
		errCh <- r.server.Run(r.mux)
	}()
	// 短暂等待以捕获立即失败（如 Redis 连接失败）
	select {
	case err := <-errCh:
		return fmt.Errorf("asynq server failed to start: %w", err)
	case <-time.After(500 * time.Millisecond):
		// 启动成功，后台运行
		go func() {
			if err := <-errCh; err != nil {
				logger.Get().Error("asynq server stopped unexpectedly", "error", err)
			}
		}()
		return nil
	}
}

func (r *AsynqRuntime) Shutdown() {
	if r.server != nil {
		r.server.Shutdown()
	}
	if r.client != nil {
		_ = r.client.Close()
	}
}

func handleRuntimeAsynqTask(ctx context.Context, service *Service, taskType string, task *asynq.Task) error {
	payload, err := decodeTaskPayload(task)
	if err != nil {
		return err
	}
	switch taskType {
	case taskTypeDispatch:
		return service.HandleDispatchTask(ctx, payload.RuntimeJobID)
	case taskTypePoll:
		return service.HandlePollTask(ctx, payload.RuntimeJobID)
	case taskTypeCallback:
		return service.HandleCallbackTask(ctx, payload.DeliveryID)
	default:
		return fmt.Errorf("unsupported runtime task type: %s", taskType)
	}
}

func decodeTaskPayload(task *asynq.Task) (*taskPayload, error) {
	var payload taskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
