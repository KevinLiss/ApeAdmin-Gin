package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/service"
)

// ─────────────────────────────────────────────────────────────────────────
// qm 式对话 run 执行器
//
// 与 Python 版 src/ai/runner.py 对齐：
// - 每条用户消息入队为一个 SysChatRun（pending），立即返回 run_id；
// - 每会话一个 worker 串行消费 pending（同会话 FIFO 排队，跨会话并行）；
// - LLM 执行与 HTTP 请求生命周期解耦：关页面/切路由不会中断生成；
// - 生成内容按 0.5s 节流落库，订阅端通过 SSE 快照 + 增量重放续流；
// - 停止分两态：pending 撤回未作答的用户消息；running 保留已生成内容。
// ─────────────────────────────────────────────────────────────────────────

const (
	runDBFlushInterval = 500 * time.Millisecond // 流式内容落库节流
	runContextWindow   = 40                     // 发送给 LLM 的最大历史条数
	runBufferTTL       = 10 * time.Minute       // 终态缓冲保留时长（供迟到订阅者取快照）
	runMaxToolRounds   = 5                      // 工具循环上限

	runClaimStale   = 90 * time.Second // 领取心跳超时阈值：超过视为执行实例已死
	runHeartbeat    = 15 * time.Second // 执行中心跳间隔
	runClaimerEvery = 500 * time.Millisecond // 全局领取器扫描间隔
)

var runTerminalStatuses = map[string]bool{"done": true, "failed": true, "stopped": true}

// runInstanceID 本实例标识（多实例部署下标识 run 归属；单实例恒为本机）
var runInstanceID = func() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s-%d-%d", host, os.Getpid(), time.Now().UnixNano()%100000)
}()

// ────────────────────────── 内存状态 ──────────────────────────

// RunBuffer 单个 run 的实时状态：SSE 事件缓冲 + 订阅唤醒通道
type RunBuffer struct {
	mu              sync.Mutex
	runID           uint
	sessionID       uint
	status          string
	answer          string
	toolEvents      []map[string]interface{}
	errMsg          string
	events          []string    // 已序列化的 SSE payload，下标即偏移量
	notify          chan struct{} // 广播通道：publish 时关闭并替换
	cancelRequested bool
	lastFlush       time.Time
}

func newRunBuffer(runID, sessionID uint) *RunBuffer {
	return &RunBuffer{
		runID:     runID,
		sessionID: sessionID,
		status:    "pending",
		notify:    make(chan struct{}),
	}
}

// publish 追加一个事件并唤醒所有订阅者
func (b *RunBuffer) publish(payload map[string]interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	b.mu.Lock()
	b.events = append(b.events, string(data))
	close(b.notify)
	b.notify = make(chan struct{})
	b.mu.Unlock()
}

func (b *RunBuffer) isCancelled() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cancelRequested
}

// appendAnswer 追加流式内容（按间隔节流落库）
func (b *RunBuffer) appendAnswer(chunk string, runID uint) {
	b.mu.Lock()
	b.answer += chunk
	answer := b.answer
	toolEvents := toolEventsJSON(b.toolEvents)
	flush := time.Since(b.lastFlush) >= runDBFlushInterval
	if flush {
		b.lastFlush = time.Now()
	}
	b.mu.Unlock()
	if flush {
		core.GetDB().Model(&model.SysChatRun{}).Where("id = ?", runID).
			Updates(map[string]interface{}{"answer": answer, "tool_events": toolEvents})
	}
}

// appendToolEvent 记录工具调用事件并落库
func (b *RunBuffer) appendToolEvent(ev map[string]interface{}, runID uint) {
	b.mu.Lock()
	b.toolEvents = append(b.toolEvents, ev)
	toolEvents := toolEventsJSON(b.toolEvents)
	b.mu.Unlock()
	core.GetDB().Model(&model.SysChatRun{}).Where("id = ?", runID).
		Updates(map[string]interface{}{"tool_events": toolEvents})
}

func toolEventsJSON(events []map[string]interface{}) string {
	if len(events) == 0 {
		return ""
	}
	data, err := json.Marshal(events)
	if err != nil {
		return ""
	}
	return string(data)
}

func toIfaceSlice(events []map[string]interface{}) []interface{} {
	out := make([]interface{}, 0, len(events))
	for _, e := range events {
		out = append(out, e)
	}
	return out
}

var (
	runBuffersMu sync.Mutex
	runBuffers   = map[uint]*RunBuffer{}

	runWorkersMu sync.Mutex
	runWorkers   = map[uint]context.CancelFunc{} // sessionID -> cancel
	runWorkersWG sync.WaitGroup
)

func putRunBuffer(runID uint, buf *RunBuffer) {
	runBuffersMu.Lock()
	runBuffers[runID] = buf
	runBuffersMu.Unlock()
}

func getRunBuffer(runID uint) *RunBuffer {
	runBuffersMu.Lock()
	defer runBuffersMu.Unlock()
	return runBuffers[runID]
}

func removeRunBuffer(runID uint) {
	runBuffersMu.Lock()
	delete(runBuffers, runID)
	runBuffersMu.Unlock()
}

// ────────────────────────── worker ──────────────────────────

// runClaimStaleAt 心跳超时判定时间点
func runClaimStaleAt() time.Time {
	return time.Now().Add(-runClaimStale)
}

// claimRun 原子抢占：仅当 run 无主/本机持有/心跳超时 且仍为 pending 时成功。
// 多实例部署下保证同一 run 只被一个实例执行；单实例下恒成功。
func claimRun(runID uint) bool {
	res := core.GetDB().Model(&model.SysChatRun{}).
		Where("id = ? AND status = 'pending' AND (claimed_by = '' OR claimed_by IS NULL OR claimed_by = ? OR claimed_at IS NULL OR claimed_at < ?)",
			runID, runInstanceID, runClaimStaleAt()).
		Updates(map[string]interface{}{"claimed_by": runInstanceID, "claimed_at": time.Now()})
	return res.Error == nil && res.RowsAffected > 0
}

// releaseClaim 释放本实例持有的 pending 领取标记（优雅停机时调用，其他实例可接管）
func releaseOwnPendingClaims() {
	core.GetDB().Model(&model.SysChatRun{}).
		Where("claimed_by = ? AND status = 'pending'", runInstanceID).
		Updates(map[string]interface{}{"claimed_by": "", "claimed_at": nil})
}

// StartRunClaimer 全局领取器：周期扫描未被认领/心跳超时的 pending run 并抢占执行。
// 多实例部署的兜底接管路径（入队实例的 fast path 是 enqueue→startSessionWorker）。
func StartRunClaimer(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(runClaimerEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			var runs []model.SysChatRun
			core.GetDB().Where("status = 'pending' AND (claimed_by = '' OR claimed_by IS NULL OR claimed_at IS NULL OR claimed_at < ?)",
				runClaimStaleAt()).Order("id ASC").Limit(10).Find(&runs)
			for _, r := range runs {
				if ctx.Err() != nil {
					return
				}
				if claimRun(r.ID) {
					startSessionWorker(r.SessionID)
				}
			}
		}
	}()
}

// startSessionWorker 确保会话有且仅有一个 worker（幂等）
func startSessionWorker(sessionID uint) {
	runWorkersMu.Lock()
	if _, exists := runWorkers[sessionID]; exists {
		runWorkersMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	runWorkers[sessionID] = cancel
	runWorkersWG.Add(1)
	runWorkersMu.Unlock()
	go sessionWorker(ctx, sessionID)
}

// sessionWorker 单会话串行执行器：FIFO 消费 pending run（执行前原子抢占）
func sessionWorker(ctx context.Context, sessionID uint) {
	defer runWorkersWG.Done()
	defer func() {
		runWorkersMu.Lock()
		delete(runWorkers, sessionID)
		runWorkersMu.Unlock()
		// 兜底：退出瞬间可能又有新的 pending 入队，重新拉起
		if ctx.Err() == nil {
			var pending int64
			core.GetDB().Model(&model.SysChatRun{}).
				Where("session_id = ? AND status = ?", sessionID, "pending").Count(&pending)
			if pending > 0 {
				startSessionWorker(sessionID)
			}
		}
	}()

	for {
		if ctx.Err() != nil {
			return
		}
		var run model.SysChatRun
		res := core.GetDB().
			Where("session_id = ? AND status = ?", sessionID, "pending").
			Where("claimed_by = '' OR claimed_by IS NULL OR claimed_by = ? OR claimed_at IS NULL OR claimed_at < ?",
				runInstanceID, runClaimStaleAt()).
			Order("id ASC").Limit(1).Find(&run)
		if res.Error != nil || res.RowsAffected == 0 {
			return // 无更多排队任务
		}
		if !claimRun(run.ID) {
			// 被其他实例抢先：跳过，继续找下一个
			continue
		}
		if getRunBuffer(run.ID) == nil {
			// 入队缓冲已丢失（排队中被撤销）：直接落终态
			now := time.Now()
			core.GetDB().Model(&model.SysChatRun{}).
				Where("id = ? AND status = ?", run.ID, "pending").
				Updates(map[string]interface{}{"status": "stopped", "error": "任务已取消", "finished_at": now})
			continue
		}
		executeRun(ctx, run.ID)
	}
}

// executeRun 执行单个 run（持有独立 goroutine）
func executeRun(ctx context.Context, runID uint) {
	buf := getRunBuffer(runID)
	if buf == nil {
		return
	}
	db := core.GetDB()

	var run model.SysChatRun
	if err := db.First(&run, runID).Error; err != nil {
		removeRunBuffer(runID)
		return
	}
	if run.Status != "pending" && run.Status != "running" {
		removeRunBuffer(runID)
		return
	}

	// 供应商可用性
	var provider model.SysAiProvider
	if run.ProviderID == 0 || db.First(&provider, run.ProviderID).Error != nil || !provider.Enabled {
		finishChatRun(&run, buf, "failed", "模型供应商不可用或已停用")
		return
	}

	now := time.Now()
	db.Model(&model.SysChatRun{}).Where("id = ?", run.ID).
		Updates(map[string]interface{}{"status": "running", "started_at": now})
	run.Status = "running"
	run.StartedAt = &now
	buf.mu.Lock()
	buf.status = "running"
	buf.mu.Unlock()
	buf.publish(map[string]interface{}{"type": "run_start", "status": "running"})

	// 领取心跳：执行期间定期刷新 claimed_at；实例崩溃后其他实例可在超时后接管
	hbStop := make(chan struct{})
	defer close(hbStop)
	go func() {
		ticker := time.NewTicker(runHeartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-hbStop:
				return
			case <-ticker.C:
				core.GetDB().Model(&model.SysChatRun{}).
					Where("id = ? AND claimed_by = ?", run.ID, runInstanceID).
					Update("claimed_at", time.Now())
			}
		}
	}()

	// 用户权限（后台执行无 gin.Context，按 user_id 重算，与流式对话一致）
	var usr model.SysUser
	userPerms := map[string]bool{}
	username := ""
	if err := db.First(&usr, run.UserID).Error; err == nil {
		username = usr.Username
		if usr.IsSuperAdmin {
			userPerms["*"] = true
		} else {
			userPerms = service.GetUserPermissionsCached(usr.ID)
		}
	}
	audit := &toolAuditIdentity{UserID: run.UserID, Username: username}

	apiKey, err := getAPIKey(&provider)
	if err != nil {
		finishChatRun(&run, buf, "failed", "API Key 解密失败")
		return
	}
	modelName := run.Model
	if modelName == "" {
		modelName = defaultModel(provider.ProviderType)
	}
	messages := buildRunContext(run.SessionID, run.Question)
	llmMessages, tools := buildLLMMessages(messages, run.EnableTools)

	status, errMsg := runLLMLoop(ctx, buf, runID, llmMessages, tools,
		providerBaseURL(&provider), apiKey, modelName, userPerms, audit)
	finishChatRun(&run, buf, status, errMsg)
}

// buildRunContext 从已落库消息组装 LLM 上下文（以服务端为唯一事实来源）
func buildRunContext(sessionID uint, question string) []map[string]string {
	var rows []model.SysChatMessage
	core.GetDB().Where("session_id = ?", sessionID).Order("id DESC").Limit(runContextWindow).Find(&rows)
	// 反转为时间正序
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	history := make([]map[string]string, 0, len(rows)+1)
	for _, m := range rows {
		if strings.TrimSpace(m.Content) == "" && m.Role != "user" {
			continue
		}
		history = append(history, map[string]string{"role": m.Role, "content": m.Content})
	}
	// 对话不能以 assistant 结尾（本轮问题须为最后一条）
	for len(history) > 0 && history[len(history)-1]["role"] == "assistant" {
		history = history[:len(history)-1]
	}
	if len(history) == 0 || history[len(history)-1]["content"] != question {
		history = append(history, map[string]string{"role": "user", "content": question})
	}
	return history
}

// runLLMLoop 与 streamChat 相同的工具循环，但输出写入 RunBuffer 并响应取消
func runLLMLoop(
	ctx context.Context,
	buf *RunBuffer,
	runID uint,
	llmMessages []map[string]interface{},
	tools []map[string]interface{},
	baseURL, apiKey, modelName string,
	userPerms map[string]bool,
	audit *toolAuditIdentity,
) (string, string) {
	for round := 0; round < runMaxToolRounds; round++ {
		if buf.isCancelled() {
			return "stopped", ""
		}
		if ctx.Err() != nil {
			return "failed", "服务关闭，本轮生成中断，请重新发送"
		}

		collectedContent := ""
		collectedToolCalls := []map[string]interface{}{}
		hasToolCalls := false

		resp, err := streamLLMRequest(ctx, llmMessages, baseURL, apiKey, modelName, tools)
		if err != nil {
			if buf.isCancelled() {
				return "stopped", ""
			}
			if ctx.Err() != nil {
				return "failed", "服务关闭，本轮生成中断，请重新发送"
			}
			return "failed", err.Error()
		}
		reader := bufio.NewReader(resp.Body)
		for {
			if buf.isCancelled() {
				resp.Body.Close()
				return "stopped", ""
			}
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			chunkStr := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if chunkStr == "[DONE]" {
				break
			}
			var chunk map[string]interface{}
			if json.Unmarshal([]byte(chunkStr), &chunk) != nil {
				continue
			}
			choices := getSlice(chunk, "choices")
			if len(choices) == 0 {
				continue
			}
			choice, ok := choices[0].(map[string]interface{})
			if !ok {
				continue
			}
			delta := getMap(choice, "delta")
			if content := getString(delta, "content"); content != "" {
				collectedContent += content
				buf.appendAnswer(content, runID)
				buf.publish(map[string]interface{}{"type": "content", "content": content})
			}
			if tcs := getSlice(delta, "tool_calls"); len(tcs) > 0 {
				hasToolCalls = true
				for _, tcI := range tcs {
					tc, ok := tcI.(map[string]interface{})
					if !ok {
						continue
					}
					idx := int(getFloat(tc, "index"))
					for len(collectedToolCalls) <= idx {
						collectedToolCalls = append(collectedToolCalls, map[string]interface{}{
							"id": "", "type": "function",
							"function": map[string]interface{}{"name": "", "arguments": ""},
						})
					}
					if id := getString(tc, "id"); id != "" {
						collectedToolCalls[idx]["id"] = id
					}
					fn := getMap(tc, "function")
					if name := getString(fn, "name"); name != "" {
						collectedToolCalls[idx]["function"].(map[string]interface{})["name"] = name
					}
					if args := getString(fn, "arguments"); args != "" {
						collectedToolCalls[idx]["function"].(map[string]interface{})["arguments"] = args
					}
				}
			}
		}
		resp.Body.Close()

		if buf.isCancelled() {
			return "stopped", ""
		}
		if ctx.Err() != nil {
			return "failed", "服务关闭，本轮生成中断，请重新发送"
		}
		if !hasToolCalls {
			return "done", ""
		}

		// 补全 tool_calls 字段
		for idx, tc := range collectedToolCalls {
			if getString(tc, "id") == "" {
				tc["id"] = fmt.Sprintf("call_%d_%d", round, idx)
			}
			fn := tc["function"].(map[string]interface{})
			if getString(fn, "arguments") == "" {
				fn["arguments"] = "{}"
			}
		}

		llmMessages = append(llmMessages, map[string]interface{}{
			"role":       "assistant",
			"content":    orNil(collectedContent),
			"tool_calls": collectedToolCalls,
		})

		for _, tc := range collectedToolCalls {
			if buf.isCancelled() {
				return "stopped", ""
			}
			fn := tc["function"].(map[string]interface{})
			fnName := getString(fn, "name")
			argsStr := getString(fn, "arguments")
			var args map[string]interface{}
			if json.Unmarshal([]byte(argsStr), &args) != nil {
				args = map[string]interface{}{}
			}
			buf.appendToolEvent(map[string]interface{}{"type": "tool_call", "name": fnName, "arguments": args}, runID)
			buf.publish(map[string]interface{}{"type": "tool_call", "name": fnName, "arguments": args})
			result := executeAITool(fnName, args, userPerms, audit)
			var resultJSON interface{}
			if json.Unmarshal([]byte(result), &resultJSON) != nil {
				resultJSON = result
			}
			buf.appendToolEvent(map[string]interface{}{"type": "tool_result", "name": fnName, "result": resultJSON}, runID)
			buf.publish(map[string]interface{}{"type": "tool_result", "name": fnName, "result": resultJSON})
			llmMessages = append(llmMessages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": getString(tc, "id"),
				"content":      result,
			})
		}
	}
	return "done", ""
}

// finishChatRun 写入终态：run 行 + assistant 消息（有内容才保留）+ 通知订阅者
func finishChatRun(run *model.SysChatRun, buf *RunBuffer, status, errMsg string) {
	buf.mu.Lock()
	buf.status = status
	buf.errMsg = errMsg
	answer := buf.answer
	toolEvents := append([]map[string]interface{}(nil), buf.toolEvents...)
	buf.mu.Unlock()

	now := time.Now()
	db := core.GetDB()
	db.Model(&model.SysChatRun{}).Where("id = ?", run.ID).Updates(map[string]interface{}{
		"status":      status,
		"answer":      answer,
		"tool_events": toolEventsJSON(toolEvents),
		"error":       errMsg,
		"finished_at": now,
	})

	// assistant 消息落库（停止且无内容的空回复不保存）
	if strings.TrimSpace(answer) != "" || len(toolEvents) > 0 {
		var session model.SysChatSession
		if err := db.Where("id = ? AND user_id = ?", run.SessionID, run.UserID).First(&session).Error; err == nil {
			db.Create(&model.SysChatMessage{
				SessionID:  session.ID,
				Role:       "assistant",
				Content:    answer,
				ToolEvents: toolEventsJSON(toolEvents),
			})
		}
	}

	buf.publish(map[string]interface{}{"type": "run_end", "status": status, "error": errMsg})
	// 保留缓冲供终态订阅者取快照，稍后回收
	time.AfterFunc(runBufferTTL, func() { removeRunBuffer(run.ID) })
}

// ────────────────────────── 对外操作 ──────────────────────────

// enqueueChatRun 创建 pending run + 持久化用户消息，立即返回
func enqueueChatRun(userID, sessionID, providerID uint, modelName, question string, enableTools bool) (*model.SysChatRun, string, error) {
	db := core.GetDB()
	var session model.SysChatSession
	if err := db.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		return nil, "", fmt.Errorf("会话不存在")
	}
	db.Model(&model.SysChatSession{}).Where("id = ?", session.ID).
		Updates(map[string]interface{}{"provider_id": providerID, "model": modelName})

	run := model.SysChatRun{
		SessionID:   sessionID,
		UserID:      userID,
		Status:      "pending",
		ProviderID:  providerID,
		Model:       modelName,
		EnableTools: enableTools,
		Question:    question,
	}
	if err := db.Create(&run).Error; err != nil {
		return nil, "", err
	}
	// 用户消息入会话（首条自动命名）
	db.Create(&model.SysChatMessage{SessionID: session.ID, Role: "user", Content: question})
	if session.Title == "" || session.Title == "新对话" {
		title := strings.TrimSpace(question)
		if runes := []rune(title); len(runes) > 50 {
			title = string(runes[:50])
		}
		if title != "" {
			db.Model(&model.SysChatSession{}).Where("id = ?", session.ID).Update("title", title)
			session.Title = title
		}
	}

	putRunBuffer(run.ID, newRunBuffer(run.ID, sessionID))
	startSessionWorker(sessionID)
	return &run, session.Title, nil
}

// stopChatRunRequest 停止 run：pending 撤回消息，running 置取消标记
func stopChatRunRequest(run *model.SysChatRun) *model.SysChatRun {
	if runTerminalStatuses[run.Status] {
		return run
	}
	buf := getRunBuffer(run.ID)
	db := core.GetDB()
	if run.Status == "pending" {
		// 撤回未作答的用户消息（最后一条同内容的 user 行）
		var orphan model.SysChatMessage
		if err := db.Where("session_id = ? AND role = ? AND content = ?", run.SessionID, "user", run.Question).
			Order("id DESC").First(&orphan).Error; err == nil {
			db.Delete(&orphan)
		}
		now := time.Now()
		db.Model(&model.SysChatRun{}).Where("id = ?", run.ID).
			Updates(map[string]interface{}{"status": "stopped", "error": "任务已取消", "finished_at": now})
		run.Status = "stopped"
		run.Error = "任务已取消"
		run.FinishedAt = &now
		if buf != nil {
			buf.mu.Lock()
			buf.status = "stopped"
			buf.errMsg = "任务已取消"
			buf.cancelRequested = true
			buf.mu.Unlock()
			buf.publish(map[string]interface{}{"type": "run_end", "status": "stopped", "error": "任务已取消"})
		}
		return run
	}
	if run.Status == "running" && buf != nil {
		buf.mu.Lock()
		buf.cancelRequested = true
		buf.mu.Unlock()
	}
	return run
}

// getActiveChatRunState 会话当前 pending/running run（优先实时缓冲状态）
func getActiveChatRunState(sessionID, userID uint) map[string]interface{} {
	db := core.GetDB()
	var runs []model.SysChatRun
	db.Where("session_id = ? AND user_id = ? AND status IN ?", sessionID, userID,
		[]string{"pending", "running"}).Order("id DESC").Find(&runs)
	if len(runs) == 0 {
		return nil
	}
	active := runs[0]
	queued := 0
	for _, r := range runs {
		if r.ID != active.ID && r.Status == "pending" {
			queued++
		}
	}
	state := runStateFromModel(&active)
	state["question"] = active.Question
	state["queued"] = queued
	return state
}

// runStateFromModel 序列化 run 状态（优先内存缓冲）
func runStateFromModel(run *model.SysChatRun) map[string]interface{} {
	status := run.Status
	answer := run.Answer
	errMsg := run.Error
	toolEvents := []interface{}{}
	if run.ToolEvents != "" {
		_ = json.Unmarshal([]byte(run.ToolEvents), &toolEvents)
	}
	if buf := getRunBuffer(run.ID); buf != nil {
		buf.mu.Lock()
		status = buf.status
		answer = buf.answer
		errMsg = buf.errMsg
		toolEvents = toIfaceSlice(buf.toolEvents)
		buf.mu.Unlock()
	}
	return map[string]interface{}{
		"run_id":      run.ID,
		"session_id":  run.SessionID,
		"status":      status,
		"answer":      answer,
		"tool_events": toolEvents,
		"error":       errMsg,
	}
}

// RecoverChatRuns 启动时回收上次进程遗留的 run（多实例安全）：
// 仅回收「无主」或「领取心跳超时」的 pending/running；
// 其他健康实例正在执行的 run（心跳新鲜）不会被误杀。
func RecoverChatRuns() int {
	db := core.GetDB()
	var runs []model.SysChatRun
	db.Where("status IN ?", []string{"pending", "running"}).
		Where("claimed_by = '' OR claimed_by IS NULL OR claimed_at IS NULL OR claimed_at < ?",
			runClaimStaleAt()).Find(&runs)
	n := 0
	for i := range runs {
		errMsg := "服务重启，请重新发送"
		if runs[i].Status == "running" {
			errMsg = "服务重启，本轮生成中断，请重新发送"
		}
		now := time.Now()
		db.Model(&model.SysChatRun{}).Where("id = ?", runs[i].ID).
			Updates(map[string]interface{}{"status": "failed", "error": errMsg, "finished_at": now})
		n++
	}
	return n
}

// ShutdownChatRuns 停机时取消所有 worker 并等待（限时）；
// pending run 释放领取标记，多实例部署下可被其他实例接管。
func ShutdownChatRuns(timeout time.Duration) {
	runWorkersMu.Lock()
	for _, cancel := range runWorkers {
		cancel()
	}
	runWorkersMu.Unlock()

	done := make(chan struct{})
	go func() {
		runWorkersWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}

	releaseOwnPendingClaims()

	runBuffersMu.Lock()
	runBuffers = map[uint]*RunBuffer{}
	runBuffersMu.Unlock()
}