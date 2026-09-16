// ═══════════════════════════════════════════════════════════════════════════
// Plugin 包示例：Hello World 插件（L1 编译期内置插件）
// ═══════════════════════════════════════════════════════════════════════════
//
// 【概述】
// 这是 apeadmin-gin 的 L1 内置插件示例。L1 插件是编译期内置插件，
// 代码直接编译进主二进制文件，通过 init() 函数自动注册到全局注册表。
// 适合需要访问数据库、注册 HTTP 路由、注册 MCP 工具的场景。
//
// 【L1 插件开发步骤】
// 1. 在 internal/plugin/builtin/ 下创建插件包（如 hello/）
// 2. 实现 plugin.Plugin 接口（12 个方法：6 元数据 + 6 生命周期）
// 3. 在 init() 中调用 plugin.Register(&MyPlugin{})
// 4. 确保插件包被 import（在 bootstrap/app.go 或通过 blank import）
//
// 【L1 vs L2 对比】
// ┌──────────┬─────────────────────────────┬──────────────────────────────────┐
// │          │ L1 编译期内置插件            │ L2 声明式 ZIP 插件               │
// ├──────────┼─────────────────────────────┼──────────────────────────────────┤
// │ 语言     │ Go（编译进主二进制）          │ plugin.json + 静态资源            │
// │ 注册方式  │ init() → plugin.Register()  │ 上传 ZIP → 自动解压注入 DB        │
// │ HTTP 路由│ ✅ r.Public / r.Authed       │ ❌ 不支持                        │
// │ MCP 工具 │ ✅ r.MCP.RegisterTool()      │ ❌ 不支持                        │
// │ 数据库   │ ✅ r.DB（*gorm.DB）          │ ❌ 仅 seed.sql 预置              │
// │ 事件总线 │ ✅ Subscribe/Emit            │ ❌ 不支持                        │
// │ 配置     │ ✅ core.GetConfig()          │ ✅ 管理界面读写 sys_plugin.config │
// │ 适用场景  │ 需要后端逻辑的业务模块        │ 纯前端页面 + 菜单注入             │
// └──────────┴─────────────────────────────┴──────────────────────────────────┘
//
// 【Plugin 接口 12 个方法速查】
// 元数据方法（6 个）：
//   Name() string             — 插件唯一标识（如 "hello"）
//   DisplayName() string      — 显示名称（如 "Hello 示例"）
//   Description() string      — 描述信息
//   Version() string          — 版本号（如 "1.0.0"）
//   Author() string           — 作者
//   Dependencies() []string   — 依赖的其他插件名（暂未强制校验，返回 nil 即可）
//
// 生命周期方法（6 个，按调用顺序）：
//   OnLoad() error             — 应用启动时调用（初始化资源、校验环境）
//   Install() error           — 安装时调用（建表、注入种子数据）— 当前框架未自动调用
//   Register(r *PluginRouter)  — 注册 HTTP 路由和 MCP 工具
//   Unregister() error         — 注销路由和工具（应用关闭时调用）
//   Uninstall() error          — 卸载时调用（清理数据）— 当前框架未自动调用
//   OnUnload()                 — 释放资源（应用关闭时调用）
//
// 【PluginRouter 提供的能力】
//   r.Public  *gin.RouterGroup  — 公开路由组（免登录，如 /api/v1/hello/public）
//   r.Authed  *gin.RouterGroup  — 认证路由组（需登录 + 权限，如 /api/v1/hello/stats）
//   r.MCP     *mcp.Manager      — MCP 管理器（注册 AI 可调用的工具）
//   r.DB      *gorm.DB          — 数据库连接（直接操作表）
//
// ═══════════════════════════════════════════════════════════════════════════

package hello

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/mcp"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/response"
	"apeadmin-gin/internal/plugin"
)

// ────────────────────────────────────────────────────────────────────────────
// 插件结构体
// ────────────────────────────────────────────────────────────────────────────

// HelloPlugin Hello World 示例插件
type HelloPlugin struct {
	// mu 保护并发访问的内部状态
	mu sync.RWMutex

	// callCount 记录 hello 接口被调用的次数（演示插件内部状态）
	callCount int64

	// startedAt 插件启动时间（演示 OnLoad 时初始化资源）
	startedAt string
}

// ────────────────────────────────────────────────────────────────────────────
// 元数据方法（6 个）
// ────────────────────────────────────────────────────────────────────────────

// Name 插件唯一标识
// 这个值用于：全局注册表的 key、MCP 工具的 PluginName 字段、日志前缀
// 命名规范：全小写、不含空格、使用下划线或连字符分隔（如 "hello"、"ai_vision"）
func (p *HelloPlugin) Name() string { return "hello" }

// DisplayName 显示名称（出现在管理界面的插件列表中）
func (p *HelloPlugin) DisplayName() string { return "Hello 示例插件" }

// Description 描述信息（出现在管理界面的插件列表中）
func (p *HelloPlugin) Description() string {
	return "L1 编译期内置插件示例，演示 HTTP 路由注册 + MCP 工具注册 + 事件订阅"
}

// Version 版本号（语义化版本，如 "1.0.0"）
func (p *HelloPlugin) Version() string { return "1.0.0" }

// Author 作者信息
func (p *HelloPlugin) Author() string { return "ApeAdmin Team" }

// Dependencies 依赖的其他插件名
// 当前框架未强制校验依赖关系，返回 nil 或空切片即可
// 未来如果框架支持依赖加载顺序，这里填被依赖插件的 Name()
func (p *HelloPlugin) Dependencies() []string { return nil }

// ────────────────────────────────────────────────────────────────────────────
// 生命周期方法（6 个）
// ────────────────────────────────────────────────────────────────────────────

// OnLoad 加载时调用
// 用途：初始化资源、校验运行环境、读取配置、在 sys_plugin 表中登记自己
// 时机：应用启动时，由 Manager.Discover() 调用
// 如果返回 error，插件会被跳过（不阻止其他插件加载），但仍保留在 plugins map 中
//
// 【L1 插件登记到 sys_plugin 表】
// L1 插件是编译期内置的，但管理界面（插件管理页面）读取的是 sys_plugin 表。
// 如果不在表中登记，插件虽然在运行（路由/MCP 工具已注册），但管理页面看不到它。
// 因此 OnLoad 中需要幂等地写入 sys_plugin 记录：
//   - 如果记录已存在（按 name 查），跳过
//   - 如果不存在，创建一条记录（Enabled=true，因为 L1 插件编译期已激活）
func (p *HelloPlugin) OnLoad() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 示例：初始化插件内部状态
	p.callCount = 0
	p.startedAt = time.Now().Format("2006-01-02 15:04:05")

	// 在 sys_plugin 表中登记自己（幂等：已存在则跳过）
	// 这样插件管理页面就能看到这个插件了
	existing, err := dal.GetPluginByName(p.Name())
	if err == nil && existing != nil {
		// 已有记录，检查是否需要更新版本号等信息
		if existing.Version != p.Version() || existing.DisplayName != p.DisplayName() {
			existing.DisplayName = p.DisplayName()
			existing.Description = p.Description()
			existing.Version = p.Version()
			existing.Author = p.Author()
			existing.Enabled = true // L1 插件编译期已激活
			_ = dal.UpdatePlugin(existing)
		}
	} else {
		// 不存在，创建新记录
		record := &model.SysPlugin{
			Name:        p.Name(),
			DisplayName: p.DisplayName(),
			Description: p.Description(),
			Version:     p.Version(),
			Author:      p.Author(),
			Enabled:     true, // L1 插件编译期已激活
			ModulePath:  "(builtin)", // L1 插件无独立文件目录
		}
		if err := dal.CreatePlugin(record); err != nil {
			log.Printf("[hello] 登记 sys_plugin 失败: %v（不影响插件运行）", err)
		}
	}

	log.Printf("[hello] 插件已加载，启动时间: %s", p.startedAt)
	return nil
}

// Install 安装时调用
// 用途：建表、注入种子数据、创建初始配置
// 注意：当前框架版本中 Install() 不会被自动调用（L1 插件编译期已内置），
//       L1 插件的登记逻辑已放在 OnLoad() 中执行，此方法预留扩展使用
func (p *HelloPlugin) Install() error {
	log.Printf("[hello] Install 调用（预留方法，当前由 OnLoad 完成登记）")
	return nil
}

// Register 注册 HTTP 路由和 MCP 工具
// 用途：在此方法中通过 PluginRouter 注册插件自己的 API 路由和 MCP 工具
// 时机：应用启动时，由 Manager.RegisterAll() 调用（在所有系统路由注册之后）
//
// 参数 r *plugin.PluginRouter 包含：
//   r.Public — 公开路由组（免登录），用于不需要认证的接口
//   r.Authed — 认证路由组（需登录 + 权限），用于需要登录的接口
//   r.MCP    — MCP 管理器，用于注册 AI 可调用的工具
//   r.DB     — 数据库连接，用于直接操作表
func (p *HelloPlugin) Register(r *plugin.PluginRouter) error {
	// ─── 注册公开 HTTP 路由（免登录）───
	// 路由路径前缀会自动加上 /api/v1，最终路径为 /api/v1/hello/public
	// 适合：公开的回调接口、Webhook、健康检查等
	r.Public.GET("/hello/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Success(gin.H{
			"message":    "Hello, World!",
			"started_at": p.getStartedAt(),
		}))
	})

	// ─── 注册认证 HTTP 路由（需登录 + 权限）───
	// 路径前缀同样自动加上 /api/v1，最终路径为 /api/v1/hello/stats
	// 适合：需要登录认证的业务接口
	// 注意：r.Authed 已包含 JWTAuth 中间件，但权限校验需要自己在 handler 中调用
	//       或使用 middleware.RequirePermission("plugin:hello:stats") 中间件
	r.Authed.GET("/hello/stats", func(c *gin.Context) {
		p.mu.Lock()
		p.callCount++
		count := p.callCount
		p.mu.Unlock()

		c.JSON(http.StatusOK, response.Success(gin.H{
			"call_count": count,
			"started_at": p.getStartedAt(),
			"version":    p.Version(),
		}))
	})

	// ─── 注册 MCP 工具（AI 可调用）───
	// MCP 工具注册后，AI 对话助手可以自动选择并调用这些工具
	// 例如用户说"帮我打个招呼"，AI 会调用 hello_say_hello 工具
	if r.MCP != nil {
		registerMCPTools(r.MCP)
	}

	log.Printf("[hello] 路由已注册: GET /api/v1/hello/public, GET /api/v1/hello/stats")
	log.Printf("[hello] MCP 工具已注册: hello_say_hello, hello_get_time")
	return nil
}

// Unregister 注销路由和工具
// 用途：清理 Register 中注册的资源
// 时机：应用优雅关闭时，由 Manager.UninstallAll() 调用
// 注意：Gin 不支持动态移除路由，MCP 工具可通过 mcp.UnregisterPluginTools() 清理
func (p *HelloPlugin) Unregister() error {
	// 如果注册了 MCP 工具，这里可以注销
	// 方式一：通过 mcp.Manager.UnregisterPluginTools("hello") 批量清理
	// 方式二：无需手动清理（应用关闭时所有内存资源自动释放）
	log.Printf("[hello] 路由已注销")
	return nil
}

// Uninstall 卸载时调用
// 用途：清理数据库表、删除文件等
// 注意：当前框架版本中 Uninstall() 不会被自动调用（L1 插件编译期已内置），
//       此方法预留给未来"插件卸载向导"功能使用，目前可留空实现
func (p *HelloPlugin) Uninstall() error {
	log.Printf("[hello] Uninstall 调用（当前框架版本中此方法为预留，不自动触发）")
	return nil
}

// OnUnload 释放资源
// 用途：关闭数据库连接、停止后台 goroutine、释放文件句柄等
// 时机：应用优雅关闭时，由 Manager.UninstallAll() 调用（在 Unregister 之后）
func (p *HelloPlugin) OnUnload() {
	log.Printf("[hello] 插件已卸载，总共被调用 %d 次", p.callCount)
}

// ────────────────────────────────────────────────────────────────────────────
// init() 自动注册——这是 L1 插件的注册入口
// ────────────────────────────────────────────────────────────────────────────

// init 在包被 import 时自动执行，将插件注册到全局注册表
// Manager.Discover() 会遍历注册表并调用每个插件的 OnLoad()
//
// 【重要】确保此包被 import 的方式：
//   方式一（推荐）：在 internal/bootstrap/app.go 中 import _ "apeadmin-gin/internal/plugin/builtin/hello"
//   方式二：在 cmd/server/main.go 中 import _ "apeadmin-gin/internal/plugin/builtin/hello"
//   使用 blank import（_ "路径"）因为只需要执行 init()，不需要直接引用包内符号
func init() {
	plugin.Register(&HelloPlugin{})
}

// ────────────────────────────────────────────────────────────────────────────
// 内部辅助方法
// ────────────────────────────────────────────────────────────────────────────

func (p *HelloPlugin) getStartedAt() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.startedAt
}

// ────────────────────────────────────────────────────────────────────────────
// MCP 工具注册
// ────────────────────────────────────────────────────────────────────────────

// registerMCPTools 注册 MCP 工具到 MCP 管理器
// MCP 工具注册后，AI 对话助手可以自动选择并调用这些工具
// 用户在对话中说"帮我打个招呼"时，AI 会识别意图并调用 hello_say_hello
//
// ToolEntry 字段说明：
//   Name                — 工具唯一标识（全局唯一，建议用 插件名_功能 格式）
//   Description         — 工具描述（AI 根据此描述判断是否调用此工具）
//   InputSchema         — JSON Schema 格式的参数定义（AI 据此生成参数）
//   PluginName          — 归属插件名（用于批量注销，填 p.Name()）
//   Category            — 工具分类（如 "demo"、"system"，MCP 管理页按分类展示）
//   RequiredPermissions — 调用此工具需要的权限码（空切片=无需权限）
//   Handler             — 处理函数，接收 args map，返回结果或 error
func registerMCPTools(mcpMgr *mcp.Manager) {
	// ─── 工具 1：hello_say_hello ───
	// 演示一个带参数的 MCP 工具
	mcpMgr.RegisterTool(&mcp.ToolEntry{
		Name:        "hello_say_hello",
		Description: "打个招呼。传入名字，返回问候语。例如用户说'帮我跟张三打个招呼'时调用此工具。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "要打招呼的人的名字",
				},
			},
			"required": []string{"name"},
		},
		PluginName:          "hello",
		Category:            "demo",
		RequiredPermissions: []string{}, // 空=无需权限校验
		Handler: func(args map[string]interface{}) (interface{}, error) {
			name, ok := args["name"].(string)
			if !ok || name == "" {
				name = "World"
			}
			return map[string]interface{}{
				"greeting": fmt.Sprintf("你好，%s！这是来自 Hello 插件的问候。", name),
			}, nil
		},
	})

	// ─── 工具 2：hello_get_time ───
	// 演示一个无参数的 MCP 工具
	mcpMgr.RegisterTool(&mcp.ToolEntry{
		Name:        "hello_get_time",
		Description: "获取当前服务器时间。用户询问时间时调用此工具。",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
		PluginName:          "hello",
		Category:            "demo",
		RequiredPermissions: []string{},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{
				"current_time": time.Now().Format("2006-01-02 15:04:05"),
				"timezone":      "Local",
			}, nil
		},
	})

	log.Printf("[hello] MCP 工具已注册: hello_say_hello, hello_get_time")
}
