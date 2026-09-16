package core

import (
	"log"
	"os"

	"apeadmin-gin/internal/config"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/utils"

	"gorm.io/gorm"
)

// SeedData 首次启动时初始化种子数据
func SeedData(db *gorm.DB, saCfg config.SuperAdminConfig) {
	seedDept(db)
	seedMenus(db)
	// 菜单布局迁移：插件管理提升为顶级菜单（已有库兼容）
	migrateMenuLayout(db)
	seedRoles(db)
	seedSuperAdmin(db, saCfg)
	seedSettings(db)
}

// migrateMenuLayout 幂等菜单布局迁移：
// 将"插件管理"从系统管理目录（ParentID=1）提升为顶级菜单（ParentID=0），
// Sort=1 置于仪表盘（Sort=0）正下方；系统管理/MCP/AI 顶级目录 Sort 顺延。
// 仅当检测到旧布局时才执行，重复启动不重复处理。
func migrateMenuLayout(db *gorm.DB) {
	var pluginMenu model.SysMenu
	// 定位插件管理菜单（path=plugin 且类型为 C 菜单）
	if err := db.Where("name = ? AND path = ? AND type = ?", "插件管理", "plugin", "C").First(&pluginMenu).Error; err != nil {
		return // 不存在则跳过（尚未初始化或已删除）
	}

	changed := false
	// ① 插件管理：若仍在系统管理目录下则提升为顶级，Sort=1
	if pluginMenu.ParentID != 0 || pluginMenu.Sort != 1 {
		db.Model(&model.SysMenu{}).Where("id = ?", pluginMenu.ID).Updates(map[string]interface{}{
			"parent_id": 0,
			"sort":      1,
		})
		changed = true
	}

	// ② 顶级目录 Sort 顺延：系统管理 1→2、MCP 管理 2→3、AI 对话 3→4（避免与插件管理 Sort=1 冲突）
	type topLevel struct {
		name string
		sort int
	}
	for _, t := range []topLevel{{name: "系统管理", sort: 2}, {name: "MCP 管理", sort: 3}, {name: "AI 对话", sort: 4}} {
		var m model.SysMenu
		if err := db.Where("name = ? AND parent_id = 0 AND type = ?", t.name, "M").First(&m).Error; err == nil && m.Sort != t.sort {
			db.Model(&model.SysMenu{}).Where("id = ?", m.ID).Update("sort", t.sort)
			changed = true
		}
	}

	if changed {
		log.Println("菜单迁移：插件管理已提升为顶级菜单（位于仪表盘下方）")
	}
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
		{Name: "系统管理", ParentID: 0, Type: "M", Path: "/system", Icon: "Setting", Sort: 2, Visible: 1, Status: 1},
		{Name: "用户管理", ParentID: 1, Type: "C", Path: "user", Component: "system/user", Permission: "system:user:list", Icon: "User", Sort: 1, Visible: 1, Status: 1},
		{Name: "角色管理", ParentID: 1, Type: "C", Path: "role", Component: "system/role", Permission: "system:role:list", Icon: "UserFilled", Sort: 2, Visible: 1, Status: 1},
		{Name: "菜单管理", ParentID: 1, Type: "C", Path: "menu", Component: "system/menu", Permission: "system:menu:list", Icon: "Menu", Sort: 3, Visible: 1, Status: 1},
		{Name: "部门管理", ParentID: 1, Type: "C", Path: "dept", Component: "system/dept", Permission: "system:dept:list", Icon: "OfficeBuilding", Sort: 4, Visible: 1, Status: 1},
		// 插件管理：顶级菜单（ParentID=0），Sort=1 置于仪表盘正下方。
		// 注意：保持本行在创建顺序中的位置不变（ID=6），仅改 ParentID/Sort，避免按钮权限 ParentID 引用漂移。
		{Name: "插件管理", ParentID: 0, Type: "C", Path: "plugin", Component: "system/plugin", Permission: "system:plugin:list", Icon: "Box", Sort: 1, Visible: 1, Status: 1},
		{Name: "日志管理", ParentID: 1, Type: "C", Path: "log", Component: "system/log", Permission: "system:log:list", Icon: "Document", Sort: 5, Visible: 1, Status: 1},
		{Name: "系统设置", ParentID: 1, Type: "C", Path: "settings", Component: "system/settings", Permission: "system:setting:list", Icon: "Tools", Sort: 6, Visible: 1, Status: 1},
		// MCP 管理
		{Name: "MCP 管理", ParentID: 0, Type: "M", Path: "/mcp", Icon: "Connection", Sort: 3, Visible: 1, Status: 1},
		{Name: "工具管理", ParentID: 9, Type: "C", Path: "tools", Component: "mcp/tools", Permission: "mcp:tools:list", Icon: "Tools", Sort: 1, Visible: 1, Status: 1},
		{Name: "资源管理", ParentID: 9, Type: "C", Path: "resources", Component: "mcp/resources", Permission: "mcp:resources:list", Icon: "FolderOpened", Sort: 2, Visible: 1, Status: 1},
		{Name: "提示词管理", ParentID: 9, Type: "C", Path: "prompts", Component: "mcp/prompts", Permission: "mcp:prompts:list", Icon: "ChatLineSquare", Sort: 3, Visible: 1, Status: 1},
		{Name: "审计日志", ParentID: 9, Type: "C", Path: "audit-logs", Component: "mcp/audit-logs", Permission: "mcp:audit:list", Icon: "Document", Sort: 4, Visible: 1, Status: 1},
		// AI 对话
		{Name: "AI 对话", ParentID: 0, Type: "M", Path: "/ai", Icon: "ChatDotRound", Sort: 4, Visible: 1, Status: 1},
		{Name: "对话助手", ParentID: 14, Type: "C", Path: "chat", Component: "ai/chat", Permission: "ai:chat", Icon: "ChatLineRound", Sort: 1, Visible: 1, Status: 1},
		{Name: "模型密钥管理", ParentID: 14, Type: "C", Path: "providers", Component: "ai/providers", Permission: "ai:provider:list", Icon: "Key", Sort: 2, Visible: 1, Status: 1},
		// 仪表盘（前端硬编码跳转 /dashboard-monitor，component 对应 apeui/dashboard/Monitor.vue）
		// 最后创建（ID=17），靠 Sort=0 排到最前
		{Name: "仪表盘", ParentID: 0, Type: "C", Path: "dashboard-monitor", Component: "apeui/dashboard/Monitor", Permission: "dashboard:view", Icon: "Odometer", Sort: 0, Visible: 1, Status: 1},
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
	db.Where("path = ?", "dashboard-monitor").Find(&viewerMenus)
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
		// 已有库：补齐缺失的品牌类设置项（幂等）
		migrateMissingSettings(db)
		return
	}

	settings := []model.SysSetting{
		{Key: "site_name", Value: "ApeAdmin-Gin", IsPublic: true},
		{Key: "logo_url", Value: "", IsPublic: true},
		{Key: "primary_color", Value: "#5A67F5", IsPublic: true},
		{Key: "admin_path", Value: "/admin", IsPublic: true},
		{Key: "footer_text", Value: "ApeAdmin © 2026", IsPublic: true},
		{Key: "login_bg", Value: "", IsPublic: true},
		{Key: "sidebar_theme", Value: "light", IsPublic: true},
	}
	for _, s := range settings {
		db.Create(&s)
	}
	log.Println("种子数据：系统设置已初始化")
}

// migrateMissingSettings 补齐缺失的品牌类设置项（不覆盖已有值）
func migrateMissingSettings(db *gorm.DB) {
	defaults := []model.SysSetting{
		{Key: "admin_path", Value: "/admin", IsPublic: true},
		{Key: "footer_text", Value: "ApeAdmin © 2026", IsPublic: true},
		{Key: "login_bg", Value: "", IsPublic: true},
		{Key: "sidebar_theme", Value: "light", IsPublic: true},
	}
	for _, d := range defaults {
		var count int64
		db.Model(&model.SysSetting{}).Where("key = ?", d.Key).Count(&count)
		if count == 0 {
			db.Create(&d)
			log.Printf("种子数据：补充设置项 %s", d.Key)
		}
	}
}

// SeedAiProvider 初始化默认 AI 供应商（DeepSeek）
func SeedAiProvider(db *gorm.DB) {
	var count int64
	db.Model(&model.SysAiProvider{}).Count(&count)
	if count > 0 {
		return
	}

	cfg := GetConfig()
	if cfg == nil {
		log.Println("种子数据：跳过 AI 供应商（配置未初始化）")
		return
	}

	// 从环境变量读取 DeepSeek API Key，未配置则跳过
	apiKey := os.Getenv("GA_AI_DEEPSEEK_KEY")
	if apiKey == "" {
		log.Println("种子数据：未设置 GA_AI_DEEPSEEK_KEY，跳过默认 DeepSeek 供应商")
		return
	}

	enc, err := utils.EncryptSecret(cfg.JWT.Secret, apiKey)
	if err != nil {
		log.Printf("种子数据：DeepSeek API Key 加密失败: %v", err)
		return
	}

	modelsJSON := `["deepseek-chat","deepseek-reasoner"]`
	provider := model.SysAiProvider{
		Name:         "DeepSeek",
		ProviderType: "deepseek",
		BaseURL:      "https://api.deepseek.com",
		Models:       &modelsJSON,
		ApiKeyEnc:    enc,
		Sort:         1,
		Remark:       "默认 DeepSeek 供应商",
		Enabled:      true,
	}
	if err := db.Create(&provider).Error; err != nil {
		log.Printf("种子数据：创建默认 DeepSeek 供应商失败: %v", err)
		return
	}
	log.Println("种子数据：默认 DeepSeek 供应商已创建")
}
