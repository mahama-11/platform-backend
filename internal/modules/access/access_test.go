package access

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAccessTestService(t *testing.T) (*Service, *repository.CoreRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("access-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}, &models.Permission{}, &models.Role{}, &models.RolePermission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := repository.NewCoreRepository(db)
	return NewService(repo), repo
}

func TestSeedDefaultsAndAccessContext(t *testing.T) {
	service, repo := newAccessTestService(t)
	if err := SeedDefaults(repo.DB()); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	org := &models.Organization{ID: "org-1", Name: "Org 1", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	user := &models.User{ID: "user-1", Email: "u@example.com", FullName: "User", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	member := &models.OrganizationMember{ID: "member-1", OrganizationID: org.ID, UserID: user.ID, Role: "owner", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := repo.DB().Create(org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	if err := repo.DB().Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := repo.DB().Create(member).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	perms, err := service.PermissionsForRole("owner")
	if err != nil || len(perms) == 0 {
		t.Fatalf("PermissionsForRole: %+v err=%v", perms, err)
	}
	ctx, err := service.AccessContext(user.ID, org.ID)
	if err != nil || ctx.UserID != user.ID || len(ctx.Permissions) == 0 {
		t.Fatalf("AccessContext: %+v err=%v", ctx, err)
	}
}

func TestSeedDefaultsRolePermissionSemanticsAndCleanup(t *testing.T) {
	_, repo := newAccessTestService(t)
	if err := SeedDefaults(repo.DB()); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	stale := models.RolePermission{RoleID: "viewer", PermissionID: "billing.read"}
	if err := repo.DB().FirstOrCreate(&stale, stale).Error; err != nil {
		t.Fatalf("create stale permission: %v", err)
	}
	if err := SeedDefaults(repo.DB()); err != nil {
		t.Fatalf("SeedDefaults second run: %v", err)
	}
	assertRoleHasPermissions(t, repo.DB(), "owner", []string{"billing.read", "billing.write", "oauth.read", "oauth.write", "logs.read", "platform.admin"}, nil)
	assertRoleHasPermissions(t, repo.DB(), "admin", []string{"billing.read", "billing.write", "oauth.read", "oauth.write", "logs.read"}, []string{"platform.admin"})
	assertRoleHasPermissions(t, repo.DB(), "developer", []string{"org.read", "org.switch", "team.read", "org.usage.read"}, []string{"billing.read", "billing.write", "oauth.read", "oauth.write", "platform.admin"})
	assertRoleHasPermissions(t, repo.DB(), "viewer", []string{"org.read", "org.switch", "team.read"}, []string{"billing.read", "billing.write", "logs.read", "oauth.read", "oauth.write", "platform.admin"})
}

func assertRoleHasPermissions(t *testing.T, db *gorm.DB, roleID string, expected, forbidden []string) {
	t.Helper()
	var rows []models.RolePermission
	if err := db.Where("role_id = ?", roleID).Find(&rows).Error; err != nil {
		t.Fatalf("load role permissions for %s: %v", roleID, err)
	}
	actual := map[string]bool{}
	for _, row := range rows {
		actual[row.PermissionID] = true
	}
	for _, permissionID := range expected {
		if !actual[permissionID] {
			t.Fatalf("role %s missing expected permission %s; actual=%v", roleID, permissionID, actual)
		}
	}
	for _, permissionID := range forbidden {
		if actual[permissionID] {
			t.Fatalf("role %s has forbidden permission %s; actual=%v", roleID, permissionID, actual)
		}
	}
}

func TestHandlerMePermissionsAndInternalMembershipAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newAccessTestService(t)
	if err := SeedDefaults(repo.DB()); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	org := &models.Organization{ID: "org-1", Name: "Org 1", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	user := &models.User{ID: "user-1", Email: "u@example.com", FullName: "User", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	member := &models.OrganizationMember{ID: "member-1", OrganizationID: org.ID, UserID: user.ID, Role: "owner", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = repo.DB().Create(org).Error
	_ = repo.DB().Create(user).Error
	_ = repo.DB().Create(member).Error
	handler := NewHandler(service)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/access/me", nil)
	c.Request = req
	c.Set("orgRole", "owner")
	handler.MePermissions(c)
	if w.Code != 200 {
		t.Fatalf("MePermissions status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest("GET", "/internal/access", bytes.NewReader(nil))
	c.Request = req
	c.Params = gin.Params{{Key: "userID", Value: user.ID}, {Key: "orgID", Value: org.ID}}
	handler.InternalMembershipAccess(c)
	if w.Code != 200 {
		t.Fatalf("InternalMembershipAccess status=%d body=%s", w.Code, w.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
}

// ---------------------------------------------------------------------------
// Helper functions for handler tests
// ---------------------------------------------------------------------------

func performAccessJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performAccessRaw(t, fn, method, path, payload, params)
}

func performAccessQuery(t *testing.T, fn func(*gin.Context), path string) *httptest.ResponseRecorder {
	t.Helper()
	return performAccessRaw(t, fn, http.MethodGet, path, nil, nil)
}

func performAccessRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	c.Params = params
	fn(c)
	return w
}

// ---------------------------------------------------------------------------
// TestNormalizePage
// ---------------------------------------------------------------------------

func TestNormalizePage(t *testing.T) {
	tests := []struct {
		name           string
		limitIn        int
		offsetIn       int
		expectedLimit  int
		expectedOffset int
	}{
		{"zero limit defaults to 20", 0, 0, 20, 0},
		{"negative limit defaults to 20", -5, 0, 20, 0},
		{"over 1000 capped", 1500, 0, 1000, 0},
		{"negative offset defaults to 0", 10, -1, 10, 0},
		{"normal values unchanged", 50, 10, 50, 10},
		{"limit exactly 1000", 1000, 5, 1000, 5},
		{"limit exactly 1", 1, 0, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, o := normalizePage(tt.limitIn, tt.offsetIn)
			if l != tt.expectedLimit || o != tt.expectedOffset {
				t.Fatalf("normalizePage(%d,%d) = (%d,%d), want (%d,%d)", tt.limitIn, tt.offsetIn, l, o, tt.expectedLimit, tt.expectedOffset)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestPermissionCRUD
// ---------------------------------------------------------------------------

func TestPermissionCRUD(t *testing.T) {
	service, _ := newAccessTestService(t)

	// CreatePermission: missing id
	if _, err := service.CreatePermission(UpsertPermissionInput{Name: "Test"}); err == nil {
		t.Fatalf("expected error when id is empty")
	}
	// CreatePermission: missing name
	if _, err := service.CreatePermission(UpsertPermissionInput{ID: "test.perm"}); err == nil {
		t.Fatalf("expected error when name is empty")
	}
	// CreatePermission: both empty
	if _, err := service.CreatePermission(UpsertPermissionInput{}); err == nil {
		t.Fatalf("expected error when both id and name are empty")
	}
	// CreatePermission: success with TrimSpace
	p1, err := service.CreatePermission(UpsertPermissionInput{
		ID: "  perm.one  ", Category: " cat1 ", Name: " Perm One ", Description: " desc1 ",
	})
	if err != nil {
		t.Fatalf("CreatePermission: %v", err)
	}
	if p1.ID != "perm.one" || p1.Name != "Perm One" || p1.Category != "cat1" || p1.Description != "desc1" {
		t.Fatalf("CreatePermission TrimSpace failed: %+v", p1)
	}
	// Create a second permission
	p2, err := service.CreatePermission(UpsertPermissionInput{
		ID: "perm.two", Category: "cat2", Name: "Perm Two", Description: "desc2",
	})
	if err != nil {
		t.Fatalf("CreatePermission p2: %v", err)
	}
	// Create a third for keyword filter
	_, err = service.CreatePermission(UpsertPermissionInput{
		ID: "special.three", Category: "special", Name: "Special Three", Description: "unique",
	})
	if err != nil {
		t.Fatalf("CreatePermission p3: %v", err)
	}

	// ListPermissions: no keyword returns all
	result, err := service.ListPermissions(ListPermissionsInput{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("ListPermissions total = %d, want 3", result.Total)
	}

	// ListPermissions: with keyword filters
	result, err = service.ListPermissions(ListPermissionsInput{Query: "special", Limit: 50})
	if err != nil {
		t.Fatalf("ListPermissions keyword: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("ListPermissions keyword total = %d, want 1", result.Total)
	}

	// ListPermissions: pagination
	result, err = service.ListPermissions(ListPermissionsInput{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("ListPermissions pagination: %v", err)
	}
	if len(result.Items) != 1 || result.Total != 3 {
		t.Fatalf("ListPermissions pagination items=%d total=%d", len(result.Items), result.Total)
	}

	// ListPermissions: default pagination (limit 0)
	result, err = service.ListPermissions(ListPermissionsInput{Limit: 0})
	if err != nil {
		t.Fatalf("ListPermissions default limit: %v", err)
	}
	if result.Limit != 20 {
		t.Fatalf("ListPermissions default limit = %d, want 20", result.Limit)
	}

	// UpdatePermission: partial update
	updated, err := service.UpdatePermission(p2.ID, UpsertPermissionInput{Name: "Updated Two"})
	if err != nil {
		t.Fatalf("UpdatePermission: %v", err)
	}
	if updated.Name != "Updated Two" {
		t.Fatalf("UpdatePermission name = %s, want Updated Two", updated.Name)
	}
	if updated.Category != "cat2" {
		t.Fatalf("UpdatePermission category should be unchanged: %s", updated.Category)
	}

	// UpdatePermission: not found
	if _, err := service.UpdatePermission("nonexistent", UpsertPermissionInput{Name: "X"}); err == nil {
		t.Fatalf("expected UpdatePermission not-found error")
	}

	// DeletePermission: first create a role permission to test cascade
	_, err = service.CreateRole(UpsertRoleInput{ID: "tmp-role", Name: "Tmp"})
	if err != nil {
		t.Fatalf("CreateRole for delete test: %v", err)
	}
	if err := service.SetRolePermissions("tmp-role", []string{p1.ID}); err != nil {
		t.Fatalf("SetRolePermissions for delete test: %v", err)
	}
	// Verify the role_permission exists
	ids, _ := service.ListRolePermissions("tmp-role")
	if len(ids) != 1 {
		t.Fatalf("expected 1 role_permission before delete, got %d", len(ids))
	}
	// Delete the permission
	if err := service.DeletePermission(p1.ID); err != nil {
		t.Fatalf("DeletePermission: %v", err)
	}
	// Verify cascade: role_permission should be removed
	ids, _ = service.ListRolePermissions("tmp-role")
	if len(ids) != 0 {
		t.Fatalf("expected 0 role_permissions after cascade delete, got %d", len(ids))
	}
	// Verify permission is gone
	result, _ = service.ListPermissions(ListPermissionsInput{Query: "perm.one"})
	if result.Total != 0 {
		t.Fatalf("expected permission perm.one to be deleted")
	}
}

// ---------------------------------------------------------------------------
// TestRoleCRUD
// ---------------------------------------------------------------------------

func TestRoleCRUD(t *testing.T) {
	service, repo := newAccessTestService(t)

	// CreateRole: missing id
	if _, err := service.CreateRole(UpsertRoleInput{Name: "Test"}); err == nil {
		t.Fatalf("expected error when role id is empty")
	}
	// CreateRole: missing name
	if _, err := service.CreateRole(UpsertRoleInput{ID: "test-role"}); err == nil {
		t.Fatalf("expected error when role name is empty")
	}
	// CreateRole: success
	r1, err := service.CreateRole(UpsertRoleInput{ID: "role-alpha", Name: "Alpha", Description: "Alpha role"})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if r1.ID != "role-alpha" || r1.Name != "Alpha" {
		t.Fatalf("CreateRole unexpected: %+v", r1)
	}
	// Create second role
	r2, err := service.CreateRole(UpsertRoleInput{ID: "role-beta", Name: "Beta", Description: "Beta role"})
	if err != nil {
		t.Fatalf("CreateRole r2: %v", err)
	}
	// Create third role with keyword
	_, err = service.CreateRole(UpsertRoleInput{ID: "role-special", Name: "Special", Description: "For search"})
	if err != nil {
		t.Fatalf("CreateRole r3: %v", err)
	}

	// ListRoles: no keyword
	result, err := service.ListRoles(ListRolesInput{Limit: 50})
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("ListRoles total = %d, want 3", result.Total)
	}

	// ListRoles: with keyword
	result, err = service.ListRoles(ListRolesInput{Query: "Special", Limit: 50})
	if err != nil {
		t.Fatalf("ListRoles keyword: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("ListRoles keyword total = %d, want 1", result.Total)
	}

	// UpdateRole: partial update + is_system flag
	updated, err := service.UpdateRole(r1.ID, UpsertRoleInput{Name: "Alpha Updated", IsSystem: true})
	if err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}
	if updated.Name != "Alpha Updated" || !updated.IsSystem {
		t.Fatalf("UpdateRole unexpected: name=%s is_system=%v", updated.Name, updated.IsSystem)
	}

	// UpdateRole: not found
	if _, err := service.UpdateRole("nonexistent", UpsertRoleInput{Name: "X"}); err == nil {
		t.Fatalf("expected UpdateRole not-found error")
	}

	// DeleteRole: cascade test
	// Setup: create permission, assign to r2, create user + member with role=r2
	now := time.Now()
	_, _ = service.CreatePermission(UpsertPermissionInput{ID: "del.perm", Name: "Del Perm"})
	_ = service.SetRolePermissions(r2.ID, []string{"del.perm"})

	org := &models.Organization{ID: "org-del", Name: "Org Del", Status: "active", CreatedAt: now, UpdatedAt: now}
	user := &models.User{ID: "user-del", Email: "del@example.com", FullName: "Del User", Status: "active", OrgRole: r2.ID, CreatedAt: now, UpdatedAt: now}
	member := &models.OrganizationMember{ID: "mem-del", OrganizationID: org.ID, UserID: user.ID, Role: r2.ID, Status: "active", CreatedAt: now, UpdatedAt: now}
	repo.DB().Create(org)
	repo.DB().Create(user)
	repo.DB().Create(member)

	if err := service.DeleteRole(r2.ID); err != nil {
		t.Fatalf("DeleteRole: %v", err)
	}
	// role permissions should be cleared
	ids, _ := service.ListRolePermissions(r2.ID)
	if len(ids) != 0 {
		t.Fatalf("expected 0 role_permissions after role delete, got %d", len(ids))
	}
	// user.org_role should be reset to viewer
	var u models.User
	repo.DB().Where("id = ?", user.ID).First(&u)
	if u.OrgRole != "viewer" {
		t.Fatalf("user org_role = %s, want viewer", u.OrgRole)
	}
	// member.role should be reset to viewer
	var m models.OrganizationMember
	repo.DB().Where("id = ?", member.ID).First(&m)
	if m.Role != "viewer" {
		t.Fatalf("member role = %s, want viewer", m.Role)
	}
	// role itself should be deleted
	result, _ = service.ListRoles(ListRolesInput{Query: r2.ID})
	if result.Total != 0 {
		t.Fatalf("expected role %s to be deleted", r2.ID)
	}
}

// ---------------------------------------------------------------------------
// TestSetRolePermissionsAndRebuild
// ---------------------------------------------------------------------------

func TestSetRolePermissionsAndRebuild(t *testing.T) {
	service, repo := newAccessTestService(t)

	// Seed a role and some permissions
	_, err := service.CreateRole(UpsertRoleInput{ID: "editor", Name: "Editor"})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	_, _ = service.CreatePermission(UpsertPermissionInput{ID: "p.read", Name: "Read"})
	_, _ = service.CreatePermission(UpsertPermissionInput{ID: "p.write", Name: "Write"})
	_, _ = service.CreatePermission(UpsertPermissionInput{ID: "p.delete", Name: "Delete"})

	// SetRolePermissions: normal
	if err := service.SetRolePermissions("editor", []string{"p.read", "p.write"}); err != nil {
		t.Fatalf("SetRolePermissions: %v", err)
	}
	ids, err := service.ListRolePermissions("editor")
	if err != nil {
		t.Fatalf("ListRolePermissions: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(ids))
	}

	// SetRolePermissions: whitespace ids skipped
	if err := service.SetRolePermissions("editor", []string{"p.delete", "  ", ""}); err != nil {
		t.Fatalf("SetRolePermissions with whitespace: %v", err)
	}
	ids, _ = service.ListRolePermissions("editor")
	if len(ids) != 1 {
		t.Fatalf("expected 1 permission after whitespace skip, got %d", len(ids))
	}
	if ids[0] != "p.delete" {
		t.Fatalf("expected p.delete, got %s", ids[0])
	}

	// SetRolePermissions: empty list clears all
	if err := service.SetRolePermissions("editor", []string{}); err != nil {
		t.Fatalf("SetRolePermissions empty: %v", err)
	}
	ids, _ = service.ListRolePermissions("editor")
	if len(ids) != 0 {
		t.Fatalf("expected 0 permissions after clear, got %d", len(ids))
	}

	// RebuildUserPermissions: set permissions back and test rebuild
	_ = service.SetRolePermissions("editor", []string{"p.read", "p.write"})
	now := time.Now()
	org := &models.Organization{ID: "org-rb", Name: "Org RB", Status: "active", CreatedAt: now, UpdatedAt: now}
	user := &models.User{ID: "user-rb", Email: "rb@example.com", FullName: "RB User", Status: "active", CreatedAt: now, UpdatedAt: now}
	member := &models.OrganizationMember{ID: "mem-rb", OrganizationID: org.ID, UserID: user.ID, Role: "editor", Status: "active", CreatedAt: now, UpdatedAt: now}
	repo.DB().Create(org)
	repo.DB().Create(user)
	repo.DB().Create(member)

	perms, err := service.RebuildUserPermissions(user.ID, org.ID, "editor")
	if err != nil {
		t.Fatalf("RebuildUserPermissions: %v", err)
	}
	if len(perms) != 2 {
		t.Fatalf("RebuildUserPermissions got %d perms, want 2", len(perms))
	}
}

// ---------------------------------------------------------------------------
// TestAccessHandlerCRUD
// ---------------------------------------------------------------------------

func TestAccessHandlerCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newAccessTestService(t)
	handler := NewHandler(service)

	// --- Permission CRUD via handlers ---

	// CreatePermission handler: success
	resp := performAccessJSON(t, handler.CreatePermission, http.MethodPost, "/permissions",
		UpsertPermissionInput{ID: "h.perm1", Category: "cat", Name: "HPerm1", Description: "d"},
		nil,
	)
	if resp.Code != http.StatusCreated {
		t.Fatalf("CreatePermission handler status=%d body=%s", resp.Code, resp.Body.String())
	}

	// CreatePermission handler: second permission
	resp = performAccessJSON(t, handler.CreatePermission, http.MethodPost, "/permissions",
		UpsertPermissionInput{ID: "h.perm2", Category: "cat", Name: "HPerm2", Description: "d2"},
		nil,
	)
	if resp.Code != http.StatusCreated {
		t.Fatalf("CreatePermission handler 2 status=%d body=%s", resp.Code, resp.Body.String())
	}

	// CreatePermission handler: bind error (bad JSON)
	resp = performAccessRaw(t, handler.CreatePermission, http.MethodPost, "/permissions", []byte("{bad"), nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected bind error for bad JSON")
	}

	// CreatePermission handler: validation error (missing fields)
	resp = performAccessJSON(t, handler.CreatePermission, http.MethodPost, "/permissions",
		UpsertPermissionInput{ID: "", Name: ""},
		nil,
	)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected validation error for empty id/name")
	}

	// ListPermissions handler
	resp = performAccessQuery(t, handler.ListPermissions, "/permissions?limit=50&offset=0")
	if resp.Code != http.StatusOK {
		t.Fatalf("ListPermissions handler status=%d body=%s", resp.Code, resp.Body.String())
	}

	// ListPermissions handler: with query
	resp = performAccessQuery(t, handler.ListPermissions, "/permissions?query=HPerm1")
	if resp.Code != http.StatusOK {
		t.Fatalf("ListPermissions handler query status=%d body=%s", resp.Code, resp.Body.String())
	}

	// UpdatePermission handler: success
	resp = performAccessJSON(t, handler.UpdatePermission, http.MethodPut, "/permissions/h.perm1",
		UpsertPermissionInput{Name: "HPerm1 Updated"},
		gin.Params{{Key: "permissionID", Value: "h.perm1"}},
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("UpdatePermission handler status=%d body=%s", resp.Code, resp.Body.String())
	}

	// UpdatePermission handler: bind error
	resp = performAccessRaw(t, handler.UpdatePermission, http.MethodPut, "/permissions/h.perm1", []byte("{bad"), gin.Params{{Key: "permissionID", Value: "h.perm1"}})
	if resp.Code == http.StatusOK {
		t.Fatalf("expected bind error for UpdatePermission")
	}

	// DeletePermission handler: success
	resp = performAccessRaw(t, handler.DeletePermission, http.MethodDelete, "/permissions/h.perm2", nil,
		gin.Params{{Key: "permissionID", Value: "h.perm2"}},
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("DeletePermission handler status=%d body=%s", resp.Code, resp.Body.String())
	}

	// --- Role CRUD via handlers ---

	// CreateRole handler: success
	resp = performAccessJSON(t, handler.CreateRole, http.MethodPost, "/roles",
		UpsertRoleInput{ID: "h.role1", Name: "HRole1", Description: "dr1"},
		nil,
	)
	if resp.Code != http.StatusCreated {
		t.Fatalf("CreateRole handler status=%d body=%s", resp.Code, resp.Body.String())
	}

	// CreateRole handler: second
	resp = performAccessJSON(t, handler.CreateRole, http.MethodPost, "/roles",
		UpsertRoleInput{ID: "h.role2", Name: "HRole2"},
		nil,
	)
	if resp.Code != http.StatusCreated {
		t.Fatalf("CreateRole handler 2 status=%d body=%s", resp.Code, resp.Body.String())
	}

	// CreateRole handler: bind error
	resp = performAccessRaw(t, handler.CreateRole, http.MethodPost, "/roles", []byte("{bad"), nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected bind error for CreateRole bad JSON")
	}

	// CreateRole handler: validation error
	resp = performAccessJSON(t, handler.CreateRole, http.MethodPost, "/roles",
		UpsertRoleInput{},
		nil,
	)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected validation error for empty role")
	}

	// ListRoles handler
	resp = performAccessQuery(t, handler.ListRoles, "/roles?limit=50&offset=0")
	if resp.Code != http.StatusOK {
		t.Fatalf("ListRoles handler status=%d body=%s", resp.Code, resp.Body.String())
	}

	// ListRoles handler: with keyword
	resp = performAccessQuery(t, handler.ListRoles, "/roles?query=HRole1")
	if resp.Code != http.StatusOK {
		t.Fatalf("ListRoles handler query status=%d body=%s", resp.Code, resp.Body.String())
	}

	// UpdateRole handler: success
	resp = performAccessJSON(t, handler.UpdateRole, http.MethodPut, "/roles/h.role1",
		UpsertRoleInput{Name: "HRole1 Updated", IsSystem: true},
		gin.Params{{Key: "roleID", Value: "h.role1"}},
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("UpdateRole handler status=%d body=%s", resp.Code, resp.Body.String())
	}

	// UpdateRole handler: bind error
	resp = performAccessRaw(t, handler.UpdateRole, http.MethodPut, "/roles/h.role1", []byte("{bad"), gin.Params{{Key: "roleID", Value: "h.role1"}})
	if resp.Code == http.StatusOK {
		t.Fatalf("expected bind error for UpdateRole")
	}

	// --- RolePermissions via handlers ---

	// SetRolePermissions handler: success
	resp = performAccessJSON(t, handler.SetRolePermissions, http.MethodPut, "/roles/h.role1/permissions",
		struct {
			PermissionIDs []string `json:"permission_ids"`
		}{PermissionIDs: []string{"h.perm1"}},
		gin.Params{{Key: "roleID", Value: "h.role1"}},
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("SetRolePermissions handler status=%d body=%s", resp.Code, resp.Body.String())
	}

	// SetRolePermissions handler: bind error
	resp = performAccessRaw(t, handler.SetRolePermissions, http.MethodPut, "/roles/h.role1/permissions", []byte("{bad"), gin.Params{{Key: "roleID", Value: "h.role1"}})
	if resp.Code == http.StatusOK {
		t.Fatalf("expected bind error for SetRolePermissions")
	}

	// GetRolePermissions handler
	resp = performAccessRaw(t, handler.GetRolePermissions, http.MethodGet, "/roles/h.role1/permissions", nil,
		gin.Params{{Key: "roleID", Value: "h.role1"}},
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("GetRolePermissions handler status=%d body=%s", resp.Code, resp.Body.String())
	}

	// DeleteRole handler: setup user and member to test cascade
	now := time.Now()
	org := &models.Organization{ID: "org-h", Name: "Org H", Status: "active", CreatedAt: now, UpdatedAt: now}
	user := &models.User{ID: "user-h", Email: "h@example.com", FullName: "H User", Status: "active", OrgRole: "h.role2", CreatedAt: now, UpdatedAt: now}
	member := &models.OrganizationMember{ID: "mem-h", OrganizationID: org.ID, UserID: user.ID, Role: "h.role2", Status: "active", CreatedAt: now, UpdatedAt: now}
	repo.DB().Create(org)
	repo.DB().Create(user)
	repo.DB().Create(member)

	resp = performAccessRaw(t, handler.DeleteRole, http.MethodDelete, "/roles/h.role2", nil,
		gin.Params{{Key: "roleID", Value: "h.role2"}},
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("DeleteRole handler status=%d body=%s", resp.Code, resp.Body.String())
	}

	// Verify user role was reset
	var u models.User
	repo.DB().Where("id = ?", user.ID).First(&u)
	if u.OrgRole != "viewer" {
		t.Fatalf("handler DeleteRole: user org_role = %s, want viewer", u.OrgRole)
	}
}
