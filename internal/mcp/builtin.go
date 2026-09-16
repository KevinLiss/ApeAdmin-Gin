package mcp

import (
	"fmt"
	"runtime"
	"time"
)

// RegisterBuiltin 注册内置资源、工具与提示词
func RegisterBuiltin(m *Manager) {
	registerBuiltinResources(m)
	registerBuiltinTools(m)
	registerBuiltinPrompts(m)
}

func registerBuiltinResources(m *Manager) {
	m.RegisterResource(&ResourceEntry{
		URI:         "apeadmin://system/status",
		Name:        "系统状态",
		Description: "获取系统运行状态信息，包括版本、运行时间等",
		MimeType:    "text/plain",
		ReadHandler: func() (string, error) {
			cfg := m.GetConfig()
			version := "dev"
			debug := false
			if cfg != nil {
				version = cfg.App.Version
				debug = cfg.App.Debug
			}
			return fmt.Sprintf("App: ApeAdmin-Gin %s\nDebug: %v\nGo: %s\nTime: %s",
				version, debug, runtime.Version(),
				time.Now().Format("2006-01-02 15:04:05")), nil
		},
	})
	m.RegisterResource(&ResourceEntry{
		URI:         "apeadmin://users/count",
		Name:        "用户总数",
		Description: "查询系统当前注册用户总数",
		MimeType:    "text/plain",
		ReadHandler: func() (string, error) {
			db := m.GetDB()
			if db == nil {
				return "", fmt.Errorf("数据库不可用")
			}
			var count int64
			db.Table("sys_user").Where("deleted_at IS NULL").Count(&count)
			return fmt.Sprintf("用户总数: %d", count), nil
		},
	})
	m.RegisterResource(&ResourceEntry{
		URI:         "apeadmin://system/info",
		Name:        "系统信息",
		Description: "获取服务器运行环境信息（Go 版本、OS、CPU 核心数）",
		MimeType:    "text/plain",
		ReadHandler: func() (string, error) {
			return fmt.Sprintf("OS: %s/%s\nGo: %s\nCPU cores: %d",
				runtime.GOOS, runtime.GOARCH, runtime.Version(), runtime.NumCPU()), nil
		},
	})
}

func registerBuiltinTools(m *Manager) {
	// 系统健康检查
	m.RegisterTool(&ToolEntry{
		Name:        "system_health_check",
		Description: "检查系统健康状态，包括数据库连接、版本信息与运行时长",
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "required": []string{}},
		RequiredPermissions: []string{},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			cfg := m.GetConfig()
			db := m.GetDB()
			dbStatus := "ok"
			if db == nil {
				dbStatus = "unavailable"
			} else {
				sqlDB, err := db.DB()
				if err != nil || sqlDB.Ping() != nil {
					dbStatus = "error"
				}
			}
			version := "unknown"
			if cfg != nil {
				version = cfg.App.Version
			}
			return map[string]interface{}{
				"status":     "healthy",
				"version":    version,
				"db":         dbStatus,
				"go_version": runtime.Version(),
				"goroutines": runtime.NumGoroutine(),
			}, nil
		},
	})

	// 角色管理工具（与 Python 版对齐：role_list/role_create）
	roleSchema := func(required []string, props map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{"type": "object", "properties": props, "required": required}
	}
	m.RegisterTool(&ToolEntry{
		Name:        "role_list",
		Description: "获取角色列表（分页）",
		InputSchema: roleSchema([]string{}, map[string]interface{}{
			"page":      map[string]interface{}{"type": "integer", "description": "页码，默认 1"},
			"page_size": map[string]interface{}{"type": "integer", "description": "每页数量，默认 20"},
		}),
		RequiredPermissions: []string{"system:role:list"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			db := m.GetDB()
			if db == nil {
				return nil, fmt.Errorf("数据库不可用")
			}
			type row struct {
				ID        uint   `json:"id"`
				Name      string `json:"name"`
				Code      string `json:"code"`
				DataScope int    `json:"data_scope"`
				Status    int    `json:"status"`
			}
			var items []row
			db.Table("sys_role").Where("deleted_at IS NULL").Find(&items)
			return map[string]interface{}{"total": len(items), "items": items}, nil
		},
	})
	m.RegisterTool(&ToolEntry{
		Name:        "role_create",
		Description: "创建新角色。需要提供角色名称和编码。",
		InputSchema: roleSchema([]string{"name", "code"}, map[string]interface{}{
			"name":       map[string]interface{}{"type": "string", "description": "角色名称"},
			"code":       map[string]interface{}{"type": "string", "description": "角色编码"},
			"data_scope": map[string]interface{}{"type": "integer", "description": "数据范围(1本人2本部门及以下3本部门4全部)"},
		}),
		RequiredPermissions: []string{"system:role:add"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			name, _ := args["name"].(string)
			code, _ := args["code"].(string)
			if name == "" || code == "" {
				return nil, fmt.Errorf("name 和 code 必填")
			}
			db := m.GetDB()
			if db == nil {
				return nil, fmt.Errorf("数据库不可用")
			}
			result := db.Exec("INSERT INTO sys_role (name, code, data_scope, sort, status, created_at, updated_at) VALUES (?, ?, 1, 0, 1, datetime('now'), datetime('now'))", name, code)
			if result.Error != nil {
				return nil, result.Error
			}
			return map[string]interface{}{"created": true, "name": name, "code": code}, nil
		},
	})
}

func registerBuiltinPrompts(m *Manager) {
	m.RegisterPrompt(&PromptEntry{
		Name:        "system_summary",
		Description: "生成系统运行概况摘要",
		Arguments:   []string{"system_name", "version", "check_time"},
		Template:    "请基于以下信息生成一份系统运行概况报告：\n系统名称：{system_name}\n版本：{version}\n检查时间：{check_time}",
	})
}