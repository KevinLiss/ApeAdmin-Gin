package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/utils"
	"apeadmin-gin/internal/service"

	"gorm.io/gorm"
)

// ─── LLM 响应辅助 ───

func getRespMessage(resp map[string]interface{}) map[string]interface{} {
	choices := getSlice(resp, "choices")
	if len(choices) == 0 {
		return map[string]interface{}{}
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return getMap(choice, "message")
}

func getToolCalls(msg map[string]interface{}) []map[string]interface{} {
	tcs := getSlice(msg, "tool_calls")
	result := make([]map[string]interface{}, 0, len(tcs))
	for _, tc := range tcs {
		if m, ok := tc.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

func getSlice(v map[string]interface{}, key string) []interface{} {
	if v == nil {
		return nil
	}
	if s, ok := v[key].([]interface{}); ok {
		return s
	}
	return nil
}

func getMap(v map[string]interface{}, key string) map[string]interface{} {
	if v == nil {
		return map[string]interface{}{}
	}
	if m, ok := v[key].(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func getString(v map[string]interface{}, key string) string {
	if v == nil {
		return ""
	}
	if s, ok := v[key].(string); ok {
		return s
	}
	return ""
}

func getFloat(v map[string]interface{}, key string) float64 {
	if v == nil {
		return 0
	}
	switch n := v[key].(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func orNil(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// defaultModel 供应商默认模型
func defaultModel(providerType string) string {
	switch providerType {
	case "deepseek":
		return "deepseek-chat"
	case "qwen":
		return "qwen-plus"
	case "glm":
		return "glm-4-flash"
	case "openai":
		return "gpt-4o-mini"
	default:
		return ""
	}
}

// getCurrentUserPerms 当前用户权限集合
func getCurrentUserPerms(c *gin.Context) map[string]bool {
	userVal, exists := c.Get("user")
	if !exists {
		return map[string]bool{}
	}
	u, ok := userVal.(*model.SysUser)
	if !ok {
		return map[string]bool{}
	}
	if u.IsSuperAdmin {
		return map[string]bool{"*": true}
	}
	return service.GetUserPermissions(u.ID)
}

// streamLLMRequest 发起流式 LLM 请求，返回响应体（调用方负责 Close）
func streamLLMRequest(llmMessages []map[string]interface{}, baseURL, apiKey, model string, tools []map[string]interface{}) (*http.Response, error) {
	body := map[string]interface{}{
		"model":       model,
		"messages":    llmMessages,
		"max_tokens":  2000,
		"temperature": 0.7,
		"stream":      true,
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	payload, _ := json.Marshal(body)
	url := strings.TrimRight(baseURL, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("模型请求失败: %v", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(bodyBytes))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return nil, fmt.Errorf("模型接口返回 %d: %s", resp.StatusCode, msg)
	}
	return resp, nil
}

// executeAITool 执行 AI 工具（带权限校验），返回 JSON 字符串
func executeAITool(toolName string, args interface{}, userPerms map[string]bool, c *gin.Context) string {
	// 权限校验
	required := aiToolPermissions[toolName]
	allowed := false
	if len(required) == 0 {
		allowed = true
	}
	if userPerms["*"] {
		allowed = true
	}
	if !allowed {
		for _, p := range required {
			if userPerms[p] {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return toJSON(map[string]string{"error": "权限不足，无法调用工具: " + toolName})
	}

	var argsMap map[string]interface{}
	switch v := args.(type) {
	case map[string]interface{}:
		argsMap = v
	case string:
		_ = json.Unmarshal([]byte(v), &argsMap)
	}

	db := core.GetDB()
	switch toolName {
	case "get_user_list":
		page := intArg(argsMap, "page", 1)
		pageSize := intArg(argsMap, "page_size", 10)
		keyword := strArg(argsMap, "keyword")
		return getAIUserList(db, page, pageSize, keyword)
	case "get_user_detail":
		return getAIUserDetail(db, uintArg(argsMap, "user_id"))
	case "create_user":
		return createAIUser(db, argsMap)
	case "update_user":
		return updateAIUser(db, argsMap)
	case "delete_user":
		return deleteAIUser(db, uintArg(argsMap, "user_id"))
	case "get_system_stats":
		return getAISystemStats(db)
	case "system_health_check", "role_list", "role_create":
		// 透传 MCP 工具
		mgr := core.GetMCPManager()
		if mgr != nil {
			if tool, ok := mgr.GetTool(toolName); ok {
				result, err := tool.Handler(argsMap)
				if err != nil {
					return toJSON(map[string]string{"error": err.Error()})
				}
				return toJSON(result)
			}
		}
		return toJSON(map[string]string{"error": "MCP 工具不可用: " + toolName})
	default:
		// MCP 工具
		mgr := core.GetMCPManager()
		if mgr != nil {
			if tool, ok := mgr.GetTool(toolName); ok {
				// MCP 工具调用需要 mcp:tools:call 权限
				if !userPerms["*"] && !userPerms["mcp:tools:call"] {
					return toJSON(map[string]string{"error": "权限不足，无法调用 MCP 工具: " + toolName})
				}
				result, err := tool.Handler(argsMap)
				if err != nil {
					return toJSON(map[string]string{"error": err.Error()})
				}
				// 写审计日志
				if c != nil {
					userID := c.GetUint("user_id")
					username := c.GetString("username")
					argsJSON, _ := json.Marshal(argsMap)
					resultJSON, _ := json.Marshal(result)
					mgr.WriteAuditLog(userID, username, "tool", toolName, string(argsJSON), string(resultJSON), "success", 0)
				}
				return toJSON(result)
			}
		}
		return toJSON(map[string]string{"error": "未知工具: " + toolName})
	}
}

// ─── 工具处理函数 ───

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func toInt(args map[string]interface{}, key string, def int) int {
	if args == nil {
		return def
	}
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		if n > 0 {
			return n
		}
	}
	return def
}

func strArg(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	if s, ok := args[key].(string); ok {
		return s
	}
	return ""
}

func uintArg(args map[string]interface{}, key string) uint {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case float64:
		return uint(v)
	case int:
		return uint(v)
	case string:
		n, _ := strconv.ParseUint(v, 10, 64)
		return uint(n)
	}
	return 0
}

// intArg 读取整数参数（默认值）
func intArg(args map[string]interface{}, key string, def int) int {
	if args == nil {
		return def
	}
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		if n > 0 {
			return n
		}
	}
	return def
}

// getAIUserList 查询用户列表
func getAIUserList(db *gorm.DB, page, pageSize int, keyword string) string {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	users, total, err := dal.ListUsers(page, pageSize, keyword)
	if err != nil {
		return toJSON(map[string]string{"error": "查询用户失败: " + err.Error()})
	}
	items := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		roles := make([]map[string]interface{}, 0, len(u.Roles))
		for _, r := range u.Roles {
			roles = append(roles, map[string]interface{}{"id": r.ID, "name": r.Name, "code": r.Code})
		}
		items = append(items, map[string]interface{}{
			"id":       u.ID,
			"username": u.Username,
			"nickname": u.Nickname,
			"email":    u.Email,
			"phone":    u.Phone,
			"status":   u.Status,
			"roles":    roles,
		})
	}
	return toJSON(map[string]interface{}{"total": total, "page": page, "page_size": pageSize, "items": items})
}

func getUserIDDetail(db *gorm.DB, userID uint) string {
	user, err := dal.GetUserWithRoles(userID)
	if err != nil {
		return toJSON(map[string]string{"error": "用户不存在"})
	}
	roles := make([]map[string]interface{}, 0, len(user.Roles))
	for _, r := range user.Roles {
		roles = append(roles, map[string]interface{}{"id": r.ID, "name": r.Name, "code": r.Code})
	}
	return toJSON(map[string]interface{}{
		"id": user.ID, "username": user.Username, "nickname": user.Nickname,
		"email": user.Email, "phone": user.Phone, "status": user.Status,
		"dept_id": user.DeptID, "roles": roles,
	})
}

func getUserDetailAlias(db *gorm.DB, userID uint) string {
	return getUserIDDetail(db, userID)
}

// getAIUserDetail 查询用户详情
func getAIUserDetail(db *gorm.DB, userID uint) string {
	return getUserIDDetail(db, userID)
}

func createAIUser(db *gorm.DB, args map[string]interface{}) string {
	username := strArg(args, "username")
	password := strArg(args, "password")
	if username == "" || password == "" {
		return toJSON(map[string]string{"error": "用户名和密码不能为空"})
	}
	existing, _ := dal.GetUserByUsername(username)
	if existing != nil {
		return toJSON(map[string]string{"error": "用户名已存在"})
	}
	hash, err := utils.HashPassword(password)
	if err != nil {
		return toJSON(map[string]string{"error": "密码加密失败"})
	}
	user := model.SysUser{
		Username: username,
		Nickname: strArg(args, "nickname"),
		Password: hash,
		Email:    strArg(args, "email"),
		Phone:    strArg(args, "phone"),
		Status:   1,
	}
	if user.Nickname == "" {
		user.Nickname = username
	}
	if err := dal.CreateUser(&user); err != nil {
		return toJSON(map[string]string{"error": "创建用户失败: " + err.Error()})
	}
	return toJSON(map[string]interface{}{"id": user.ID, "username": user.Username, "message": "用户创建成功"})
}

func updateAIUser(db *gorm.DB, args map[string]interface{}) string {
	userID := uintArg(args, "user_id")
	if userID == 0 {
		return toJSON(map[string]string{"error": "缺少参数: user_id"})
	}
	user, err := dal.GetUserByID(userID)
	if err != nil {
		return toJSON(map[string]string{"error": "用户不存在"})
	}
	updates := map[string]interface{}{}
	if v := strArg(args, "nickname"); v != "" {
		updates["nickname"] = v
	}
	if v := strArg(args, "email"); v != "" {
		updates["email"] = v
	}
	if v := strArg(args, "phone"); v != "" {
		updates["phone"] = v
	}
	if v, ok := args["status"].(float64); ok {
		updates["status"] = int(v)
	}
	if len(updates) > 0 {
		core.GetDB().Model(&model.SysUser{}).Where("id = ?", user.ID).Updates(updates)
	}
	return toJSON(map[string]interface{}{"id": user.ID, "message": "用户更新成功"})
}

func deleteAIUser(db *gorm.DB, userID uint) string {
	if userID == 0 {
		return toJSON(map[string]string{"error": "缺少参数: user_id"})
	}
	if err := dal.DeleteUser(userID); err != nil {
		return toJSON(map[string]string{"error": "删除用户失败: " + err.Error()})
	}
	return toJSON(map[string]string{"message": "用户已删除"})
}

func getAISystemStats(db *gorm.DB) string {
	var userCount, roleCount, menuCount, deptCount int64
	db.Model(&model.SysUser{}).Count(&userCount)
	db.Model(&model.SysRole{}).Count(&roleCount)
	db.Model(&model.SysMenu{}).Where("status = ?", 1).Count(&menuCount)
	db.Model(&model.SysDept{}).Count(&deptCount)
	return toJSON(map[string]interface{}{
		"user_count": userCount,
		"role_count": roleCount,
		"menu_count": menuCount,
		"dept_count": deptCount,
	})
}

// runChatOnce 非流式单次对话
func runChatOnce(provider *model.SysAiProvider, model string, messages []map[string]string, enableTools bool, c *gin.Context) (map[string]interface{}, error) {
	return chatNonStreamOnce(provider, model, messages, enableTools, c)
}