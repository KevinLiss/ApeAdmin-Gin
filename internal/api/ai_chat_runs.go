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

// ─────────────────────────────────────────────────────────────────────────
// AI 后台运行任务路由：入队 / SSE 订阅 / 查询 / 停止 / 活跃 run
//
// run 在后台 worker 中执行（见 ai_runner.go），生命周期独立于 HTTP 请求：
// 关闭页面或切换路由不会中断生成，订阅端可断线重连续流。
// ─────────────────────────────────────────────────────────────────────────

// CreateChatRun 入队一条对话 run（立即返回 run_id + session_id）
// 若会话已有 pending/running run，新问题排队（FIFO）而非被拒绝。
func (h *AiHandler) CreateChatRun(c *gin.Context) {
	var req struct {
		SessionID   *uint  `json:"session_id"`
		Question    string `json:"question" binding:"required"`
		ProviderID  uint   `json:"provider_id"`
		Model       string `json:"model"`
		EnableTools *bool  `json:"enable_tools"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	if runes := []rune(question); len(runes) > 8000 {
		question = string(runes[:8000])
	}

	userID := c.GetUint("user_id")
	db := core.GetDB()

	// 解析/创建会话（服务端校验归属）
	var session model.SysChatSession
	if req.SessionID != nil && *req.SessionID > 0 {
		if err := db.Where("id = ? AND user_id = ?", *req.SessionID, userID).First(&session).Error; err != nil {
			c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "ai.session_missing", nil))
			return
		}
	} else {
		session = model.SysChatSession{UserID: userID, Title: "新对话"}
		if err := db.Create(&session).Error; err != nil {
			c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, err.Error()))
			return
		}
	}

	provider, err := resolveProvider(req.ProviderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Body{Code: 400, Msg: err.Error()})
		return
	}

	enableTools := true
	if req.EnableTools != nil {
		enableTools = *req.EnableTools
	}

	run, title, err := enqueueChatRun(userID, session.ID, provider.ID, req.Model, question, enableTools)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"run_id":      run.ID,
		"session_id":  session.ID,
		"status":      run.Status,
		"provider_id": provider.ID,
		"model":       req.Model,
		"title":       title,
	}))
}

// ChatRunEvents run 事件 SSE 订阅（快照 + 增量 + 心跳）
// 生成与订阅解耦：重连时带 seen_events 续传，不带则重新取快照。
func (h *AiHandler) ChatRunEvents(c *gin.Context) {
	runID64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	seen, _ := strconv.Atoi(c.DefaultQuery("seen_events", "0"))
	if seen < 0 {
		seen = 0
	}
	userID := c.GetUint("user_id")

	var run model.SysChatRun
	if err := core.GetDB().Where("id = ? AND user_id = ?", runID64, userID).First(&run).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "ai.session_missing", nil))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "SSE 不可用")
		return
	}
	send := func(payload interface{}) {
		data, err := json.Marshal(payload)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}

	buf := getRunBuffer(run.ID)
	if buf == nil {
		// 无实时缓冲（进程重启/终态缓冲已回收）：回放落库状态后结束
		send(runStateFromModel(&run))
		return
	}

	// 初始快照：保护已交付内容（客户端据此对齐本地状态）
	buf.mu.Lock()
	snap := map[string]interface{}{
		"type":        "snapshot",
		"run_id":      buf.runID,
		"session_id":  buf.sessionID,
		"status":      buf.status,
		"answer":      buf.answer,
		"tool_events": toIfaceSlice(buf.toolEvents),
		"error":       buf.errMsg,
	}
	buf.mu.Unlock()
	send(snap)

	ctx := c.Request.Context()
	offset := seen
	for {
		buf.mu.Lock()
		total := len(buf.events)
		var pending []string
		if offset < total {
			pending = append([]string(nil), buf.events[offset:]...)
		}
		status := buf.status
		notify := buf.notify
		buf.mu.Unlock()

		for _, ev := range pending {
			fmt.Fprintf(c.Writer, "data: %s\n\n", ev)
		}
		if len(pending) > 0 {
			offset += len(pending)
			flusher.Flush()
		}
		if runTerminalStatuses[status] {
			return
		}
		select {
		case <-notify:
		case <-time.After(20 * time.Second):
			// 心跳注释（SSE 注释行，不触发客户端事件）
			fmt.Fprint(c.Writer, ": keep-alive\n\n")
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

// GetChatRun 轮询查询 run 当前状态（SSE 不可用时的兜底）
func (h *AiHandler) GetChatRun(c *gin.Context) {
	runID64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID := c.GetUint("user_id")
	var run model.SysChatRun
	if err := core.GetDB().Where("id = ? AND user_id = ?", runID64, userID).First(&run).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "ai.session_missing", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(runStateFromModel(&run)))
}

// StopChatRun 停止 pending/running run（已生成内容保留）
func (h *AiHandler) StopChatRun(c *gin.Context) {
	runID64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID := c.GetUint("user_id")
	var run model.SysChatRun
	if err := core.GetDB().Where("id = ? AND user_id = ?", runID64, userID).First(&run).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "ai.session_missing", nil))
		return
	}
	if runTerminalStatuses[run.Status] {
		c.JSON(http.StatusOK, response.Success(map[string]interface{}{
			"run_id": run.ID, "status": run.Status,
		}))
		return
	}
	updated := stopChatRunRequest(&run)
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"run_id": updated.ID, "status": updated.Status,
	}))
}

// ActiveChatRun 会话当前 pending/running run（含部分回答与排队数）
// GET /ai/chat/active-run?session_id=N
func (h *AiHandler) ActiveChatRun(c *gin.Context) {
	sessionID64, _ := strconv.ParseUint(c.Query("session_id"), 10, 64)
	if sessionID64 == 0 {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	userID := c.GetUint("user_id")
	var session model.SysChatSession
	if err := core.GetDB().Where("id = ? AND user_id = ?", sessionID64, userID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "ai.session_missing", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(getActiveChatRunState(uint(sessionID64), userID)))
}