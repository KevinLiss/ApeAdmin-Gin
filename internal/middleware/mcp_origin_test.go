package middleware

import (
	"testing"

	"apeadmin-gin/internal/config"
	"apeadmin-gin/internal/core"
)

func TestMCPOriginGuardAllowed(t *testing.T) {
	core.SetConfig(&config.Config{CORS: config.CORSConfig{Origins: []string{"http://localhost:5173", "https://admin.example.com/"}}})
	t.Cleanup(func() { core.ResetTest() })

	cases := []struct {
		origin, host string
		want         bool
		desc         string
	}{
		{"http://127.0.0.1:8001", "127.0.0.1:8001", true, "同源"},
		{"http://localhost:5173", "127.0.0.1:8001", true, "开发代理白名单"},
		{"https://admin.example.com", "127.0.0.1:8001", true, "分域部署白名单（尾斜杠容错）"},
		{"https://evil.example.org", "127.0.0.1:8001", false, "未登记来源"},
		{"null", "127.0.0.1:8001", false, "非 URL Origin"},
	}
	for _, c := range cases {
		if got := MCPOriginAllowed(c.origin, c.host); got != c.want {
			t.Fatalf("%s: MCPOriginAllowed(%q,%q) = %v, want %v", c.desc, c.origin, c.host, got, c.want)
		}
	}
}

func TestMCPOriginGuardWithoutConfig(t *testing.T) {
	core.ResetTest()
	// 无配置时仅同源放行，任何外部 Origin 一律拒绝
	if !MCPOriginAllowed("http://127.0.0.1:8001", "127.0.0.1:8001") {
		t.Fatal("同源应放行")
	}
	if MCPOriginAllowed("https://evil.example.org", "127.0.0.1:8001") {
		t.Fatal("外部来源必须拒绝")
	}
}
