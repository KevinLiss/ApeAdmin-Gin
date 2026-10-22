package core

import (
	"strings"
	"sync"
)

// ────────────────────────────────────────────────────────────────────────────
// i18n 国际化核心（后端消息）
//
// 三层结构：
//  1. 内置字典 builtinMessages（编译期兜底，保证无 DB 也能翻译）
//  2. DB 覆盖层（启动时从 sys_translation 加载，后台改文案热生效——见 ReloadMessages）
//  3. key 兜底（查不到时返回 key 本身，避免裸中文/空白）
//
// locale 协议：前端每个请求注入 Accept-Language 头（如 zh-CN / en-US），
// ParseLocale 解析后传给 T 翻译。
// ────────────────────────────────────────────────────────────────────────────

// CtxLocaleKey gin.Context 中存放当前请求语言的键（middleware 与 response 共用，避免循环依赖）
const CtxLocaleKey = "i18n_locale"

// builtinMessages 内置消息字典（zh-CN / en-US）
var builtinMessages = map[string]map[string]string{
	"zh-CN": {
		// 通用
		"common.param_error":     "参数错误",
		"common.internal_error":  "服务器内部错误",
		"common.create_ok":       "创建成功",
		"common.create_fail":     "创建失败",
		"common.update_ok":       "更新成功",
		"common.update_fail":     "更新失败",
		"common.delete_ok":      "删除成功",
		"common.delete_fail":    "删除失败",
		"common.query_fail":      "查询失败",
		"common.clear_ok":        "清空成功",
		"common.clear_fail":      "清空失败",
		"common.reset_fail":     "重置失败",
		"common.operation_fail": "操作失败",
		"common.not_found":       "{name}不存在",
		"common.exists":          "{name}已存在",
		"common.api_not_found":   "接口不存在",
		"common.config_missing":  "配置未初始化",
		"common.file_missing":   "缺少文件字段 file",

		// 认证
		"auth.not_logged_in":     "请先登录",
		"auth.login_expired":     "登录已过期，请重新登录",
		"auth.login_invalid":     "登录状态已失效，请重新登录",
		"auth.login_abnormal":    "登录状态异常",
		"auth.user_disabled":     "用户不存在或已禁用",
		"auth.user_not_found":    "用户不存在",
		"auth.login_too_many":    "登录失败次数过多，请稍后再试",
		"auth.rate_limited":      "请求过于频繁，请稍后再试",
		"auth.logout_ok":         "登出成功",
		"auth.logout_fail":       "登出失败",
		"auth.get_userinfo_fail": "获取用户信息失败",
		"auth.pwd_reset_ok":      "密码重置成功",
		"auth.pwd_change_ok":     "密码修改成功",
		"auth.wrong_credentials":  "用户名或密码错误",
		"auth.account_disabled":   "用户已禁用",
		"auth.old_pwd_error":      "原密码错误",
		"auth.refresh_invalid":    "refresh token 无效",

		// 用户
		"user.label":          "用户",
		"user.username_exists": "用户名已存在",
		"user.pwd_hash_fail":   "密码加密失败",

		// 角色
		"role.not_found":   "角色不存在",
		"role.code_exists": "角色编码已存在",

		// 部门
		"dept.parent_not_found": "上级部门不存在",
		"dept.parent_invalid":   "上级部门不能是自身或其子部门",
		"dept.has_children":    "存在子部门，无法删除",
		"dept.has_users":       "部门下存在用户，无法删除",
		"dept.name_exists":     "同级下已存在同名部门",

		// 日志
		"log.not_found": "日志不存在",

		// 设置
		"setting.get_fail":   "获取设置失败",
		"setting.save_ok":    "配置已保存",

		// 文件
		"file.image_only":     "仅支持图片格式（jpg/png/gif/webp/svg/ico）",
		"file.image_too_large": "图片大小不能超过 10MB",
		"file.save_fail":      "保存上传文件失败",
		"file.write_fail":     "保存文件失败",
		"file.mkdir_fail":     "创建目录失败",
		"file.tmpdir_fail":    "创建临时目录失败",

		// 插件
		"plugin.not_found":     "插件不存在",
		"plugin.zip_only":      "仅支持 .zip 格式插件包",
		"plugin.not_enabled":   "插件功能未启用",
		"plugin.restarting":    "服务正在重启...",

		// MCP
		"mcp.not_initialized":  "MCP 管理器未初始化",
		"mcp.tool_not_found":   "工具不存在",
		"mcp.call_fail":        "调用失败: {error}",
		"mcp.resource_fail":    "资源不存在或读取失败",
		"mcp.prompt_not_found": "提示词不存在",
		"mcp.uri_required":     "uri 参数必填",

		// AI
		"ai.provider_exists":   "供应商已存在",
		"ai.provider_missing":   "供应商不存在",
		"ai.provider_disabled":  "该模型供应商已被禁用",
		"ai.no_provider":        "未配置任何可用的模型供应商，请先在「模型密钥管理」中添加",
		"ai.key_encrypt_fail":   "密钥加密失败",
		"ai.key_decrypt_fail":   "密钥解密失败",
		"ai.base_url_required":  "未配置 API 地址",
		"ai.role_invalid":      "role 必须为 user 或 assistant",
		"ai.session_missing":   "会话不存在",
		"ai.session_deleted":    "会话已删除",

		// 权限
		"perm.denied": "无权限: {permission}",

		// 多语言管理
		"i18n.locale":              "语言",
		"i18n.translation":         "翻译条目",
		"i18n.default_cannot_delete": "默认语言不可删除",
		"i18n.code_required":        "语言编码必填",
	},
	"en-US": {
		// Common
		"common.param_error":     "Invalid parameters",
		"common.internal_error":  "Internal server error",
		"common.create_ok":       "Created successfully",
		"common.create_fail":     "Failed to create",
		"common.update_ok":       "Updated successfully",
		"common.update_fail":     "Failed to update",
		"common.delete_ok":       "Deleted successfully",
		"common.delete_fail":     "Failed to delete",
		"common.query_fail":      "Failed to query",
		"common.clear_ok":        "Cleared successfully",
		"common.clear_fail":      "Failed to clear",
		"common.reset_fail":      "Failed to reset",
		"common.operation_fail":  "Operation failed",
		"common.not_found":       "{name} not found",
		"common.exists":          "{name} already exists",
		"common.api_not_found":   "API endpoint not found",
		"common.config_missing":  "Configuration not initialized",
		"common.file_missing":    "Missing file field: file",

		// Auth
		"auth.not_logged_in":     "Please log in first",
		"auth.login_expired":     "Login expired, please log in again",
		"auth.login_invalid":     "Login session invalid, please log in again",
		"auth.login_abnormal":    "Abnormal login state",
		"auth.user_disabled":     "User does not exist or is disabled",
		"auth.user_not_found":    "User not found",
		"auth.login_too_many":    "Too many failed login attempts, please try again later",
		"auth.rate_limited":      "Too many requests, please try again later",
		"auth.logout_ok":         "Logged out successfully",
		"auth.logout_fail":       "Failed to log out",
		"auth.get_userinfo_fail": "Failed to get user info",
		"auth.pwd_reset_ok":      "Password reset successfully",
		"auth.pwd_change_ok":     "Password changed successfully",
		"auth.wrong_credentials": "Incorrect username or password",
		"auth.account_disabled":  "Account is disabled",
		"auth.old_pwd_error":     "Incorrect old password",
		"auth.refresh_invalid":   "Invalid refresh token",

		// User
		"user.label":          "user",
		"user.username_exists": "Username already exists",
		"user.pwd_hash_fail":   "Failed to hash password",

		// Role
		"role.not_found":   "Role not found",
		"role.code_exists": "Role code already exists",

		// Dept
		"dept.parent_not_found": "Parent department not found",
		"dept.parent_invalid":   "The parent department cannot be itself or one of its sub-departments",
		"dept.has_children":    "Cannot delete: sub-departments exist",
		"dept.has_users":       "Cannot delete: users are still assigned to this department",
		"dept.name_exists":     "A department with the same name already exists at this level",

		// Log
		"log.not_found": "Log not found",

		// Setting
		"setting.get_fail":  "Failed to get settings",
		"setting.save_ok":   "Configuration saved",

		// File
		"file.image_only":       "Only image formats are supported (jpg/png/gif/webp/svg/ico)",
		"file.image_too_large":  "Image size cannot exceed 10MB",
		"file.save_fail":        "Failed to save uploaded file",
		"file.write_fail":       "Failed to write file",
		"file.mkdir_fail":       "Failed to create directory",
		"file.tmpdir_fail":      "Failed to create temporary directory",

		// Plugin
		"plugin.not_found":   "Plugin not found",
		"plugin.zip_only":    "Only .zip plugin packages are supported",
		"plugin.not_enabled": "Plugin feature is not enabled",
		"plugin.restarting":  "Service is restarting...",

		// MCP
		"mcp.not_initialized":  "MCP manager not initialized",
		"mcp.tool_not_found":   "Tool not found",
		"mcp.call_fail":        "Call failed: {error}",
		"mcp.resource_fail":    "Resource not found or failed to read",
		"mcp.prompt_not_found": "Prompt not found",
		"mcp.uri_required":     "The uri parameter is required",

		// AI
		"ai.provider_exists":   "Provider already exists",
		"ai.provider_missing":   "Provider not found",
		"ai.provider_disabled":  "The model provider is disabled",
		"ai.no_provider":        "No model provider is configured. Please add one in Model Keys Management first.",
		"ai.key_encrypt_fail":   "Failed to encrypt the key",
		"ai.key_decrypt_fail":   "Failed to decrypt the key",
		"ai.base_url_required":  "API base URL is not configured",
		"ai.role_invalid":      "role must be user or assistant",
		"ai.session_missing":   "Session not found",
		"ai.session_deleted":   "Session deleted",

		// Permission
		"perm.denied": "Permission denied: {permission}",

		// I18n management
		"i18n.locale":                "locale",
		"i18n.translation":           "translation entry",
		"i18n.default_cannot_delete": "The default language cannot be deleted",
		"i18n.code_required":          "Locale code is required",
	},
}

// i18nState 运行时状态：DB 覆盖层 + 默认语言
type i18nState struct {
	mu           sync.RWMutex
	overrides    map[string]map[string]string // locale -> key -> value（DB 覆盖层）
	defaultLocale string                     // 默认语言（sys_locale.is_default=1）
}

var i18nS = &i18nState{
	overrides:     map[string]map[string]string{},
	defaultLocale: "zh-CN",
}

// ParseLocale 解析 Accept-Language 头，归一化为已支持的语言码
// 未携带/无法识别时返回默认语言
func ParseLocale(acceptLang string) string {
	if acceptLang == "" {
		return i18nS.defaultLocale
	}
	primary := strings.TrimSpace(strings.Split(acceptLang, ",")[0])
	primary = strings.Split(primary, ";")[0]
	primary = strings.TrimSpace(primary)

	i18nS.mu.RLock()
	defer i18nS.mu.RUnlock()
	// 精确匹配 DB 覆盖层（含后台新增的语言，如 ja-JP）
	if _, ok := i18nS.overrides[primary]; ok {
		return primary
	}
	// 前缀匹配（zh -> zh-CN，en -> en-US）
	switch {
	case strings.HasPrefix(primary, "zh"):
		return "zh-CN"
	case strings.HasPrefix(primary, "en"):
		return "en-US"
	}
	// 内置兜底
	if _, ok := builtinMessages[primary]; ok {
		return primary
	}
	return i18nS.defaultLocale
}

// T 翻译 key 到指定语言；params 用于 {name} 插值
// 查找顺序：DB 覆盖层 → 内置字典 → key 本身
func T(locale, key string, params ...map[string]string) string {
	if locale == "" {
		locale = i18nS.defaultLocale
	}

	var msg string
	i18nS.mu.RLock()
	if ov, ok := i18nS.overrides[locale]; ok {
		if v, ok := ov[key]; ok {
			msg = v
		}
	}
	i18nS.mu.RUnlock()

	if msg == "" {
		if dict, ok := builtinMessages[locale]; ok {
			msg = dict[key]
		}
		// 回退默认语言（zh-CN）
		if msg == "" && locale != "zh-CN" {
			if dict, ok := builtinMessages["zh-CN"]; ok {
				msg = dict[key]
			}
		}
	}
	if msg == "" {
		return key
	}

	// {name} 插值
	if len(params) > 0 {
		for k, v := range params[0] {
			msg = strings.ReplaceAll(msg, "{"+k+"}", v)
		}
	}
	return msg
}

// SetI18nOverrides 设置 DB 覆盖层 + 默认语言（启动加载 / 翻译保存后热刷新时调用）
func SetI18nOverrides(overrides map[string]map[string]string, defaultLocale string) {
	if defaultLocale == "" {
		defaultLocale = "zh-CN"
	}
	i18nS.mu.Lock()
	i18nS.overrides = overrides
	i18nS.defaultLocale = defaultLocale
	i18nS.mu.Unlock()
}

// DefaultLocale 当前默认语言
func DefaultLocale() string {
	i18nS.mu.RLock()
	defer i18nS.mu.RUnlock()
	return i18nS.defaultLocale
}
