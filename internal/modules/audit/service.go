package audit

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
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

type DiagnosticsInput struct {
	RequestID string `form:"-"`
	TraceID   string `form:"trace_id"`
	Lookback  string `form:"lookback"`
	Limit     int    `form:"limit"`
}

type RequestDiagnosticsResult struct {
	RequestID          string                     `json:"request_id"`
	TraceID            string                     `json:"trace_id,omitempty"`
	LogQuery           string                     `json:"log_query,omitempty"`
	LogSummary         DiagnosticsLogSummary      `json:"log_summary"`
	TraceSummary       DiagnosticsTraceSummary    `json:"trace_summary"`
	OperatorSummary    DiagnosticsOperatorSummary `json:"operator_summary,omitempty"`
	Findings           []DiagnosticsFinding       `json:"findings"`
	LogLines           []DiagnosticsLogLine       `json:"log_lines,omitempty"`
	Spans              []DiagnosticsSpanSummary   `json:"spans,omitempty"`
	ExternalURLs       map[string]string          `json:"external_urls,omitempty"`
	DiagnosticsEnabled bool                       `json:"diagnostics_enabled"`
}

type DiagnosticsLogSummary struct {
	TotalLines  int      `json:"total_lines"`
	Services    []string `json:"services"`
	Routes      []string `json:"routes"`
	Statuses    []int    `json:"statuses"`
	ErrorCodes  []string `json:"error_codes"`
	FirstSeenAt string   `json:"first_seen_at,omitempty"`
	LastSeenAt  string   `json:"last_seen_at,omitempty"`
}

type DiagnosticsTraceSummary struct {
	Found          bool     `json:"found"`
	SpanCount      int      `json:"span_count"`
	ServiceNames   []string `json:"service_names"`
	RootOperation  string   `json:"root_operation,omitempty"`
	DurationMS     int64    `json:"duration_ms,omitempty"`
	ErrorSpanCount int      `json:"error_span_count"`
}

type DiagnosticsFinding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type DiagnosticsLogLine struct {
	Timestamp string         `json:"timestamp,omitempty"`
	Service   string         `json:"service,omitempty"`
	Level     string         `json:"level,omitempty"`
	Message   string         `json:"message,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
}

type DiagnosticsSpanSummary struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	Service      string            `json:"service,omitempty"`
	Name         string            `json:"name"`
	DurationMS   int64             `json:"duration_ms,omitempty"`
	Status       string            `json:"status,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

type DiagnosticsOperatorSummary struct {
	RequestPath           []DiagnosticsRequestPathStep      `json:"request_path"`
	ParticipatingServices []string                          `json:"participating_services"`
	BusinessStages        []DiagnosticsBusinessStageSummary `json:"business_stages"`
	Failure               *DiagnosticsFailureSummary        `json:"failure,omitempty"`
	LikelyCause           string                            `json:"likely_cause"`
	NextSteps             []string                          `json:"next_steps"`
}

type DiagnosticsRequestPathStep struct {
	Timestamp string `json:"timestamp,omitempty"`
	Service   string `json:"service,omitempty"`
	Operation string `json:"operation,omitempty"`
	Route     string `json:"route,omitempty"`
	Status    int    `json:"status,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

type DiagnosticsBusinessStageSummary struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Service   string `json:"service,omitempty"`
	Operation string `json:"operation,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

type DiagnosticsFailureSummary struct {
	Category  string `json:"category"`
	Stage     string `json:"stage,omitempty"`
	Service   string `json:"service,omitempty"`
	Operation string `json:"operation,omitempty"`
	Status    int    `json:"status,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message,omitempty"`
}

var ErrInvalidPagination = errors.New("invalid audit log pagination")
var ErrMissingDiagnosticsRequestID = errors.New("missing diagnostics request_id")

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

func (s *Service) GetRequestDiagnostics(input DiagnosticsInput) (RequestDiagnosticsResult, error) {
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.TraceID = strings.TrimSpace(input.TraceID)
	if input.RequestID == "" {
		return RequestDiagnosticsResult{}, ErrMissingDiagnosticsRequestID
	}
	limit := input.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}
	query := QueryInput{RequestID: input.RequestID, TraceID: input.TraceID, Limit: limit}
	result, err := s.QueryLogs(query)
	if err != nil {
		return RequestDiagnosticsResult{}, err
	}
	return buildRequestDiagnostics(input, result.Items), nil
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

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func buildRequestDiagnostics(input DiagnosticsInput, items []models.AuditLog) RequestDiagnosticsResult {
	traceID := input.TraceID
	for _, item := range items {
		if traceID == "" && item.TraceID != "" {
			traceID = item.TraceID
		}
	}
	lines := buildDiagnosticsLogLines(items)
	services := []string{}
	if len(items) > 0 {
		services = []string{"platform-service"}
	}
	routes := uniqueStringsFromAudit(items, func(item models.AuditLog) string { return item.Route })
	errorCodes := uniqueStringsFromAudit(items, func(item models.AuditLog) string {
		if strings.EqualFold(item.Status, "success") || item.Status == "" {
			return ""
		}
		return "AUDIT_STATUS_" + strings.ToUpper(strings.ReplaceAll(item.Status, "-", "_"))
	})
	findings := []DiagnosticsFinding{
		{Severity: "info", Code: "AUDIT_DIAGNOSTICS_AUDIT_FACTS", Message: "Embedded diagnostics are derived from platform_audit_logs and sanitized audit fields. Raw stdout logs and trace spans remain external until a log/trace backend is connected."},
	}
	if len(items) == 0 {
		findings = append(findings, DiagnosticsFinding{Severity: "warning", Code: "AUDIT_DIAGNOSTICS_AUDIT_FACT_NOT_FOUND", Message: "No platform audit row matched this request_id/trace_id. Use the generated search key in container logs or external log search to inspect transport-level failures."})
	}
	if len(items) >= defaultInt(input.Limit, defaultQueryLimit) && input.Limit > 0 {
		findings = append(findings, DiagnosticsFinding{Severity: "warning", Code: "AUDIT_DIAGNOSTICS_RESULT_LIMIT_REACHED", Message: "Diagnostics result reached the requested limit; narrow request_id/trace_id or raise limit up to 200 if needed."})
	}
	summary := DiagnosticsLogSummary{
		TotalLines: len(lines),
		Services:   services,
		Routes:     routes,
		Statuses:   []int{},
		ErrorCodes: errorCodes,
	}
	if len(items) > 0 {
		newest := items[0].CreatedAt
		oldest := items[len(items)-1].CreatedAt
		summary.FirstSeenAt = oldest.Format(time.RFC3339Nano)
		summary.LastSeenAt = newest.Format(time.RFC3339Nano)
	}
	traceServices := []string{}
	if traceID != "" && len(items) > 0 {
		traceServices = services
	}
	operator := DiagnosticsOperatorSummary{
		RequestPath:           buildDiagnosticsRequestPath(items),
		ParticipatingServices: services,
		BusinessStages:        buildDiagnosticsBusinessStages(items),
		Failure:               buildDiagnosticsFailure(items),
		LikelyCause:           diagnosticsLikelyCause(items),
		NextSteps: []string{
			"Open the matching audit row to inspect actor, target, route, before/after snapshots, and diff summary.",
			"Search container or external logs with request_id for transport-level details that are intentionally not stored in platform_audit_logs.",
			"Use trace_id in the configured trace backend after tracing is enabled for this environment.",
		},
	}
	return RequestDiagnosticsResult{
		RequestID:  input.RequestID,
		TraceID:    traceID,
		LogQuery:   strings.TrimSpace("request_id=" + input.RequestID + defaultStringIf(input.TraceID != "", " trace_id="+input.TraceID, "")),
		LogSummary: summary,
		TraceSummary: DiagnosticsTraceSummary{
			Found:          traceID != "" && len(items) > 0,
			SpanCount:      0,
			ServiceNames:   traceServices,
			RootOperation:  firstNonEmptyRoute(items),
			ErrorSpanCount: 0,
		},
		OperatorSummary:    operator,
		Findings:           findings,
		LogLines:           lines,
		Spans:              []DiagnosticsSpanSummary{},
		DiagnosticsEnabled: true,
	}
}

func buildDiagnosticsLogLines(items []models.AuditLog) []DiagnosticsLogLine {
	lines := make([]DiagnosticsLogLine, 0, len(items))
	for _, item := range items {
		message := strings.TrimSpace(strings.Join([]string{item.Action, item.TargetType, item.TargetID, item.Details}, " "))
		fields := map[string]any{
			"request_id":             item.RequestID,
			"trace_id":               item.TraceID,
			"method":                 item.Method,
			"route":                  item.Route,
			"action":                 item.Action,
			"target_type":            item.TargetType,
			"target_id":              item.TargetID,
			"audit_status":           item.Status,
			"billing_subject_type":   item.BillingSubjectType,
			"billing_subject_id":     item.BillingSubjectID,
			"diff_summary_available": item.DiffSummary != "",
		}
		lines = append(lines, DiagnosticsLogLine{
			Timestamp: item.CreatedAt.Format(time.RFC3339Nano),
			Service:   "platform-service",
			Level:     auditStatusLevel(item.Status),
			Message:   message,
			Fields:    fields,
		})
	}
	return lines
}

func buildDiagnosticsRequestPath(items []models.AuditLog) []DiagnosticsRequestPathStep {
	steps := make([]DiagnosticsRequestPathStep, 0, len(items))
	for _, item := range items {
		steps = append(steps, DiagnosticsRequestPathStep{
			Timestamp: item.CreatedAt.Format(time.RFC3339Nano),
			Service:   "platform-service",
			Operation: item.Action,
			Route:     strings.TrimSpace(item.Method + " " + item.Route),
			Outcome:   auditOutcome(item.Status),
			ErrorCode: auditErrorCode(item.Status),
		})
	}
	if len(steps) == 0 {
		return []DiagnosticsRequestPathStep{{Service: "platform-service", Operation: "request_id correlation", Outcome: "warning"}}
	}
	return steps
}

func buildDiagnosticsBusinessStages(items []models.AuditLog) []DiagnosticsBusinessStageSummary {
	stages := make([]DiagnosticsBusinessStageSummary, 0, len(items))
	for _, item := range items {
		stages = append(stages, DiagnosticsBusinessStageSummary{
			Name:      defaultString(item.TargetType, "audit_event"),
			Status:    auditOutcome(item.Status),
			Service:   "platform-service",
			Operation: item.Action,
			ErrorCode: auditErrorCode(item.Status),
		})
	}
	return stages
}

func buildDiagnosticsFailure(items []models.AuditLog) *DiagnosticsFailureSummary {
	for _, item := range items {
		if auditOutcome(item.Status) == "failed" {
			return &DiagnosticsFailureSummary{
				Category:  "audit_status",
				Stage:     defaultString(item.TargetType, "audit_event"),
				Service:   "platform-service",
				Operation: item.Action,
				ErrorCode: auditErrorCode(item.Status),
				Message:   "Audit event status was not success; inspect the audit row and correlated request logs.",
			}
		}
	}
	return nil
}

func diagnosticsLikelyCause(items []models.AuditLog) string {
	if len(items) == 0 {
		return "No audit fact was recorded for this request_id. The request may have failed before a business audit event, or the request_id belongs to a non-audited read path."
	}
	if buildDiagnosticsFailure(items) != nil {
		return "A correlated audit event has non-success status; inspect its target and diff summary, then use request_id to search raw request logs."
	}
	return "Correlated audit facts were found and no non-success audit status was present. Continue with external logs or trace backend for transport/runtime details."
}

func auditStatusLevel(status string) string {
	switch strings.ToLower(status) {
	case "", "success", "ok", "completed":
		return "info"
	case "failed", "failure", "error", "denied":
		return "warning"
	default:
		return "info"
	}
}

func auditOutcome(status string) string {
	switch strings.ToLower(status) {
	case "", "success", "ok", "completed":
		return "ok"
	case "failed", "failure", "error", "denied":
		return "failed"
	default:
		return "warning"
	}
}

func auditErrorCode(status string) string {
	if auditOutcome(status) != "failed" {
		return ""
	}
	return "AUDIT_STATUS_" + strings.ToUpper(strings.ReplaceAll(status, "-", "_"))
}

func uniqueStringsFromAudit(items []models.AuditLog, pick func(models.AuditLog) string) []string {
	seen := map[string]struct{}{}
	values := make([]string, 0)
	for _, item := range items {
		value := strings.TrimSpace(pick(item))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func firstNonEmptyRoute(items []models.AuditLog) string {
	for _, item := range items {
		if item.Route != "" {
			return item.Route
		}
	}
	return ""
}

func defaultStringIf(ok bool, value, fallback string) string {
	if ok {
		return value
	}
	return fallback
}
