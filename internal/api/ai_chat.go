package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/response"
)

// marshalJSON 序列化辅助
func marshalJSON(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

// jsonUnmarshal 反序列化辅助
func jsonUnmarshal(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

// sessionToDict 会话转前端结构
func sessionToDict(s *model.SysChatSession, messageCount int64) map[string]interface{} {
	return map[string]interface{}{
		"id":            s.ID,
		"title":         s.Title,
		"provider_id":   s.ProviderID,
		"model":         s.Model,
		"message_count": messageCount,
		"created_at":    s.CreatedAt.Format(time.RFC3339),
		"updated_at":    s.UpdatedAt.Format(time.RFC3339),
	}
}

// ListSessions 当前用户会话列表（最新在前）
func (h *AiHandler) ListSessions(c *gin.Context) {
	userID := c.GetUint("user_id")
	db := core.GetDB()
	var sessions []model.SysChatSession
	db.Where("user_id = ?", userID).Order("id DESC").Find(&sessions)
	items := make([]map[string]interface{}, 0, len(sessions))
	for i := range sessions {
		var count int64
		db.Model(&model.SysChatMessage{}).Where("session_id = ?", sessions[i].ID).Count(&count)
		items = append(items, sessionToDict(&sessions[i], count))
	}
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{"total": len(items), "items": items}))
}

// CreateSession 新建会话
func (h *AiHandler) CreateSession(c *gin.Context) {
	var req struct {
		Title string `json:"title"`
	}
	_ = c.ShouldBindJSON(&req)
	userID := c.GetUint("user_id")
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "新对话"
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	db := core.GetDB()
	s := model.SysChatSession{UserID: userID, Title: title}
	db.Create(&s)
	c.JSON(http.StatusOK, response.Success(sessionToDict(&s, 0)))
}

// GetSessionMessages 获取会话消息（时间正序）
func (h *AiHandler) GetSessionMessages(c *gin.Context) {
	sessionID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID := c.GetUint("user_id")
	db := core.GetDB()
	var s model.SysChatSession
	if err := db.Where("id = ? AND user_id = ?", sessionID, userID).First(&s).Error; err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "会话不存在"))
		return
	}
	var msgs []model.SysChatMessage
	db.Where("session_id = ?", s.ID).Order("id ASC").Find(&msgs)
	items := make([]map[string]interface{}, 0, len(msgs))
	for _, m := range msgs {
		var toolEvents interface{}
		if m.ToolEvents != "" {
			var te []interface{}
			if jsonUnmarshal(m.ToolEvents, &te) == nil {
				toolEvents = te
			}
		}
		items = append(items, map[string]interface{}{
			"id":          m.ID,
			"session_id":  m.SessionID,
			"role":        m.Role,
			"content":     m.Content,
			"tool_events": toolEvents,
			"created_at":  m.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"session":  sessionToDict(&s, int64(len(msgs))),
		"messages": items,
	}))
}

// RenameSession 重命名会话
func (h *AiHandler) RenameSession(c *gin.Context) {
	sessionID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID := c.GetUint("user_id")
	var req struct {
		Title string `json:"title" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	db := core.GetDB()
	var s model.SysChatSession
	if err := db.Where("id = ? AND user_id = ?", sessionID, userID).First(&s).Error; err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "会话不存在"))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "新对话"
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	s.Title = title
	db.Save(&s)
	c.JSON(http.StatusOK, response.Success(sessionToDict(&s, 0)))
}

// DeleteSession 删除会话及其消息
func (h *AiHandler) DeleteSession(c *gin.Context) {
	sessionID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID := c.GetUint("user_id")
	db := core.GetDB()
	var s model.SysChatSession
	if err := db.Where("id = ? AND user_id = ?", sessionID, userID).First(&s).Error; err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "会话不存在"))
		return
	}
	db.Where("session_id = ?", s.ID).Delete(&model.SysChatMessage{})
	db.Delete(&s)
	c.JSON(http.StatusOK, response.SuccessMsg("会话已删除"))
}

// AppendMessage 追加消息到会话
func (h *AiHandler) AppendMessage(c *gin.Context) {
	sessionID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID := c.GetUint("user_id")
	var req struct {
		Role       string        `json:"role" binding:"required"`
		Content    string        `json:"content"`
		ToolEvents []interface{} `json:"tool_events"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	db := core.GetDB()
	var s model.SysChatSession
	if err := db.Where("id = ? AND user_id = ?", sessionID, userID).First(&s).Error; err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "会话不存在"))
		return
	}
	if req.Role != "user" && req.Role != "assistant" {
		c.JSON(http.StatusBadRequest, response.Error(400, "role 必须为 user 或 assistant"))
		return
	}
	toolEventsJSON := ""
	if req.ToolEvents != nil {
		toolEventsJSON, _ = marshalJSON(req.ToolEvents)
	}
	msg := model.SysChatMessage{
		SessionID:  s.ID,
		Role:       req.Role,
		Content:    req.Content,
		ToolEvents: toolEventsJSON,
	}
	db.Create(&msg)
	// 首条用户消息自动更新标题
	if req.Role == "user" && s.Title == "新对话" && req.Content != "" {
		title := strings.TrimSpace(req.Content)
		if len([]rune(title)) > 50 {
			title = string([]rune(title)[:50])
		}
		s.Title = title
		db.Save(&s)
	}
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"id":         msg.ID,
		"session_id": msg.SessionID,
		"role":       msg.Role,
		"content":    msg.Content,
		"created_at": msg.CreatedAt.Format(time.RFC3339),
	}))
}

// defaultBaseURL 供应商默认 API 地址
func defaultBaseURL(providerType string) string {
	switch providerType {
	case "deepseek":
		return "https://api.deepseek.com"
	case "qwen":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "glm":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "openai":
		return "https://api.openai.com/v1"
	default:
		return ""
	}
}

// fetchModels 调用供应商 /models 端点拉取模型列表
func fetchModels(baseURL, apiKey string) ([]string, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("未配置 API 地址")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("响应解析失败: %v", err)
	}
	models := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	return models, nil
}