package repository

import (
	"strings"
	"time"

	"platform-service/internal/models"

	"gorm.io/gorm"
)

type AuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(item *models.AuditLog) error {
	return r.db.Create(item).Error
}

type AuditLogQuery struct {
	Query       string
	Action      string
	TargetType  string
	Status      string
	ActorUserID string
	ActorOrgID  string
	RequestID   string
	TraceID     string
	Limit       int
	Offset      int
}

type AuditLogStats struct {
	Total           int64            `json:"total"`
	SuccessCount    int64            `json:"success_count"`
	FailureCount    int64            `json:"failure_count"`
	DistinctActions int64            `json:"distinct_actions"`
	LatestCreatedAt *time.Time       `json:"latest_created_at,omitempty"`
	ByStatus        map[string]int64 `json:"by_status"`
	ByAction        map[string]int64 `json:"by_action"`
	ByTargetType    map[string]int64 `json:"by_target_type"`
}

func (r *AuditRepository) List(query AuditLogQuery) ([]models.AuditLog, int64, AuditLogStats, error) {
	var total int64
	if err := r.applyFilters(r.db.Model(&models.AuditLog{}), query).Count(&total).Error; err != nil {
		return nil, 0, AuditLogStats{}, err
	}

	stats, err := r.Stats(query)
	if err != nil {
		return nil, 0, AuditLogStats{}, err
	}

	items := make([]models.AuditLog, 0)
	if err := r.applyFilters(r.db.Model(&models.AuditLog{}), query).
		Order("created_at DESC").
		Limit(query.Limit).
		Offset(query.Offset).
		Find(&items).Error; err != nil {
		return nil, 0, AuditLogStats{}, err
	}
	return items, total, stats, nil
}

func (r *AuditRepository) FindByID(id string) (*models.AuditLog, error) {
	var item models.AuditLog
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *AuditRepository) Stats(query AuditLogQuery) (AuditLogStats, error) {
	stats := AuditLogStats{
		ByStatus:     map[string]int64{},
		ByAction:     map[string]int64{},
		ByTargetType: map[string]int64{},
	}
	if err := r.applyFilters(r.db.Model(&models.AuditLog{}), query).Count(&stats.Total).Error; err != nil {
		return stats, err
	}
	if err := r.applyFilters(r.db.Model(&models.AuditLog{}), query).Where("status = ?", "success").Count(&stats.SuccessCount).Error; err != nil {
		return stats, err
	}
	if err := r.applyFilters(r.db.Model(&models.AuditLog{}), query).Where("status <> ?", "success").Count(&stats.FailureCount).Error; err != nil {
		return stats, err
	}
	type distinctRow struct {
		DistinctActions int64
	}
	var distinct distinctRow
	if err := r.applyFilters(r.db.Model(&models.AuditLog{}), query).
		Select("COUNT(DISTINCT action) AS distinct_actions").
		Scan(&distinct).Error; err != nil {
		return stats, err
	}
	var latest models.AuditLog
	err := r.applyFilters(r.db.Model(&models.AuditLog{}), query).
		Select("created_at").
		Order("created_at DESC").
		Limit(1).
		First(&latest).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return stats, err
	}
	if err == nil {
		stats.LatestCreatedAt = &latest.CreatedAt
	}
	stats.DistinctActions = distinct.DistinctActions
	if stats.ByStatus, err = r.groupCount(query, "status"); err != nil {
		return stats, err
	}
	if stats.ByAction, err = r.groupCount(query, "action"); err != nil {
		return stats, err
	}
	if stats.ByTargetType, err = r.groupCount(query, "target_type"); err != nil {
		return stats, err
	}
	return stats, nil
}

type auditGroupCountRow struct {
	Key   string
	Count int64
}

func (r *AuditRepository) groupCount(query AuditLogQuery, column string) (map[string]int64, error) {
	rows := make([]auditGroupCountRow, 0)
	if err := r.applyFilters(r.db.Model(&models.AuditLog{}), query).
		Select(column + " AS key, COUNT(*) AS count").
		Where(column + " <> ''").
		Group(column).
		Order("count DESC").
		Limit(20).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		result[row.Key] = row.Count
	}
	return result, nil
}

func (r *AuditRepository) applyFilters(db *gorm.DB, query AuditLogQuery) *gorm.DB {
	if query.Query != "" {
		pattern := "%" + strings.ToLower(query.Query) + "%"
		db = db.Where(`lower(request_id) LIKE ? OR lower(trace_id) LIKE ? OR lower(actor_user_id) LIKE ? OR lower(actor_org_id) LIKE ? OR lower(action) LIKE ? OR lower(target_type) LIKE ? OR lower(target_id) LIKE ? OR lower(route) LIKE ? OR lower(details) LIKE ?`,
			pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if query.Action != "" {
		db = db.Where("action = ?", query.Action)
	}
	if query.TargetType != "" {
		db = db.Where("target_type = ?", query.TargetType)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.ActorUserID != "" {
		db = db.Where("actor_user_id = ?", query.ActorUserID)
	}
	if query.ActorOrgID != "" {
		db = db.Where("actor_org_id = ?", query.ActorOrgID)
	}
	if query.RequestID != "" {
		db = db.Where("request_id = ?", query.RequestID)
	}
	if query.TraceID != "" {
		db = db.Where("trace_id = ?", query.TraceID)
	}
	return db
}
