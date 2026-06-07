package organization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	access "platform-service/internal/modules/access"
	identity "platform-service/internal/modules/identity"
	"platform-service/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOrganizationTestService(t *testing.T) (*Service, *repository.CoreRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("organization-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}, &models.RolePermission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := repository.NewCoreRepository(db)
	_ = db.Create(&models.RolePermission{RoleID: "owner", PermissionID: "platform.admin"}).Error
	cfg := config.Config{}
	cfg.Security.JWTSecret = "jwt-secret"
	cfg.Security.JWTExpiration = time.Hour
	idService := identity.NewService(repo, access.NewService(repo), cfg)
	return NewService(repo, idService), repo
}

func TestOrganizationServiceAndHandler(t *testing.T) {
	service, repo := newOrganizationTestService(t)
	org1 := &models.Organization{ID: "org-1", Name: "Org 1", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	org2 := &models.Organization{ID: "org-2", Name: "Org 2", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	user := &models.User{ID: "user-1", Email: "u@example.com", FullName: "User", Status: "active", CurrentOrgID: "org-1", OrgID: "org-1", OrgRole: "owner", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	member1 := &models.OrganizationMember{ID: "m1", OrganizationID: "org-1", UserID: "user-1", Role: "owner", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	member2 := &models.OrganizationMember{ID: "m2", OrganizationID: "org-2", UserID: "user-1", Role: "owner", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = repo.DB().Create(org1).Error
	_ = repo.DB().Create(org2).Error
	_ = repo.DB().Create(user).Error
	_ = repo.DB().Create(member1).Error
	_ = repo.DB().Create(member2).Error

	items, err := service.List("user-1")
	if err != nil || len(items) != 2 {
		t.Fatalf("List: %+v err=%v", items, err)
	}
	switched, err := service.Switch("user-1", "org-2")
	if err != nil || switched.CurrentOrgID != "org-2" || switched.AccessToken == "" {
		t.Fatalf("Switch: %+v err=%v", switched, err)
	}
	updated, err := service.UpdateProfile("org-2", UpdateProfileInput{Name: "Org 2 Updated", BillingEmail: "billing@example.com"})
	if err != nil || updated.Name != "Org 2 Updated" {
		t.Fatalf("UpdateProfile: %+v err=%v", updated, err)
	}

	gin.SetMode(gin.TestMode)
	handler := NewHandler(service)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/orgs", nil)
	c.Request = req
	c.Set("userID", "user-1")
	handler.List(c)
	if w.Code != http.StatusOK {
		t.Fatalf("List handler status=%d body=%s", w.Code, w.Body.String())
	}
	payload, _ := json.Marshal(SwitchInput{OrgID: "org-1"})
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodPost, "/switch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("userID", "user-1")
	handler.Switch(c)
	if w.Code != http.StatusOK {
		t.Fatalf("Switch handler status=%d body=%s", w.Code, w.Body.String())
	}
	payload, _ = json.Marshal(UpdateProfileInput{Name: "Org 1 Updated"})
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodPut, "/internal/v1/orgs/org-1/profile", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "orgID", Value: "org-1"}}
	handler.InternalUpdateProfile(c)
	if w.Code != http.StatusOK {
		t.Fatalf("InternalUpdateProfile handler status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodGet, "/ops/organizations?limit=10&offset=0", nil)
	c.Request = req
	handler.ListAll(c)
	if w.Code != http.StatusOK {
		t.Fatalf("ListAll handler status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCreateOrganization(t *testing.T) {
	service, _ := newOrganizationTestService(t)

	t.Run("normal create with all fields", func(t *testing.T) {
		org, err := service.Create(UpsertOrganizationInput{
			Name:         "Test Org",
			PlanID:       "plan-1",
			BillingEmail: "billing@test.com",
			Status:       "active",
			OwnerID:      "owner-1",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if org.Name != "Test Org" {
			t.Fatalf("expected name 'Test Org', got %q", org.Name)
		}
		if org.Status != "active" {
			t.Fatalf("expected status 'active', got %q", org.Status)
		}
		if org.ID == "" {
			t.Fatal("expected generated ID")
		}
	})

	t.Run("empty name defaults to New Organization", func(t *testing.T) {
		org, err := service.Create(UpsertOrganizationInput{})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if org.Name != "New Organization" {
			t.Fatalf("expected name 'New Organization', got %q", org.Name)
		}
	})

	t.Run("empty status defaults to active", func(t *testing.T) {
		org, err := service.Create(UpsertOrganizationInput{Name: "StatusTest"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if org.Status != "active" {
			t.Fatalf("expected status 'active', got %q", org.Status)
		}
	})
}

func TestUpdateOrganization(t *testing.T) {
	service, _ := newOrganizationTestService(t)

	org, err := service.Create(UpsertOrganizationInput{Name: "Before", PlanID: "plan-a", BillingEmail: "a@test.com", Status: "active", OwnerID: "o1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	t.Run("partial update name only", func(t *testing.T) {
		updated, err := service.Update(org.ID, UpsertOrganizationInput{Name: "After"})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if updated.Name != "After" {
			t.Fatalf("expected name 'After', got %q", updated.Name)
		}
		if updated.PlanID != "plan-a" {
			t.Fatalf("expected plan_id unchanged 'plan-a', got %q", updated.PlanID)
		}
	})

	t.Run("full update all fields", func(t *testing.T) {
		updated, err := service.Update(org.ID, UpsertOrganizationInput{
			Name:         "Full",
			PlanID:       "plan-b",
			BillingEmail: "b@test.com",
			Status:       "inactive",
			OwnerID:      "o2",
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if updated.Name != "Full" || updated.PlanID != "plan-b" || updated.BillingEmail != "b@test.com" || updated.Status != "inactive" || updated.OwnerID != "o2" {
			t.Fatalf("unexpected updated org: %+v", updated)
		}
	})
}

func TestDeleteOrganization(t *testing.T) {
	service, repo := newOrganizationTestService(t)

	now := time.Now()
	org := &models.Organization{ID: "del-org", Name: "ToDelete", Status: "active", CreatedAt: now, UpdatedAt: now}
	user1 := &models.User{ID: "del-u1", Email: "del1@test.com", FullName: "DelUser1", Status: "active", CurrentOrgID: "del-org", OrgID: "del-org", OrgRole: "owner", LastActiveOrgID: "del-org", CreatedAt: now, UpdatedAt: now}
	user2 := &models.User{ID: "del-u2", Email: "del2@test.com", FullName: "DelUser2", Status: "active", CurrentOrgID: "other-org", OrgID: "other-org", OrgRole: "viewer", LastActiveOrgID: "del-org", CreatedAt: now, UpdatedAt: now}
	member1 := &models.OrganizationMember{ID: "del-m1", OrganizationID: "del-org", UserID: "del-u1", Role: "owner", Status: "active", CreatedAt: now, UpdatedAt: now}
	member2 := &models.OrganizationMember{ID: "del-m2", OrganizationID: "del-org", UserID: "del-u2", Role: "viewer", Status: "active", CreatedAt: now, UpdatedAt: now}

	for _, item := range []any{org, user1, user2, member1, member2} {
		if err := repo.DB().Create(item).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	if err := service.Delete("del-org"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// members should be deleted
	var memberCount int64
	repo.DB().Model(&models.OrganizationMember{}).Where("organization_id = ?", "del-org").Count(&memberCount)
	if memberCount != 0 {
		t.Fatalf("expected 0 members, got %d", memberCount)
	}

	// user1 current_org was del-org → should be cleared
	var u1 models.User
	repo.DB().Where("id = ?", "del-u1").First(&u1)
	if u1.CurrentOrgID != "" || u1.OrgID != "" || u1.OrgRole != "" {
		t.Fatalf("user1 org fields not cleared: current_org=%q org_id=%q org_role=%q", u1.CurrentOrgID, u1.OrgID, u1.OrgRole)
	}

	// user2 current_org was other-org → should NOT be cleared, but last_active_org should be cleared
	var u2 models.User
	repo.DB().Where("id = ?", "del-u2").First(&u2)
	if u2.LastActiveOrgID != "" {
		t.Fatalf("user2 last_active_org_id not cleared: %q", u2.LastActiveOrgID)
	}
	if u2.CurrentOrgID != "other-org" {
		t.Fatalf("user2 current_org_id should remain 'other-org', got %q", u2.CurrentOrgID)
	}

	// org should be deleted
	var orgCount int64
	repo.DB().Model(&models.Organization{}).Where("id = ?", "del-org").Count(&orgCount)
	if orgCount != 0 {
		t.Fatalf("expected org deleted, got count %d", orgCount)
	}
}

func TestListAllWithFilters(t *testing.T) {
	service, repo := newOrganizationTestService(t)

	now := time.Now()
	owner := &models.User{ID: "list-owner", Email: "owner@test.com", FullName: "OwnerUser", Status: "active", CreatedAt: now, UpdatedAt: now}
	org1 := &models.Organization{ID: "la-1", Name: "Alpha Corp", Status: "active", OwnerID: "list-owner", CreatedAt: now, UpdatedAt: now}
	org2 := &models.Organization{ID: "la-2", Name: "Beta Inc", Status: "inactive", OwnerID: "list-owner", CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)}
	org3 := &models.Organization{ID: "la-3", Name: "Alpha Two", Status: "active", CreatedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second)}
	mem1 := &models.OrganizationMember{ID: "la-m1", OrganizationID: "la-1", UserID: "list-owner", Role: "owner", Status: "active", CreatedAt: now, UpdatedAt: now}
	mem2 := &models.OrganizationMember{ID: "la-m2", OrganizationID: "la-1", UserID: "la-extra", Role: "viewer", Status: "active", CreatedAt: now, UpdatedAt: now}

	for _, item := range []any{owner, org1, org2, org3, mem1, mem2} {
		if err := repo.DB().Create(item).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	t.Run("no filter returns all", func(t *testing.T) {
		result, err := service.ListAll(ListAllInput{Limit: 10, Offset: 0})
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		if result.Total != 3 {
			t.Fatalf("expected total 3, got %d", result.Total)
		}
	})

	t.Run("query filter", func(t *testing.T) {
		result, err := service.ListAll(ListAllInput{Query: "Alpha", Limit: 10})
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		if result.Total != 2 {
			t.Fatalf("expected total 2 for 'Alpha' query, got %d", result.Total)
		}
	})

	t.Run("status filter", func(t *testing.T) {
		result, err := service.ListAll(ListAllInput{Status: "inactive", Limit: 10})
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		if result.Total != 1 {
			t.Fatalf("expected total 1 for inactive, got %d", result.Total)
		}
		if result.Items[0].ID != "la-2" {
			t.Fatalf("expected la-2, got %q", result.Items[0].ID)
		}
	})

	t.Run("owner enrichment", func(t *testing.T) {
		result, err := service.ListAll(ListAllInput{Query: "Alpha Corp", Limit: 10})
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		if len(result.Items) == 0 {
			t.Fatal("expected at least 1 item")
		}
		item := result.Items[0]
		if item.OwnerName != "OwnerUser" || item.OwnerEmail != "owner@test.com" {
			t.Fatalf("owner enrichment failed: name=%q email=%q", item.OwnerName, item.OwnerEmail)
		}
	})

	t.Run("member count", func(t *testing.T) {
		result, err := service.ListAll(ListAllInput{Query: "Alpha Corp", Limit: 10})
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		if len(result.Items) == 0 {
			t.Fatal("expected items")
		}
		if result.Items[0].MemberCount != 2 {
			t.Fatalf("expected member count 2, got %d", result.Items[0].MemberCount)
		}
	})

	t.Run("default limit and offset", func(t *testing.T) {
		result, err := service.ListAll(ListAllInput{})
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		if result.Limit != 20 {
			t.Fatalf("expected default limit 20, got %d", result.Limit)
		}
		if result.Offset != 0 {
			t.Fatalf("expected default offset 0, got %d", result.Offset)
		}
	})
}

func TestMemberCRUD(t *testing.T) {
	service, repo := newOrganizationTestService(t)

	now := time.Now()
	org := &models.Organization{ID: "mem-org", Name: "Member Org", Status: "active", OwnerID: "mem-u1", CreatedAt: now, UpdatedAt: now}
	user1 := &models.User{ID: "mem-u1", Email: "m1@test.com", FullName: "MemberUser1", Status: "active", CreatedAt: now, UpdatedAt: now}
	user2 := &models.User{ID: "mem-u2", Email: "m2@test.com", FullName: "MemberUser2", Status: "active", AvatarURL: "https://avatar.test/2", CreatedAt: now, UpdatedAt: now}

	for _, item := range []any{org, user1, user2} {
		if err := repo.DB().Create(item).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	t.Run("CreateMember default role", func(t *testing.T) {
		rec, err := service.CreateMember("mem-org", UpsertMembershipInput{UserID: "mem-u1"})
		if err != nil {
			t.Fatalf("CreateMember: %v", err)
		}
		if rec.Role != "viewer" {
			t.Fatalf("expected default role 'viewer', got %q", rec.Role)
		}
		if rec.UserEmail != "m1@test.com" {
			t.Fatalf("expected enriched email, got %q", rec.UserEmail)
		}
	})

	t.Run("CreateMember specified role + syncUserPrimaryOrg", func(t *testing.T) {
		rec, err := service.CreateMember("mem-org", UpsertMembershipInput{UserID: "mem-u2", Role: "editor"})
		if err != nil {
			t.Fatalf("CreateMember: %v", err)
		}
		if rec.Role != "editor" {
			t.Fatalf("expected role 'editor', got %q", rec.Role)
		}
		// user2 had empty org fields → syncUserPrimaryOrg should fill them
		var u2 models.User
		repo.DB().Where("id = ?", "mem-u2").First(&u2)
		if u2.CurrentOrgID != "mem-org" {
			t.Fatalf("expected current_org_id 'mem-org', got %q", u2.CurrentOrgID)
		}
		if u2.OrgID != "mem-org" {
			t.Fatalf("expected org_id 'mem-org', got %q", u2.OrgID)
		}
		if u2.OrgRole != "editor" {
			t.Fatalf("expected org_role 'editor', got %q", u2.OrgRole)
		}
	})

	t.Run("ListMembers enrichment and IsCurrentOwner", func(t *testing.T) {
		result, err := service.ListMembers("mem-org")
		if err != nil {
			t.Fatalf("ListMembers: %v", err)
		}
		if result.Total != 2 {
			t.Fatalf("expected 2 members, got %d", result.Total)
		}
		for _, item := range result.Items {
			if item.UserID == "mem-u1" {
				if !item.IsCurrentOwner {
					t.Fatal("expected mem-u1 to be current owner")
				}
				if item.UserFullName != "MemberUser1" {
					t.Fatalf("expected enriched full name, got %q", item.UserFullName)
				}
			}
			if item.UserID == "mem-u2" {
				if item.IsCurrentOwner {
					t.Fatal("expected mem-u2 NOT to be current owner")
				}
				if item.UserAvatarURL != "https://avatar.test/2" {
					t.Fatalf("expected enriched avatar url, got %q", item.UserAvatarURL)
				}
			}
		}
	})

	t.Run("UpdateMember role change", func(t *testing.T) {
		rec, err := service.UpdateMember("mem-org", "mem-u2", UpsertMembershipInput{Role: "admin"})
		if err != nil {
			t.Fatalf("UpdateMember: %v", err)
		}
		if rec.Role != "admin" {
			t.Fatalf("expected role 'admin', got %q", rec.Role)
		}
		// sync effect: user2 current_org is mem-org → org_role should update
		var u2 models.User
		repo.DB().Where("id = ?", "mem-u2").First(&u2)
		if u2.OrgRole != "admin" {
			t.Fatalf("expected org_role 'admin' after sync, got %q", u2.OrgRole)
		}
	})

	t.Run("DeleteMember clears primary org", func(t *testing.T) {
		err := service.DeleteMember("mem-org", "mem-u2")
		if err != nil {
			t.Fatalf("DeleteMember: %v", err)
		}
		var u2 models.User
		repo.DB().Where("id = ?", "mem-u2").First(&u2)
		if u2.CurrentOrgID != "" {
			t.Fatalf("expected current_org_id cleared, got %q", u2.CurrentOrgID)
		}
		if u2.OrgID != "" {
			t.Fatalf("expected org_id cleared, got %q", u2.OrgID)
		}

		// verify member is gone
		result, err := service.ListMembers("mem-org")
		if err != nil {
			t.Fatalf("ListMembers after delete: %v", err)
		}
		if result.Total != 1 {
			t.Fatalf("expected 1 member after delete, got %d", result.Total)
		}
	})
}

func TestDefaultMemberRole(t *testing.T) {
	if r := defaultMemberRole(""); r != "viewer" {
		t.Fatalf("empty → expected 'viewer', got %q", r)
	}
	if r := defaultMemberRole("   "); r != "viewer" {
		t.Fatalf("spaces → expected 'viewer', got %q", r)
	}
	if r := defaultMemberRole("admin"); r != "admin" {
		t.Fatalf("admin → expected 'admin', got %q", r)
	}
	if r := defaultMemberRole(" editor "); r != "editor" {
		t.Fatalf("' editor ' → expected 'editor', got %q", r)
	}
}

func TestOrganizationHandlerCRUD(t *testing.T) {
	service, repo := newOrganizationTestService(t)
	gin.SetMode(gin.TestMode)
	handler := NewHandler(service)

	now := time.Now()
	user := &models.User{ID: "h-u1", Email: "h1@test.com", FullName: "HandlerUser", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := repo.DB().Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var createdOrgID string

	t.Run("Create handler", func(t *testing.T) {
		payload, _ := json.Marshal(UpsertOrganizationInput{Name: "Handler Org", Status: "active", OwnerID: "h-u1"})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPost, "/ops/organizations", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req
		handler.Create(c)
		if w.Code != http.StatusCreated {
			t.Fatalf("Create handler status=%d body=%s", w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		createdOrgID = resp.Data.ID
		if createdOrgID == "" {
			t.Fatal("expected org ID in response")
		}
	})

	t.Run("Update handler", func(t *testing.T) {
		payload, _ := json.Marshal(UpsertOrganizationInput{Name: "Handler Org Updated"})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPut, "/ops/organizations/"+createdOrgID, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req
		c.Params = gin.Params{{Key: "orgID", Value: createdOrgID}}
		handler.Update(c)
		if w.Code != http.StatusOK {
			t.Fatalf("Update handler status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("CreateMember handler", func(t *testing.T) {
		payload, _ := json.Marshal(UpsertMembershipInput{UserID: "h-u1", Role: "owner"})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPost, "/ops/organizations/"+createdOrgID+"/members", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req
		c.Params = gin.Params{{Key: "orgID", Value: createdOrgID}}
		handler.CreateMember(c)
		if w.Code != http.StatusCreated {
			t.Fatalf("CreateMember handler status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("ListMembers handler", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodGet, "/ops/organizations/"+createdOrgID+"/members", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "orgID", Value: createdOrgID}}
		handler.ListMembers(c)
		if w.Code != http.StatusOK {
			t.Fatalf("ListMembers handler status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("UpdateMember handler", func(t *testing.T) {
		payload, _ := json.Marshal(UpsertMembershipInput{Role: "admin"})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPut, "/ops/organizations/"+createdOrgID+"/members/h-u1", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req
		c.Params = gin.Params{{Key: "orgID", Value: createdOrgID}, {Key: "userID", Value: "h-u1"}}
		handler.UpdateMember(c)
		if w.Code != http.StatusOK {
			t.Fatalf("UpdateMember handler status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("DeleteMember handler", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodDelete, "/ops/organizations/"+createdOrgID+"/members/h-u1", nil)
		c.Request = req
		c.Params = gin.Params{{Key: "orgID", Value: createdOrgID}, {Key: "userID", Value: "h-u1"}}
		handler.DeleteMember(c)
		if w.Code != http.StatusOK {
			t.Fatalf("DeleteMember handler status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("Delete handler", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodDelete, "/ops/organizations/"+createdOrgID, nil)
		c.Request = req
		c.Params = gin.Params{{Key: "orgID", Value: createdOrgID}}
		handler.Delete(c)
		if w.Code != http.StatusOK {
			t.Fatalf("Delete handler status=%d body=%s", w.Code, w.Body.String())
		}
	})
}
