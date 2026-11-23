package mcp

import (
	"strings"
	"testing"

	"apeadmin-gin/internal/config"
)

func newTestManager() *Manager {
	m := NewManager(nil)
	m.SetConfig(&config.Config{})
	return m
}

func TestRegisterAndListTool(t *testing.T) {
	m := newTestManager()
	m.RegisterTool(&ToolEntry{
		Name:        "t_hello",
		Description: "say hello",
		InputSchema: map[string]interface{}{"type": "object"},
		Handler:     func(args map[string]interface{}) (interface{}, error) { return "hi", nil },
	})
	tools := m.ListTools()
	if len(tools) != 1 || tools[0].Name != "t_hello" {
		t.Fatalf("tools = %d", len(tools))
	}
	if _, ok := m.GetTool("t_hello"); !ok {
		t.Fatal("GetTool(t_hello) not found")
	}
	if _, ok := m.GetTool("nope"); ok {
		t.Fatal("GetTool(nope) should miss")
	}
}

func TestUnregisterPluginTools(t *testing.T) {
	m := newTestManager()
	m.RegisterTool(&ToolEntry{Name: "p1_a", PluginName: "alpha", Handler: func(map[string]interface{}) (interface{}, error) { return nil, nil }})
	m.RegisterTool(&ToolEntry{Name: "p1_b", PluginName: "alpha", Handler: func(map[string]interface{}) (interface{}, error) { return nil, nil }})
	m.RegisterTool(&ToolEntry{Name: "keep", PluginName: "beta", Handler: func(map[string]interface{}) (interface{}, error) { return nil, nil }})

	m.UnregisterPluginTools("alpha")
	if _, ok := m.GetTool("p1_a"); ok {
		t.Fatal("alpha tool should be removed")
	}
	if _, ok := m.GetTool("keep"); !ok {
		t.Fatal("beta tool must survive")
	}
}

func TestListCategories(t *testing.T) {
	m := newTestManager()
	m.RegisterTool(&ToolEntry{Name: "a1", Category: "系统", Handler: func(map[string]interface{}) (interface{}, error) { return nil, nil }})
	m.RegisterTool(&ToolEntry{Name: "a2", Category: "系统", Handler: func(map[string]interface{}) (interface{}, error) { return nil, nil }})
	m.RegisterTool(&ToolEntry{Name: "b1", Category: "用户", Handler: func(map[string]interface{}) (interface{}, error) { return nil, nil }})
	cats := m.ListCategories()
	if len(cats) != 2 {
		t.Fatalf("categories = %d, want 2", len(cats))
	}
}

func TestRegisterReadResource(t *testing.T) {
	m := newTestManager()
	m.RegisterResource(&ResourceEntry{
		URI:         "system://info",
		Name:        "系统信息",
		Description: "desc",
		MimeType:    "text/plain",
		ReadHandler: func() (string, error) { return "payload", nil },
	})
	if len(m.ListResources()) != 1 {
		t.Fatal("resource not listed")
	}
	s, ok := m.ReadResource("system://info")
	if !ok || s != "payload" {
		t.Fatalf("read = %q ok=%v", s, ok)
	}
	if _, ok := m.ReadResource("system://none"); ok {
		t.Fatal("unknown uri should miss")
	}
}

func TestRenderPrompt(t *testing.T) {
	m := newTestManager()
	m.RegisterPrompt(&PromptEntry{
		Name:        "greet",
		Description: "greeting prompt",
		Arguments:   []string{"name"},
		Template:    "请向 {name} 问好",
	})
	s, ok := m.RenderPrompt("greet", map[string]string{"name": "管理员"})
	if !ok || !strings.Contains(s, "管理员") {
		t.Fatalf("render = %q ok=%v", s, ok)
	}
	if _, ok := m.RenderPrompt("none", nil); ok {
		t.Fatal("unknown prompt should miss")
	}
}

func TestMarshalSchema(t *testing.T) {
	s := MarshalSchema(map[string]interface{}{"type": "object"})
	if !strings.Contains(s, `"type":"object"`) && !strings.Contains(s, `"type": "object"`) {
		t.Fatalf("schema json = %s", s)
	}
}
