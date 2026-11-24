package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupRunnerTestDB 初始化内存 SQLite（chat_run / chat_session / chat_message）
func setupRunnerTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SysChatRun{}, &model.SysChatSession{}, &model.SysChatMessage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	core.SetDB(db)
	t.Cleanup(func() { core.ResetTest() })
}

// ─── RunBuffer ───

func TestToolEventsJSONEmpty(t *testing.T) {
	if got := toolEventsJSON(nil); got != "" {
		t.Fatalf("toolEventsJSON(nil) = %q, want empty", got)
	}
	if got := toolEventsJSON([]map[string]interface{}{}); got != "" {
		t.Fatalf("toolEventsJSON(empty) = %q, want empty", got)
	}
}

func TestToolEventsJSONRoundTrip(t *testing.T) {
	events := []map[string]interface{}{
		{"type": "tool_call", "name": "hello_get_time", "arguments": map[string]interface{}{}},
		{"type": "tool_result", "name": "hello_get_time", "result": "12:00"},
	}
	s := toolEventsJSON(events)
	if !strings.Contains(s, "tool_call") || !strings.Contains(s, "hello_get_time") {
		t.Fatalf("unexpected json: %s", s)
	}
}

func TestToIfaceSlice(t *testing.T) {
	in := []map[string]interface{}{{"a": 1}, {"b": 2}}
	out := toIfaceSlice(in)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	if m, ok := out[0].(map[string]interface{}); !ok || m["a"] != 1 {
		t.Fatalf("out[0] = %#v", out[0])
	}
}

func TestRunBufferPublishEventOrder(t *testing.T) {
	buf := newRunBuffer(1, 2)
	buf.publish(map[string]interface{}{"type": "run_start"})
	buf.publish(map[string]interface{}{"type": "content", "content": "a"})
	buf.publish(map[string]interface{}{"type": "content", "content": "b"})
	if len(buf.events) != 3 {
		t.Fatalf("events len = %d, want 3", len(buf.events))
	}
	if !strings.Contains(buf.events[0], "run_start") {
		t.Fatalf("events[0] = %s", buf.events[0])
	}
	if !strings.Contains(buf.events[1], `"a"`) {
		t.Fatalf("events[1] = %s", buf.events[1])
	}
}

func TestRunBufferPublishWakesSubscriber(t *testing.T) {
	buf := newRunBuffer(1, 2)
	buf.mu.Lock()
	notify := buf.notify
	buf.mu.Unlock()

	done := make(chan struct{})
	go func() {
		buf.publish(map[string]interface{}{"type": "content", "content": "x"})
		close(done)
	}()
	select {
	case <-notify:
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber was not woken by publish")
	}
	<-done
}

func TestRunBufferCancel(t *testing.T) {
	buf := newRunBuffer(1, 2)
	if buf.isCancelled() {
		t.Fatal("fresh buffer must not be cancelled")
	}
	buf.mu.Lock()
	buf.cancelRequested = true
	buf.mu.Unlock()
	if !buf.isCancelled() {
		t.Fatal("cancel flag not visible")
	}
}

// ─── appendAnswer（节流落库）───

func TestAppendAnswerFlushesToDB(t *testing.T) {
	setupRunnerTestDB(t)
	run := model.SysChatRun{SessionID: 1, UserID: 1, Status: "running", Question: "q"}
	core.GetDB().Create(&run)

	buf := newRunBuffer(run.ID, run.SessionID)
	buf.appendAnswer("你好", run.ID) // 首次必落库（lastFlush 为零值）

	var got model.SysChatRun
	if err := core.GetDB().First(&got, run.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Answer != "你好" {
		t.Fatalf("answer = %q, want 你好", got.Answer)
	}
}

func TestAppendAnswerThrottle(t *testing.T) {
	setupRunnerTestDB(t)
	run := model.SysChatRun{SessionID: 1, UserID: 1, Status: "running", Question: "q"}
	core.GetDB().Create(&run)

	buf := newRunBuffer(run.ID, run.SessionID)
	buf.appendAnswer("第一次", run.ID) // 落库
	// 强制刚落库过 → 第二次在节流窗口内不落库
	buf.mu.Lock()
	buf.lastFlush = time.Now()
	buf.mu.Unlock()
	buf.appendAnswer("第二次", run.ID)

	var got model.SysChatRun
	core.GetDB().First(&got, run.ID)
	if got.Answer != "第一次" {
		t.Fatalf("throttled write leaked: answer = %q", got.Answer)
	}
	// 内存 answer 仍是完整内容
	if buf.answer != "第一次第二次" {
		t.Fatalf("in-memory answer = %q", buf.answer)
	}
}

// ─── buildRunContext ───

func TestBuildRunContextTrimsTrailingAssistant(t *testing.T) {
	setupRunnerTestDB(t)
	core.GetDB().Create(&model.SysChatSession{UserID: 1})
	core.GetDB().Create(&model.SysChatMessage{SessionID: 1, Role: "user", Content: "q1"})
	core.GetDB().Create(&model.SysChatMessage{SessionID: 1, Role: "assistant", Content: "a1"})
	core.GetDB().Create(&model.SysChatMessage{SessionID: 1, Role: "assistant", Content: "a1 续"})

	msgs := buildRunContext(1, "q2")
	if len(msgs) != 2 {
		t.Fatalf("msgs = %d (%v), want 2", len(msgs), msgs)
	}
	if msgs[0]["role"] != "user" || msgs[0]["content"] != "q1" {
		t.Fatalf("msgs[0] = %v", msgs[0])
	}
	last := msgs[len(msgs)-1]
	if last["role"] != "user" || last["content"] != "q2" {
		t.Fatalf("last must be the new question, got %v", last)
	}
}

func TestBuildRunContextDedupsQuestion(t *testing.T) {
	setupRunnerTestDB(t)
	core.GetDB().Create(&model.SysChatSession{UserID: 1})
	core.GetDB().Create(&model.SysChatMessage{SessionID: 1, Role: "user", Content: "同一问题"})

	msgs := buildRunContext(1, "同一问题")
	if len(msgs) != 1 {
		t.Fatalf("question duplicated: %v", msgs)
	}
}

func TestBuildRunContextWindowLimit(t *testing.T) {
	setupRunnerTestDB(t)
	core.GetDB().Create(&model.SysChatSession{UserID: 1})
	for i := 0; i < 50; i++ {
		core.GetDB().Create(&model.SysChatMessage{SessionID: 1, Role: "user", Content: strings.Repeat("m", 1) + time.Duration(i).String()})
	}
	msgs := buildRunContext(1, "最新问题")
	// 窗口 40 条历史 + 1 条本轮问题；且不能以 assistant 结尾（此处全是 user）
	if len(msgs) > runContextWindow+1 {
		t.Fatalf("window exceeded: %d", len(msgs))
	}
	if msgs[len(msgs)-1]["content"] != "最新问题" {
		t.Fatalf("last message must be current question")
	}
}

// ─── 失效 running run 回收（H1）───

func TestRecycleStaleRunningRun(t *testing.T) {
	setupRunnerTestDB(t)
	db := core.GetDB()
	stale := time.Now().Add(-3 * time.Minute)
	run := model.SysChatRun{
		SessionID: 9, UserID: 1, Status: "running", Question: "q",
		Answer: "部分内容", ClaimedBy: "dead-instance", ClaimedAt: &stale,
	}
	db.Create(&run)

	n := recycleStaleRunningRuns(context.Background())
	if n != 1 {
		t.Fatalf("recycled = %d, want 1", n)
	}

	var got model.SysChatRun
	db.First(&got, run.ID)
	if got.Status != "failed" {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if got.Error != runStaleErr {
		t.Fatalf("error = %q", got.Error)
	}
	if got.FinishedAt == nil {
		t.Fatal("finished_at 未写入")
	}
	if got.ClaimedBy != "" {
		t.Fatalf("claimed_by = %q, want cleared", got.ClaimedBy)
	}

	// 已生成的部分内容必须落为 assistant 消息，刷新会话不丢内容
	var msgs []model.SysChatMessage
	db.Where("session_id = ? AND role = ?", 9, "assistant").Find(&msgs)
	if len(msgs) != 1 || msgs[0].Content != "部分内容" {
		t.Fatalf("assistant messages = %#v", msgs)
	}
}

func TestRecycleSkipsFreshRunningRun(t *testing.T) {
	setupRunnerTestDB(t)
	db := core.GetDB()
	fresh := time.Now()
	run := model.SysChatRun{
		SessionID: 1, UserID: 1, Status: "running", Question: "q",
		ClaimedBy: "alive-instance", ClaimedAt: &fresh,
	}
	db.Create(&run)

	if n := recycleStaleRunningRuns(context.Background()); n != 0 {
		t.Fatalf("recycled = %d, want 0（心跳新鲜的 run 不得误杀）", n)
	}
	var got model.SysChatRun
	db.First(&got, run.ID)
	if got.Status != "running" {
		t.Fatalf("status = %s, want running", got.Status)
	}
}
