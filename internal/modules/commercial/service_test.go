package commercial

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCommercialTestService(t *testing.T) (*Service, *repository.CommercialRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("commercial-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Product{}, &models.CommercialEntity{}, &models.BillingProfile{}, &models.RoutingPolicy{}, &models.OrgBillingProfile{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := repository.NewCommercialRepository(db)
	return NewService(repo), repo
}

func TestCommercialServiceCrudAndResolveRoute(t *testing.T) {
	service, repo := newCommercialTestService(t)
	product := &models.Product{
		ID:        "prod-1",
		Code:      "ecommerce",
		Name:      "Agent Ecommerce",
		Status:    "active",
		OwnerTeam: "ecommerce",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateProduct(product); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	entity, err := service.CreateCommercialEntity(CreateCommercialEntityInput{
		Code:       "ecom-cn",
		Name:       "Ecommerce CN",
		EntityType: "product_operator",
		Currency:   "CNY",
	})
	if err != nil {
		t.Fatalf("CreateCommercialEntity: %v", err)
	}
	entities, err := service.ListCommercialEntities()
	if err != nil || len(entities) != 1 {
		t.Fatalf("ListCommercialEntities: %+v err=%v", entities, err)
	}
	profile, err := service.CreateBillingProfile(CreateBillingProfileInput{
		Code:               "bp-ecom-default",
		ProductID:          product.ID,
		CommercialEntityID: entity.ID,
		RegionScope:        "CN",
		Currency:           "CNY",
		Status:             "active",
	})
	if err != nil {
		t.Fatalf("CreateBillingProfile: %v", err)
	}
	if _, err := service.GetBillingProfile(profile.ID); err != nil {
		t.Fatalf("GetBillingProfile: %v", err)
	}
	policy, err := service.CreateRoutingPolicy(CreateRoutingPolicyInput{
		BillingProfileID: profile.ID,
		Priority:         10,
		MatchType:        "channel",
		MatchConfig:      `{"channel":"wechat"}`,
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("CreateRoutingPolicy: %v", err)
	}
	updated, err := service.UpdateRoutingPolicy(policy.ID, UpdateRoutingPolicyInput{Status: "inactive"})
	if err != nil || updated.Status != "inactive" {
		t.Fatalf("UpdateRoutingPolicy: %+v err=%v", updated, err)
	}
	if err := repo.DB().Create(&models.OrgBillingProfile{
		ID:               "org-bp-1",
		OrganizationID:   "org-1",
		BillingProfileID: profile.ID,
		IsDefault:        true,
		Status:           "active",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}).Error; err != nil {
		t.Fatalf("CreateOrgBillingProfile: %v", err)
	}
	result, err := service.ResolveRoute(ResolveRouteInput{
		OrganizationID: "org-1",
		Channel:        "wechat",
	})
	if err != nil || result.BillingProfileID != profile.ID {
		t.Fatalf("ResolveRoute: %+v err=%v", result, err)
	}
	deleted, err := service.DeleteRoutingPolicy(policy.ID)
	if err != nil || deleted.ID != policy.ID {
		t.Fatalf("DeleteRoutingPolicy: %+v err=%v", deleted, err)
	}
}
