---
AIGC:
  ContentProducer: '001191110102MAD55U9H0F10002'
  ContentPropagator: '001191110102MAD55U9H0F10002'
  Label: '1'
  ProduceID: 'ddf021af-cc2a-4519-9512-5566b3a31da7'
  PropagateID: 'ddf021af-cc2a-4519-9512-5566b3a31da7'
  ReservedCode1: 'fe41c7d9-bc76-4d1f-8e78-7fc45711cc3f'
  ReservedCode2: 'fe41c7d9-bc76-4d1f-8e78-7fc45711cc3f'
---

# Gin-ApeAdmin

基于 Go (Gin) 的插件化后台管理框架，后端重写自 [ApeAdmin](https://github.com/KevinLiss/ApeAdmin)（Python FastAPI），前端复用 apeadmin 现有 Vue3 前端。

## 技术栈

- **Web 框架**: Gin v1.12
- **ORM**: GORM v1.31（MySQL / SQLite 双驱动）
- **认证**: JWT v5（黑名单 + TokenVersion 双机制）
- **配置**: Viper v1.21
- **日志**: Zap v1.28
- **MCP**: mcp-go（Model Context Protocol）

## 核心特性

- 插件化架构：L1 编译内置 / L2 声明式 ZIP / L3 外部进程
- MCP 工具管理：RBAC 过滤 + 审计日志 + 超时隔离
- 四层权限模型：免登录 / 仅登录 / 规则鉴权 / 数据权限
- 事件总线：APP_STARTUP / APP_SHUTDOWN / DB_READY 等
- 优雅关闭：三阶段（停请求 → 卸载插件 → 日志 drain + DB 断连）

## 快速开始

```bash
# 编译
go build ./...

# 启动（默认 SQLite，端口 8001）
go run cmd/server/main.go

# 默认管理员
# 账号: admin  密码: admin123
```

## 目录结构

```
cmd/server/          程序入口
configs/             配置文件
internal/
  bootstrap/         启动编排
  config/            配置结构体
  core/              核心组件（DB/JWT/Container/Logger）
  middleware/        中间件链
  model/             GORM 模型
  schema/            请求/响应 DTO
  dal/               数据访问层
  service/           业务逻辑层
  api/               HTTP 路由 + Handler
  mcp/               MCP 管理器
  plugin/            插件系统
  pkg/               公共工具包
uploads/             上传文件目录
```

## License

MIT

> AI生成