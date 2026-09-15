package core

import (
	"log"

	"gin-apeadmin/internal/config"
	"gin-apeadmin/internal/model"
	"gin-apeadmin/internal/pkg/utils"

	"gorm.io/gorm"
)

// SeedData 首次启动时初始化种子数据
func SeedData(db *gorm.DB, saCfg config.SuperAdminConfig) {
	seedDept(db)
	seedMenus(db)
	seedRoles(db)
	seedSuperAdmin(db, saCfg)
	seedSettings(db)
}

func seedDept(db *gorm.DB) {
	var count int64
	db.Model(&model.SysDept{}).Count(&count)
	if count > 0 {
		return
	}
	db.Create(&model.SysDept{Name: "总经办", Sort: 1, Status: 1})
}

func seedMenus(db *gorm.DB) {
	var count int64
	db.Model(&model.SysMenu{}).Count(&count)
	if count > 0 {
		return
	}

	menus := []model.SysMenu{
		// 系统管理目录
		{Name: "系统管理", ParentID: 0, Type: "M", Path: "/system", Icon: "Setting", Sort: 1, Visible: 1, Status: 1},
		{Name: "用户管理", ParentID: 1, Type: "C", Path: "user", Component: "system/user", Permission: "system:user:list", Icon: "User", Sort: 1, Visible: 1, Status: 1},
		{Name: "角色管理", ParentID: 1, Type: "C", Path: "role", Component: "system/role", Permission: "system:role:list", Icon: "UserFilled", Sort: 2, Visible: 1, Status: 1},
		{Name: "菜单管理", ParentID: 1, Type: "C", Path: "menu", Component: "system/menu", Permission: "system:menu:list", Icon: "Menu", Sort: 3, Visible: 1, Status: 1},
		{Name: "部门管理", ParentID: 1, Type: "C", Path: "dept", Component: "system/dept", Permission: "system:dept:list", Icon: "OfficeBuilding", Sort: 4, Visible: 1, Status: 1},
		{Name: "插件管理", ParentID: 1, Type: "C", Path: "plugin", Component: "system/plugin", Permission: "system:plugin:list", Icon: "Box", Sort: 5, Visible: 1, Status: 1},
		{Name: "日志管理", ParentID: 1, Type: "C", Path: "log", Component: "system/log", Permission: "system:log:list", Icon: "Document", Sort: 6, Visible: 1, Status: 1},
		{Name: "系统设置", ParentID: 1, Type: "C", Path: "setting", Component: "system/setting", Permission: "system:setting:list", Icon: "Tools", Sort: 7, Visible: 1, Status: 1},
		// MCP 管理
		{Name: "MCP 管理", ParentID: 0, Type: "M", Path: "/mcp", Icon: "Connection", Sort: 2, Visible: 1, Status: 1},
		{Name: "工具管理", ParentID: 9, Type: "C", Path: "tool", Component: "mcp/tool", Permission: "mcp:tools:list", Icon: "Tools", Sort: 1, Visible: 1, Status: 1},
		// AI 对话
		{Name: "AI 对话", ParentID: 0, Type: "M", Path: "/ai", Icon: "ChatDotRound", Sort: 3, Visible: 1, Status: 1},
		{Name: "对话助手", ParentID: 11, Type: "C", Path: "chat", Component: "ai/chat", Permission: "ai:chat", Icon: "ChatLineRound", Sort: 1, Visible: 1, Status: 1},
		// 仪表盘
		{Name: "仪表盘", ParentID: 0, Type: "C", Path: "/dashboard", Component: "dashboard/index", Permission: "dashboard:view", Icon: "Odometer", Sort: 0, Visible: 1, Status: 1},
	}

	// 用户管理按钮权限
	menus = append(menus, []model.SysMenu{
		{Name: "用户新增", ParentID: 2, Type: "F", Permission: "system:user:add", Sort: 1, Status: 1},
		{Name: "用户编辑", ParentID: 2, Type: "F", Permission: "system:user:edit", Sort: 2, Status: 1},
		{Name: "用户删除", ParentID: 2, Type: "F", Permission: "system:user:delete", Sort: 3, Status: 1},
		{Name: "重置密码", ParentID: 2, Type: "F", Permission: "system:user:reset-password", Sort: 4, Status: 1},
		{Name: "角色新增", ParentID: 3, Type: "F", Permission: "system:role:add", Sort: 1, Status: 1},
		{Name: "角色编辑", ParentID: 3, Type: "F", Permission: "system:role:edit", Sort: 2, Status: 1},
		{Name: "角色删除", ParentID: 3, Type: "F", Permission: "system:role:delete", Sort: 3, Status: 1},
		{Name: "菜单新增", ParentID: 4, Type: "F", Permission: "system:menu:add", Sort: 1, Status: 1},
		{Name: "菜单编辑", ParentID: 4, Type: "F", Permission: "system:menu:edit", Sort: 2, Status: 1},
		{Name: "菜单删除", ParentID: 4, Type: "F", Permission: "system:menu:delete", Sort: 3, Status: 1},
		{Name: "部门新增", ParentID: 5, Type: "F", Permission: "system:dept:add", Sort: 1, Status: 1},
		{Name: "部门编辑", ParentID: 5, Type: "F", Permission: "system:dept:edit", Sort: 2, Status: 1},
		{Name: "部门删除", ParentID: 5, Type: "F", Permission: "system:dept:delete", Sort: 3, Status: 1},
		{Name: "插件上传", ParentID: 6, Type: "F", Permission: "system:plugin:upload", Sort: 1, Status: 1},
		{Name: "插件启停", ParentID: 6, Type: "F", Permission: "system:plugin:toggle", Sort: 2, Status: 1},
		{Name: "插件配置", ParentID: 6, Type: "F", Permission: "system:plugin:config", Sort: 3, Status: 1},
		{Name: "插件删除", ParentID: 6, Type: "F", Permission: "system:plugin:delete", Sort: 4, Status: 1},
		{Name: "插件重启", ParentID: 6, Type: "F", Permission: "system:plugin:restart", Sort: 5, Status: 1},
		{Name: "日志删除", ParentID: 7, Type: "F", Permission: "system:log:delete", Sort: 1, Status: 1},
		{Name: "设置编辑", ParentID: 8, Type: "F", Permission: "system:setting:edit", Sort: 1, Status: 1},
		{Name: "MCP 调用", ParentID: 10, Type: "F", Permission: "mcp:tools:call", Sort: 1, Status: 1},
	}...)

	for _, m := range menus {
		db.Create(&m)
	}
	log.Println("种子数据：菜单树已初始化")
}

func seedRoles(db *gorm.DB) {
	var count int64
	db.Model(&model.SysRole{}).Count(&count)
	if count > 0 {
		return
	}

	// 超管角色 — 分配所有菜单
	var allMenus []model.SysMenu
	db.Find(&allMenus)
	var menuIDs []uint
	for _, m := range allMenus {
		menuIDs = append(menuIDs, m.ID)
	}

	adminRole := model.SysRole{
		Name:      "超级管理员",
		Code:      "admin",
		DataScope: 4, // 全部
		Sort:      1,
		Status:    1,
		Remark:    "系统超管角色",
	}
	db.Create(&adminRole)
	db.Model(&adminRole).Association("Menus").Replace(&allMenus)

	// 开发者角色
	devMenus := []model.SysMenu{}
	db.Where("path IN ?", []string{"user", "role", "menu", "dept", "plugin", "log"}).Find(&devMenus)
	devRole := model.SysRole{
		Name:      "开发者",
		Code:      "developer",
		DataScope: 2, // 本部门及以下
		Sort:      2,
		Status:    1,
		Remark:    "开发者角色",
	}
	db.Create(&devRole)
	db.Model(&devRole).Association("Menus").Replace(&devMenus)

	// 访客角色
	viewerMenus := []model.SysMenu{}
	db.Where("path = ?", "dashboard/index").Find(&viewerMenus)
	viewerRole := model.SysRole{
		Name:      "访客",
		Code:      "viewer",
		DataScope: 1, // 本人
		Sort:      3,
		Status:    1,
		Remark:    "只读访客",
	}
	db.Create(&viewerRole)
	db.Model(&viewerRole).Association("Menus").Replace(&viewerMenus)

	log.Println("种子数据：角色已初始化")
}

func seedSuperAdmin(db *gorm.DB, saCfg config.SuperAdminConfig) {
	var count int64
	db.Model(&model.SysUser{}).Count(&count)
	if count > 0 {
		return
	}

	hash, err := utils.HashPassword(saCfg.Password)
	if err != nil {
		log.Printf("种子数据：超管密码加密失败: %v", err)
		return
	}

	// 获取超管角色
	var adminRole model.SysRole
	db.Where("code = ?", "admin").First(&adminRole)

	user := model.SysUser{
		Username:     saCfg.Username,
		Nickname:     "超级管理员",
		Password:     hash,
		Status:       1,
		IsSuperAdmin: true,
	}
	if err := db.Create(&user).Error; err != nil {
		log.Printf("种子数据：创建超管失败: %v", err)
		return
	}
	db.Model(&user).Association("Roles").Replace(&adminRole)

	log.Println("种子数据：超级管理员已创建")
}

func seedSettings(db *gorm.DB) {
	var count int64
	db.Model(&model.SysSetting{}).Count(&count)
	if count > 0 {
		return
	}

	settings := []model.SysSetting{
		{Key: "site_name", Value: "GinApeAdmin", IsPublic: true},
		{Key: "logo_url", Value: "", IsPublic: true},
		{Key: "primary_color", Value: "#0E7C7B", IsPublic: true},
	}
	for _, s := range settings {
		db.Create(&s)
	}
	log.Println("种子数据：系统设置已初始化")
}
