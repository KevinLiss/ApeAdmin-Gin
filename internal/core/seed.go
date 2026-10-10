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
	// 菜单 i18n_key 回填（已有库幂等补充；新库 seedMenus 已写入）
	migrateMenuI18nKeys(db)
	seedRoles(db)
	seedSuperAdmin(db, saCfg)
	seedSettings(db)
	seedLocales(db)
	seedI18nMenus(db)
}

// menuI18nKeys 菜单名 → i18n key 映射（与前端 locales/menu.json 对齐）
var menuI18nKeys = map[string]string{
	// 系统管理
	"系统管理": "menu.system.label",
	"用户管理": "menu.system.user",
	"角色管理": "menu.system.role",
	"菜单管理": "menu.system.menu",
	"部门管理": "menu.system.dept",
	"日志管理": "menu.system.log",
	"系统设置": "menu.system.settings",
	// 插件
	"插件管理": "menu.plugin.label",
	// MCP
	"MCP 管理": "menu.mcp.label",
	"工具管理": "menu.mcp.tools",
	"资源管理": "menu.mcp.resources",
	"提示词管理": "menu.mcp.prompts",
	"审计日志": "menu.mcp.auditLogs",
	// AI
	"AI 对话":  "menu.ai.label",
	"对话助手":  "menu.ai.chat",
	"模型密钥管理": "menu.ai.providers",
	// 仪表盘
	"仪表盘": "menu.dashboard",
	// 按钮权限
	"用户新增": "menu.system.userAdd",
	"用户编辑": "menu.system.userEdit",
	"用户删除": "menu.system.userDelete",
	"重置密码": "menu.system.userResetPassword",
	"角色新增": "menu.system.roleAdd",
	"角色编辑": "menu.system.roleEdit",
	"角色删除": "menu.system.roleDelete",
	"菜单新增": "menu.system.menuAdd",
	"菜单编辑": "menu.system.menuEdit",
	"菜单删除": "menu.system.menuDelete",
	"部门新增": "menu.system.deptAdd",
	"部门编辑": "menu.system.deptEdit",
	"部门删除": "menu.system.deptDelete",
	"插件上传": "menu.plugin.import",
	"插件启停": "menu.plugin.toggle",
	"插件配置": "menu.plugin.config",
	"插件删除": "menu.plugin.delete",
	"插件重启": "menu.plugin.restart",
	"日志删除": "menu.system.logDelete",
	"设置编辑": "menu.system.settingsEdit",
	"MCP 调用": "menu.mcp.toolCall",
	// 多语言管理
	"语言管理": "menu.system.i18nLocale",
	"翻译维护": "menu.system.i18nTranslation",
}

// migrateMenuI18nKeys 幂等回填菜单 i18n_key（已有库按菜单名匹配补充，不覆盖已填值）
func migrateMenuI18nKeys(db *gorm.DB) {
	var menus []model.SysMenu
	if err := db.Find(&menus).Error; err != nil {
		return
	}
	patched := 0
	for _, m := range menus {
		if m.I18nKey != "" {
			continue
		}
		key, ok := menuI18nKeys[m.Name]
		if !ok {
			continue
		}
		if err := db.Model(&model.SysMenu{}).Where("id = ?", m.ID).Update("i18n_key", key).Error; err == nil {
			patched++
		}
	}
	if patched > 0 {
		log.Printf("种子数据：已为 %d 个菜单回填 i18n_key", patched)
	}
}

// seedLocales 语言注册表种子（已有库幂等：缺哪条补哪条，不动已有值）
func seedLocales(db *gorm.DB) {
	defaults := []model.SysLocale{
		{Code: "zh-CN", Name: "简体中文", Icon: "🇨🇳", IsEnabled: 1, IsDefault: 1, Sort: 1},
		{Code: "en-US", Name: "English", Icon: "🇺🇸", IsEnabled: 1, IsDefault: 0, Sort: 2},
	}
	for _, l := range defaults {
		var count int64
		db.Model(&model.SysLocale{}).Where("code = ?", l.Code).Count(&count)
		if count == 0 {
			db.Create(&l)
		}
	}
	// 保证恰好一门默认语言
	var defCount int64
	db.Model(&model.SysLocale{}).Where("is_default = ?", 1).Count(&defCount)
	if defCount == 0 {
		db.Model(&model.SysLocale{}).Where("code = ?", "zh-CN").Update("is_default", 1)
	}
}

// seedI18nMenus 多语言管理菜单（语言管理/翻译维护，挂系统管理下）+ 按钮权限
// 幂等：按 path 判断存在与否
func seedI18nMenus(db *gorm.DB) {
	// 语言管理 + 翻译维护（C 菜单）
	i18nMenus := []model.SysMenu{
		{Name: "语言管理", I18nKey: "menu.system.i18nLocale", ParentID: 1, Type: "C", Path: "i18n-locale", Component: "system/i18n/locale", Permission: "system:i18n:list", Icon: "Sell", Sort: 7, Visible: 1, Status: 1},
		{Name: "翻译维护", I18nKey: "menu.system.i18nTranslation", ParentID: 1, Type: "C", Path: "i18n-translation", Component: "system/i18n/translation", Permission: "system:i18n:list", Icon: "EditPen", Sort: 8, Visible: 1, Status: 1},
	}
	for _, m := range i18nMenus {
		var count int64
		db.Model(&model.SysMenu{}).Where("path = ? AND parent_id = 1", m.Path).Count(&count)
		if count == 0 {
			db.Create(&m)
		}
	}

	// i18n 按钮权限（挂在语言管理菜单下）
	var localeMenu model.SysMenu
	if err := db.Where("path = ? AND parent_id = 1", "i18n-locale").First(&localeMenu).Error; err != nil {
		return
	}
	buttonPerms := []model.SysMenu{
		{Name: "语言新增", I18nKey: "menu.system.i18nLocaleAdd", ParentID: localeMenu.ID, Type: "F", Permission: "system:i18n:add", Sort: 1, Status: 1},
		{Name: "语言编辑", I18nKey: "menu.system.i18nLocaleEdit", ParentID: localeMenu.ID, Type: "F", Permission: "system:i18n:edit", Sort: 2, Status: 1},
		{Name: "语言删除", I18nKey: "menu.system.i18nLocaleDelete", ParentID: localeMenu.ID, Type: "F", Permission: "system:i18n:delete", Sort: 3, Status: 1},
	}
	for _, m := range buttonPerms {
		var count int64
		db.Model(&model.SysMenu{}).Where("permission = ?", m.Permission).Count(&count)
		if count == 0 {
			db.Create(&m)
		}
	}

	// 超管角色补挂新菜单（超管持有全部权限；普通角色按需在角色管理中自行勾选）
	var adminRole model.SysRole
	if err := db.Where("code = ?", "admin").First(&adminRole).Error; err == nil {
		var newMenus []model.SysMenu
		db.Where("permission LIKE ?", "system:i18n:%").Find(&newMenus)
		if len(newMenus) > 0 {
			db.Model(&adminRole).Association("Menus").Append(&newMenus)
		}
	}
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
		{Name: "系统管理", I18nKey: "menu.system.label", ParentID: 0, Type: "M", Path: "/system", Icon: "Setting", Sort: 2, Visible: 1, Status: 1},
		{Name: "用户管理", I18nKey: "menu.system.user", ParentID: 1, Type: "C", Path: "user", Component: "system/user", Permission: "system:user:list", Icon: "User", Sort: 1, Visible: 1, Status: 1},
		{Name: "角色管理", I18nKey: "menu.system.role", ParentID: 1, Type: "C", Path: "role", Component: "system/role", Permission: "system:role:list", Icon: "UserFilled", Sort: 2, Visible: 1, Status: 1},
		{Name: "菜单管理", I18nKey: "menu.system.menu", ParentID: 1, Type: "C", Path: "menu", Component: "system/menu", Permission: "system:menu:list", Icon: "Menu", Sort: 3, Visible: 1, Status: 1},
		{Name: "部门管理", I18nKey: "menu.system.dept", ParentID: 1, Type: "C", Path: "dept", Component: "system/dept", Permission: "system:dept:list", Icon: "OfficeBuilding", Sort: 4, Visible: 1, Status: 1},
		// 插件管理：顶级菜单（ParentID=0），Sort=1 置于仪表盘正下方。
		// 注意：保持本行在创建顺序中的位置不变（ID=6），仅改 ParentID/Sort，避免按钮权限 ParentID 引用漂移。
		{Name: "插件管理", I18nKey: "menu.plugin.label", ParentID: 0, Type: "C", Path: "plugin", Component: "system/plugin", Permission: "system:plugin:list", Icon: "Box", Sort: 1, Visible: 1, Status: 1},
		{Name: "日志管理", I18nKey: "menu.system.log", ParentID: 1, Type: "C", Path: "log", Component: "system/log", Permission: "system:log:list", Icon: "Document", Sort: 5, Visible: 1, Status: 1},
		{Name: "系统设置", I18nKey: "menu.system.settings", ParentID: 1, Type: "C", Path: "settings", Component: "system/settings", Permission: "system:setting:list", Icon: "Tools", Sort: 6, Visible: 1, Status: 1},
		// MCP 管理
		{Name: "MCP 管理", I18nKey: "menu.mcp.label", ParentID: 0, Type: "M", Path: "/mcp", Icon: "Connection", Sort: 3, Visible: 1, Status: 1},
		{Name: "工具管理", I18nKey: "menu.mcp.tools", ParentID: 9, Type: "C", Path: "tools", Component: "mcp/tools", Permission: "mcp:tools:list", Icon: "Tools", Sort: 1, Visible: 1, Status: 1},
		{Name: "资源管理", I18nKey: "menu.mcp.resources", ParentID: 9, Type: "C", Path: "resources", Component: "mcp/resources", Permission: "mcp:resources:list", Icon: "FolderOpened", Sort: 2, Visible: 1, Status: 1},
		{Name: "提示词管理", I18nKey: "menu.mcp.prompts", ParentID: 9, Type: "C", Path: "prompts", Component: "mcp/prompts", Permission: "mcp:prompts:list", Icon: "ChatLineSquare", Sort: 3, Visible: 1, Status: 1},
		{Name: "审计日志", I18nKey: "menu.mcp.auditLogs", ParentID: 9, Type: "C", Path: "audit-logs", Component: "mcp/audit-logs", Permission: "mcp:audit:list", Icon: "Document", Sort: 4, Visible: 1, Status: 1},
		// AI 对话
		{Name: "AI 对话", I18nKey: "menu.ai.label", ParentID: 0, Type: "M", Path: "/ai", Icon: "ChatDotRound", Sort: 4, Visible: 1, Status: 1},
		{Name: "对话助手", I18nKey: "menu.ai.chat", ParentID: 14, Type: "C", Path: "chat", Component: "ai/chat", Permission: "ai:chat", Icon: "ChatLineRound", Sort: 1, Visible: 1, Status: 1},
		{Name: "模型密钥管理", I18nKey: "menu.ai.providers", ParentID: 14, Type: "C", Path: "providers", Component: "ai/providers", Permission: "ai:provider:list", Icon: "Key", Sort: 2, Visible: 1, Status: 1},
		// 仪表盘（前端硬编码跳转 /dashboard-monitor，component 对应 apeui/dashboard/Monitor.vue）
		// 最后创建（ID=17），靠 Sort=0 排到最前
		{Name: "仪表盘", I18nKey: "menu.dashboard", ParentID: 0, Type: "C", Path: "dashboard-monitor", Component: "apeui/dashboard/Monitor", Permission: "dashboard:view", Icon: "Odometer", Sort: 0, Visible: 1, Status: 1},
	}

	// 用户管理按钮权限
	menus = append(menus, []model.SysMenu{
		{Name: "用户新增", I18nKey: "menu.system.userAdd", ParentID: 2, Type: "F", Permission: "system:user:add", Sort: 1, Status: 1},
		{Name: "用户编辑", I18nKey: "menu.system.userEdit", ParentID: 2, Type: "F", Permission: "system:user:edit", Sort: 2, Status: 1},
		{Name: "用户删除", I18nKey: "menu.system.userDelete", ParentID: 2, Type: "F", Permission: "system:user:delete", Sort: 3, Status: 1},
		{Name: "重置密码", I18nKey: "menu.system.userResetPassword", ParentID: 2, Type: "F", Permission: "system:user:reset-password", Sort: 4, Status: 1},
		{Name: "角色新增", I18nKey: "menu.system.roleAdd", ParentID: 3, Type: "F", Permission: "system:role:add", Sort: 1, Status: 1},
		{Name: "角色编辑", I18nKey: "menu.system.roleEdit", ParentID: 3, Type: "F", Permission: "system:role:edit", Sort: 2, Status: 1},
		{Name: "角色删除", I18nKey: "menu.system.roleDelete", ParentID: 3, Type: "F", Permission: "system:role:delete", Sort: 3, Status: 1},
		{Name: "菜单新增", I18nKey: "menu.system.menuAdd", ParentID: 4, Type: "F", Permission: "system:menu:add", Sort: 1, Status: 1},
		{Name: "菜单编辑", I18nKey: "menu.system.menuEdit", ParentID: 4, Type: "F", Permission: "system:menu:edit", Sort: 2, Status: 1},
		{Name: "菜单删除", I18nKey: "menu.system.menuDelete", ParentID: 4, Type: "F", Permission: "system:menu:delete", Sort: 3, Status: 1},
		{Name: "部门新增", I18nKey: "menu.system.deptAdd", ParentID: 5, Type: "F", Permission: "system:dept:add", Sort: 1, Status: 1},
		{Name: "部门编辑", I18nKey: "menu.system.deptEdit", ParentID: 5, Type: "F", Permission: "system:dept:edit", Sort: 2, Status: 1},
		{Name: "部门删除", I18nKey: "menu.system.deptDelete", ParentID: 5, Type: "F", Permission: "system:dept:delete", Sort: 3, Status: 1},
		{Name: "插件上传", I18nKey: "menu.plugin.import", ParentID: 6, Type: "F", Permission: "system:plugin:upload", Sort: 1, Status: 1},
		{Name: "插件启停", I18nKey: "menu.plugin.toggle", ParentID: 6, Type: "F", Permission: "system:plugin:toggle", Sort: 2, Status: 1},
		{Name: "插件配置", I18nKey: "menu.plugin.config", ParentID: 6, Type: "F", Permission: "system:plugin:config", Sort: 3, Status: 1},
		{Name: "插件删除", I18nKey: "menu.plugin.delete", ParentID: 6, Type: "F", Permission: "system:plugin:delete", Sort: 4, Status: 1},
		{Name: "插件重启", I18nKey: "menu.plugin.restart", ParentID: 6, Type: "F", Permission: "system:plugin:restart", Sort: 5, Status: 1},
		{Name: "日志删除", I18nKey: "menu.system.logDelete", ParentID: 7, Type: "F", Permission: "system:log:delete", Sort: 1, Status: 1},
		{Name: "设置编辑", I18nKey: "menu.system.settingsEdit", ParentID: 8, Type: "F", Permission: "system:setting:edit", Sort: 1, Status: 1},
		{Name: "MCP 调用", I18nKey: "menu.mcp.toolCall", ParentID: 10, Type: "F", Permission: "mcp:tools:call", Sort: 1, Status: 1},
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

	// is_public 与 Python 版对齐：后台访问路径/侧边栏主题为私密项，不通过公共接口暴露
	settings := []model.SysSetting{
		{Key: "site_name", Value: "ApeAdmin-Gin", IsPublic: true},
		{Key: "logo_url", Value: "", IsPublic: true},
		{Key: "primary_color", Value: "#5A67F5", IsPublic: true},
		{Key: "admin_path", Value: "/admin", IsPublic: false},
		{Key: "site_url", Value: "", IsPublic: true},
		{Key: "footer_text", Value: "ApeAdmin © 2026", IsPublic: true},
		{Key: "login_bg", Value: "", IsPublic: true},
		{Key: "sidebar_theme", Value: "light", IsPublic: false},
		{Key: "theme_mode", Value: "light", IsPublic: true},
	}
	for _, s := range settings {
		db.Create(&s)
	}
	log.Println("种子数据：系统设置已初始化")
}

// migrateMissingSettings 补齐缺失的品牌类设置项（不覆盖已有值）
func migrateMissingSettings(db *gorm.DB) {
	defaults := []model.SysSetting{
		{Key: "admin_path", Value: "/admin", IsPublic: false},
		{Key: "site_url", Value: "", IsPublic: true},
		{Key: "footer_text", Value: "ApeAdmin © 2026", IsPublic: true},
		{Key: "login_bg", Value: "", IsPublic: true},
		{Key: "sidebar_theme", Value: "light", IsPublic: false},
		{Key: "theme_mode", Value: "light", IsPublic: true},
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
