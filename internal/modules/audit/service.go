package audit

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"github.com/gin-gonic/gin"
)

type Service struct {
	repo *repository.AuditRepository
}

const (
	defaultQueryLimit = 50
	maxQueryLimit     = 200
)

type RecordInput struct {
	Action             string
	TargetType         string
	TargetID           string
	BillingSubjectType string
	BillingSubjectID   string
	Status             string
	Details            string
	BeforeSnapshot     any
	AfterSnapshot      any
}

type QueryInput struct {
	Query       string `form:"query"`
	Action      string `form:"action"`
	TargetType  string `form:"target_type"`
	Status      string `form:"status"`
	ActorUserID string `form:"actor_user_id"`
	ActorOrgID  string `form:"actor_org_id"`
	RequestID   string `form:"request_id"`
	TraceID     string `form:"trace_id"`
	Limit       int    `form:"limit"`
	Offset      int    `form:"offset"`
}

type QueryResult struct {
	Items  []models.AuditLog        `json:"items"`
	Total  int64                    `json:"total"`
	Limit  int                      `json:"limit"`
	Offset int                      `json:"offset"`
	Stats  repository.AuditLogStats `json:"stats"`
}

var ErrInvalidPagination = errors.New("invalid audit log pagination")

func NewService(repo *repository.AuditRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RecordFromGin(c *gin.Context, input RecordInput) error {
	beforeSnapshot, _ := encodeSnapshot(input.BeforeSnapshot)
	afterSnapshot, _ := encodeSnapshot(input.AfterSnapshot)
	item := &models.AuditLog{
		ID:                 utils.GenerateID(),
		RequestID:          c.GetString(platformconst.CtxRequestID),
		TraceID:            c.GetString(platformconst.CtxTraceID),
		ActorUserID:        c.GetString(platformconst.CtxUserID),
		ActorOrgID:         c.GetString(platformconst.CtxOrgID),
		Action:             input.Action,
		TargetType:         input.TargetType,
		TargetID:           input.TargetID,
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		Status:             defaultString(input.Status, "success"),
		Route:              c.FullPath(),
		Method:             c.Request.Method,
		Details:            input.Details,
		BeforeSnapshot:     beforeSnapshot,
		AfterSnapshot:      afterSnapshot,
		DiffSummary:        buildDiffSummary(input.BeforeSnapshot, input.AfterSnapshot),
		CreatedAt:          time.Now(),
	}
	return s.repo.Create(item)
}

func (s *Service) QueryLogs(input QueryInput) (QueryResult, error) {
	input = normalizeQueryInput(input)
	if input.Offset < 0 {
		return QueryResult{}, ErrInvalidPagination
	}
	items, total, stats, err := s.repo.List(repository.AuditLogQuery{
		Query:       input.Query,
		Action:      input.Action,
		TargetType:  input.TargetType,
		Status:      input.Status,
		ActorUserID: input.ActorUserID,
		ActorOrgID:  input.ActorOrgID,
		RequestID:   input.RequestID,
		TraceID:     input.TraceID,
		Limit:       input.Limit,
		Offset:      input.Offset,
	})
	if err != nil {
		return QueryResult{}, err
	}
	return QueryResult{Items: items, Total: total, Limit: input.Limit, Offset: input.Offset, Stats: stats}, nil
}

func (s *Service) GetLog(id string) (*models.AuditLog, error) {
	return s.repo.FindByID(id)
}

func normalizeQueryInput(input QueryInput) QueryInput {
	if input.Limit <= 0 {
		input.Limit = defaultQueryLimit
	}
	if input.Limit > maxQueryLimit {
		input.Limit = maxQueryLimit
	}
	return input
}

func encodeSnapshot(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func buildDiffSummary(before, after any) string {
	if before == nil && after == nil {
		return ""
	}
	if before == nil {
		keys := snapshotKeys(after)
		return "created:" + joinKeys(keys)
	}
	if after == nil {
		keys := snapshotKeys(before)
		return "deleted:" + joinKeys(keys)
	}
	beforeMap := toMap(before)
	afterMap := toMap(after)
	keySet := map[string]struct{}{}
	for key := range beforeMap {
		keySet[key] = struct{}{}
	}
	for key := range afterMap {
		keySet[key] = struct{}{}
	}
	changed := make([]string, 0, len(keySet))
	for key := range keySet {
		if !reflect.DeepEqual(beforeMap[key], afterMap[key]) {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return "changed:" + joinKeys(changed)
}

func snapshotKeys(value any) []string {
	keys := make([]string, 0)
	for key := range toMap(value) {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func toMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func joinKeys(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	data, _ := json.Marshal(keys)
	return string(data)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
