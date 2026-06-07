package repository

import (
	"testing"
	"time"

	"platform-service/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuditRepositoryListFindAndStats(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:audit-repository?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := NewAuditRepository(db)
	base := time.Now().Add(-time.Hour)
	logs := []models.AuditLog{
		{ID: "audit-repo-1", RequestID: "req-1", TraceID: "trace-1", ActorUserID: "user-1", ActorOrgID: "org-1", Action: "wallet.grant", TargetType: "wallet", TargetID: "wallet-1", Status: "success", Route: "/wallet", Details: "grant promo", CreatedAt: base},
		{ID: "audit-repo-2", RequestID: "req-2", TraceID: "trace-2", ActorUserID: "user-1", ActorOrgID: "org-1", Action: "wallet.consume", TargetType: "wallet", TargetID: "wallet-1", Status: "failure", Route: "/wallet", Details: "insufficient quota", CreatedAt: base.Add(time.Minute)},
		{ID: "audit-repo-3", RequestID: "req-3", TraceID: "trace-3", ActorUserID: "user-2", ActorOrgID: "org-2", Action: "runtime.job.create", TargetType: "runtime_job", TargetID: "runtime-1", Status: "success", Route: "/runtime", Details: "job created", CreatedAt: base.Add(2 * time.Minute)},
	}
	for i := range logs {
		if err := repo.Create(&logs[i]); err != nil {
			t.Fatalf("Create audit log %d: %v", i, err)
		}
	}

	items, total, stats, err := repo.List(AuditLogQuery{Query: "wallet", ActorUserID: "user-1", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 || len(items) != 2 || items[0].ID != "audit-repo-2" {
		t.Fatalf("unexpected list result total=%d items=%+v", total, items)
	}
	if stats.Total != 2 || stats.SuccessCount != 1 || stats.FailureCount != 1 || stats.DistinctActions != 2 || stats.ByStatus["success"] != 1 || stats.ByTargetType["wallet"] != 2 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.LatestCreatedAt == nil {
		t.Fatalf("expected latest_created_at")
	}
	item, err := repo.FindByID("audit-repo-1")
	if err != nil || item.Action != "wallet.grant" {
		t.Fatalf("FindByID: item=%+v err=%v", item, err)
	}
	filtered, err := repo.Stats(AuditLogQuery{Action: "runtime.job.create", TargetType: "runtime_job", Status: "success", ActorOrgID: "org-2", RequestID: "req-3", TraceID: "trace-3"})
	if err != nil {
		t.Fatalf("Stats filtered: %v", err)
	}
	if filtered.Total != 1 || filtered.ByAction["runtime.job.create"] != 1 {
		t.Fatalf("unexpected filtered stats: %+v", filtered)
	}
}
