package service

import (
	"testing"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPermTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.SysUser{}, &model.SysRole{},
		&model.SysMenu{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	core.SetDB(db)
	dal.Init()
	t.Cleanup(func() { core.ResetTest(); InvalidateAllPermCache() })
}

func seedPermFixture(t *testing.T) (userID uint) {
	t.Helper()
	db := core.GetDB()

	user := model.SysUser{Username: "alice", Password: "x", Status: 1}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	role := model.SysRole{Name: "editor", Code: "editor", Status: 1}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	db.Exec("INSERT INTO sys_user_role (sys_user_id, sys_role_id) VALUES (?, ?)", user.ID, role.ID)
	menu := model.SysMenu{Name: "用户管理", Permission: "system:user:list", Status: 1}
	if err := db.Create(&menu).Error; err != nil {
		t.Fatal(err)
	}
	db.Exec("INSERT INTO sys_role_menu (sys_role_id, sys_menu_id) VALUES (?, ?)", role.ID, menu.ID)
	return user.ID
}

func TestGetUserPermissionsFromRoles(t *testing.T) {
	setupPermTestDB(t)
	uid := seedPermFixture(t)

	perms := GetUserPermissions(uid)
	if !perms["system:user:list"] {
		t.Fatalf("perms = %v, want system:user:list", perms)
	}
	if perms["system:role:add"] {
		t.Fatal("unrelated permission must be absent")
	}
}

func TestGetUserPermissionsDisabledRoleIgnored(t *testing.T) {
	setupPermTestDB(t)
	uid := seedPermFixture(t)
	core.GetDB().Model(&model.SysRole{}).Where("code = ?", "editor").Update("status", 0)

	if perms := GetUserPermissions(uid); len(perms) != 0 {
		t.Fatalf("disabled role should grant nothing, got %v", perms)
	}
}

func TestGetUserPermissionsCachedAndInvalidate(t *testing.T) {
	setupPermTestDB(t)
	uid := seedPermFixture(t)

	p1 := GetUserPermissionsCached(uid)
	if !p1["system:user:list"] {
		t.Fatalf("cached perms = %v", p1)
	}
	// 直接改库（绕过失效钩子）：缓存期内仍读到旧值
	core.GetDB().Exec("DELETE FROM sys_role_menu")
	p2 := GetUserPermissionsCached(uid)
	if !p2["system:user:list"] {
		t.Fatal("within TTL cache must serve old value")
	}
	// 失效后重新计算 → 权限消失
	InvalidateUserPermCache(uid)
	p3 := GetUserPermissionsCached(uid)
	if len(p3) != 0 {
		t.Fatalf("after invalidation perms = %v, want empty", p3)
	}
}

func TestInvalidateAllPermCache(t *testing.T) {
	setupPermTestDB(t)
	uid := seedPermFixture(t)

	_ = GetUserPermissionsCached(uid)
	InvalidateAllPermCache()
	permMu.RLock()
	n := len(permCache)
	permMu.RUnlock()
	if n != 0 {
		t.Fatalf("cache size after invalidate-all = %d", n)
	}
}
