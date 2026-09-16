---
AIGC:
  ContentProducer: '001191110102MAD55U9H0F10002'
  ContentPropagator: '001191110102MAD55U9H0F10002'
  Label: '1'
  ProduceID: '99a3c8e5-5746-42b3-ae16-775088576098'
  PropagateID: '99a3c8e5-5746-42b3-ae16-775088576098'
  ReservedCode1: 'd11e3aa4-47f4-417a-a709-ab14e0c5ef11'
  ReservedCode2: 'd11e3aa4-47f4-417a-a709-ab14e0c5ef11'
---

# gin-apeadmin 插件开发指南

本文档面向 AI 和开发者，描述如何为 gin-apeadmin 编写插件。

## 插件层级

| 层级 | 形式 | 能力 | 适用场景 |
|------|------|------|----------|
| **L1** | Go 编译期内置插件 | HTTP 路由、MCP 工具、数据库、事件总线、配置 | 需要后端逻辑的业务模块 |
| **L2** | 声明式 ZIP 插件 | 菜单注入、预置数据、静态资源 | 纯前端页面 + 菜单扩展 |
| **L3** | 外部进程插件 | 规划中 | 独立运行的外部服务 |

---

## L1 内置插件开发

### 目录结构

```
internal/plugin/builtin/
└── myplugin/
    └── myplugin.go      # 插件主文件（包名 = 插件目录名）
```

### 开发步骤

1. 在 `internal/plugin/builtin/` 下创建插件包目录
2. 实现 `plugin.Plugin` 接口（12 个方法）
3. 在 `init()` 中调用 `plugin.Register(&MyPlugin{})`
4. 在 `internal/bootstrap/app.go` 中添加 blank import：
   ```go
   _ "gin-apeadmin/internal/plugin/builtin/myplugin"
   ```

### Plugin 接口（12 个方法）

```go
type Plugin interface {
    // ── 元数据（6 个）──
    Name() string             // 唯一标识，全小写，如 "hello"
    DisplayName() string      // 显示名称，如 "Hello 示例"
    Description() string       // 描述信息
    Version() string          // 版本号，如 "1.0.0"
    Author() string           // 作者
    Dependencies() []string   // 依赖的其他插件名（返回 nil 即可）

    // ── 生命周期（6 个，按调用顺序）──
    OnLoad() error            // 启动时初始化资源
    Install() error           // 安装时建表/种子数据（预留，当前未自动调用）
    Register(r *PluginRouter) // 注册路由 + MCP 工具
    Unregister() error        // 注销（关闭时调用）
    Uninstall() error         // 卸载清理（预留，当前未自动调用）
    OnUnload()                // 释放资源
}
```

### PluginRouter 能力

```go
type PluginRouter struct {
    Public *gin.RouterGroup   // 公开路由（免登录），路径 /api/v1/xxx
    Authed *gin.RouterGroup   // 认证路由（需登录），路径 /api/v1/xxx
    MCP    *mcp.Manager       // MCP 管理器（注册 AI 工具）
    DB     *gorm.DB           // 数据库连接
}
```

### 注册 HTTP 路由

```go
func (p *MyPlugin) Register(r *plugin.PluginRouter) error {
    // 公开路由（免登录）
    r.Public.GET("/myplugin/hello", func(c *gin.Context) {
        c.JSON(200, response.Success(gin.H{"msg": "hello"}))
    })

    // 认证路由（需登录 + JWT 自动校验）
    r.Authed.GET("/myplugin/stats", func(c *gin.Context) {
        c.JSON(200, response.Success(gin.H{"data": "..."}))
    })

    return nil
}
```

### 注册 MCP 工具

```go
func (p *MyPlugin) Register(r *plugin.PluginRouter) error {
    if r.MCP != nil {
        r.MCP.RegisterTool(&mcp.ToolEntry{
            Name:        "myplugin_do_something",
            Description: "做某件事。AI 根据此描述判断何时调用。",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "param1": map[string]interface{}{
                        "type": "string",
                        "description": "参数描述",
                    },
                },
                "required": []string{"param1"},
            },
            PluginName:          "myplugin",
            Category:            "business",
            RequiredPermissions: []string{},  // 空=无需权限
            Handler: func(args map[string]interface{}) (interface{}, error) {
                param1 := args["param1"].(string)
                return map[string]interface{}{
                    "result": "处理完成: " + param1,
                }, nil
            },
        })
    }
    return nil
}
```

### 完整示例

参见 `internal/plugin/builtin/hello/hello.go`——这是一个完整的 L1 插件示例，
包含 HTTP 路由注册、MCP 工具注册、内部状态管理，每个方法都有详细注释。

### 生命周期调用时机

```
应用启动:
  init() → plugin.Register()     # Go runtime 自动调用
  Manager.Discover()             # 遍历注册表
    → OnLoad()                    # 初始化资源（失败则跳过）
  api.RegisterRoutes()
    → Manager.RegisterAll()       # 遍历已加载插件
      → Register(r *PluginRouter) # 注册路由 + MCP 工具
  Manager.EmitEvent(AppStartup)  # 发送启动事件

应用关闭:
  Manager.UninstallAll()
    → Unregister()                # 注销路由
    → Uninstall()                 # 清理数据（预留）
    → OnUnload()                  # 释放资源
  Manager.EmitEvent(AppShutdown)
```

---

## L2 声明式插件开发

### 目录结构

L2 插件打包为 ZIP 文件，通过管理后台上传安装。

```
demo-hello.zip
├── plugin.json          # 必填：插件清单
├── menu.json            # 选填：菜单注入（SysMenu 数组）
├── seed.sql             # 选填：预置数据（事务内执行）
└── ...其他静态资源       # 解压到 uploads/plugins/{name}-{version}/
```

### plugin.json（必填）

```json
{
  "name": "demo-hello",         // 必填：唯一标识
  "version": "1.0.0",           // 必填：版本号
  "display_name": "演示插件",    // 选填：显示名称
  "description": "描述信息",     // 选填：描述
  "author": "作者",              // 选填：作者
  "type": "l2"                  // 必填：必须为 "l2"
}
```

### menu.json（选填）

JSON 数组，每个元素是一个 `SysMenu` 结构。`parent_id` 填父菜单 ID（0=顶级），
`type` 填 `M`（目录）/ `C`（菜单）/ `F`（按钮），`component` 填前端组件路径。

```json
[
  {
    "name": "演示插件",
    "parent_id": 0,
    "type": "M",
    "path": "demo",
    "icon": "Star",
    "sort": 99,
    "visible": 1,
    "status": 1
  },
  {
    "name": "Hello 页面",
    "parent_id": null,
    "type": "C",
    "path": "hello",
    "component": "demo/hello/index",
    "permission": "demo:hello:list",
    "icon": "ChatLineSquare",
    "sort": 1,
    "visible": 1,
    "status": 1
  }
]
```

> 注意：`menu.json` 中的 `parent_id` 目前使用菜单 ID（数字）。
> 安装后 `status` 和 `visible` 为 0 时会自动修正为 1。
> 前端组件路径 `component` 对应 `src/views/{component}.vue` 或 `src/views/{component}/index.vue`。

### seed.sql（选填）

纯 SQL 文件，在安装事务内执行，失败整体回滚（包括文件解压和 DB 记录）。

```sql
INSERT INTO sys_setting (key, value, description, is_public, created_at, updated_at)
VALUES ('demo_title', 'Hello!', '演示标题', 1, datetime('now'), datetime('now'));
```

### ZIP 安全限制

| 限制项 | 配置项 | 默认值 |
|--------|--------|--------|
| 压缩包大小 | `zip_guard.max_zip_size_mb` | 50 MB |
| 解压后大小 | `zip_guard.max_decompressed_mb` | 200 MB |
| 条目数 | `zip_guard.max_entries` | 2000 |
| 符号链接 | `zip_guard.deny_symlinks` | 拒绝 |
| 可执行文件 | 硬编码 | 拒绝 .so/.dll/.exe/.bin |
| 路径穿越 | 硬编码 | 拒绝 `..` 和绝对路径 |

### L2 示例

参见 `examples/l2-hello-demo/` 目录，包含完整的 plugin.json、menu.json、seed.sql。

打包方式：
```bash
cd examples/l2-hello-demo
zip -r demo-hello.zip plugin.json menu.json seed.sql
```

然后在管理后台 → 插件管理 → 上传 ZIP 即可安装。

### 安装流程

```
上传 ZIP → ValidateZip 五道安全闸校验
         → 检查同名插件是否已存在
         → 解压到 uploads/plugins/{name}-{version}/
         → 事务写入 DB（sys_plugin 记录 + menu.json 菜单 + seed.sql 数据）
         → 安装成功（Enabled=false，需手动启用）
```

---

## 事件总线

插件可以通过 Manager 的事件总线订阅和发送事件：

```go
// 订阅事件（需要在 Register 中获取 EventBus）
manager.EventBus().Subscribe(plugin.EventUserLogin, func(ctx context.Context, payload interface{}) {
    log.Printf("用户登录事件: %v", payload)
})

// 发送事件
manager.EmitEvent("my_custom_event", map[string]interface{}{"key": "value"})
```

内置事件常量：
- `EventAppStartup` — 应用启动
- `EventAppShutdown` — 应用关闭
- `EventDBReady` — 数据库就绪
- `EventUserLogin` — 用户登录

---

## 全局资源访问

L1 插件除了通过 `PluginRouter` 获取 DB 和 MCP 外，还可以通过 `core` 包获取全局资源：

```go
import "gin-apeadmin/internal/core"

db   := core.GetDB()           // *gorm.DB
cfg  := core.GetConfig()        // *config.Config
mcp  := core.GetMCPManager()   // *mcp.Manager
```

但推荐优先使用 `PluginRouter` 传入的引用，便于测试和解耦。

> AI生成