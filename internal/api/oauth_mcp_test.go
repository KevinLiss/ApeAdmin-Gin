package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"apeadmin-gin/internal/config"
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/mcp"
	"apeadmin-gin/internal/middleware"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOAuthMcpTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.SysUser{}, &model.SysOAuthClient{}, &model.SysOAuthCode{}, &model.SysOAuthRefreshToken{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	core.SetDB(db)
	dal.Init()
	core.SetConfig(&config.Config{
		JWT: config.JWTConfig{
			Secret:            "unit-test-secret-key-32-bytes-ok!!",
			Algorithm:         "HS256",
			ExpireMinutes:     60,
			RefreshExpireDays: 7,
			BlacklistEnabled:  false,
		},
	})
	core.SetJWTConfig(&config.JWTConfig{
		Secret:            "unit-test-secret-key-32-bytes-ok!!",
		Algorithm:         "HS256",
		ExpireMinutes:     60,
		RefreshExpireDays: 7,
		BlacklistEnabled:  false,
	})
	t.Cleanup(func() { core.ResetTest() })

	// 种子超管 admin/admin123
	hash, _ := utils.HashPassword("admin123")
	admin := model.SysUser{Username: "admin", Password: hash, Status: 1, IsSuperAdmin: true, TokenVersion: 0}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}

	// MCP 管理器 + 无权限要求的内置工具
	mgr := mcp.NewManager(nil)
	mgr.SetConfig(core.GetConfig())
	mgr.RegisterTool(&mcp.ToolEntry{
		Name:        "mcp_test_echo",
		Description: "echo 工具（测试用）",
		InputSchema: map[string]interface{}{"type": "object"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{"echo": args["text"]}, nil
		},
	})
	core.SetMCPManager(mgr)

	r := gin.New()
	r.GET("/.well-known/oauth-protected-resource", OAuthWellKnownProtectedResource)
	r.GET("/.well-known/oauth-authorization-server", OAuthWellKnownAuthorizationServer)
	r.POST("/oauth/register", OAuthRegister)
	r.GET("/oauth/authorize", OAuthAuthorize)
	r.POST("/oauth/authorize/consent", OAuthAuthorizeConsent)
	r.POST("/oauth/token", OAuthToken)
	mcpH := &McpHandler{}
	r.POST("/mcp", middleware.MCPAuth(), mcpH.MCPGateway)
	return r
}

// pkcePair 生成 PKCE S256 校验对
func pkcePair() (verifier, challenge string) {
	verifier = base64.RawURLEncoding.EncodeToString(randomBytesForTest())
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge
}

func randomBytesForTest() []byte {
	b := make([]byte, 48)
	for i := range b {
		b[i] = byte('a' + i%26)
	}
	return b
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body interface{}, token string) (int, map[string]interface{}, *httptest.ResponseRecorder) {
	t.Helper()
	var req *http.Request
	if body != nil {
		data, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var m map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m, w
}

func doForm(t *testing.T, r *gin.Engine, path string, form url.Values) (int, string, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String(), w
}

func TestMCPGatewayFullOAuthFlow(t *testing.T) {
	r := setupOAuthMcpTest(t)

	// 1. 动态注册客户端
	code, resp, _ := doJSON(t, r, http.MethodPost, "/oauth/register", map[string]interface{}{
		"client_name":   "test-agent",
		"redirect_uris": []string{"http://localhost:9999/cb"},
	}, "")
	if code != http.StatusCreated {
		t.Fatalf("register: %d %v", code, resp)
	}
	clientID, _ := resp["client_id"].(string)
	clientSecret, _ := resp["client_secret"].(string)
	if clientID == "" || clientSecret == "" {
		t.Fatalf("register missing credentials: %v", resp)
	}

	// 2. PKCE
	verifier, challenge := pkcePair()
	redirect := "http://localhost:9999/cb"

	// 3. 授权端点（同意页）
	authorizeURL := fmt.Sprintf("/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=mcp&state=xyz123&code_challenge=%s&code_challenge_method=S256",
		url.QueryEscape(clientID), url.QueryEscape(redirect), url.QueryEscape(challenge))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, authorizeURL, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "授权") {
		t.Fatalf("authorize page: %d %s", w.Code, w.Body.String()[:min(200, len(w.Body.String()))])
	}

	// 4. 同意页提交（表单）→ 302 带 code
	form := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirect},
		"scope":                 {"mcp"},
		"state":                 {"xyz123"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"username":              {"admin"},
		"password":              {"admin123"},
	}
	code2, _, w2 := doForm(t, r, "/oauth/authorize/consent", form)
	if code2 != http.StatusFound {
		t.Fatalf("consent: %d %s", code2, w2.Body.String()[:min(300, len(w2.Body.String()))])
	}
	loc, _ := url.Parse(w2.Header().Get("Location"))
	authCode := loc.Query().Get("code")
	state := loc.Query().Get("state")
	if authCode == "" || state != "xyz123" {
		t.Fatalf("redirect missing code/state: %s", w2.Header().Get("Location"))
	}

	// 5. 换 token（authorization_code + PKCE verifier）
	tokForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode},
		"redirect_uri":  {redirect},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code_verifier": {verifier},
	}
	code3, body3, _ := doForm(t, r, "/oauth/token", tokForm)
	if code3 != http.StatusOK {
		t.Fatalf("token: %d %s", code3, body3)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		Refresh     string `json:"refresh_token"`
		TokenType   string `json:"token_type"`
	}
	_ = json.Unmarshal([]byte(body3), &tok)
	if tok.AccessToken == "" || tok.Refresh == "" || tok.TokenType != "Bearer" {
		t.Fatalf("token resp: %s", body3)
	}

	// 6. initialize
	code4, resp4, _ := doJSON(t, r, http.MethodPost, "/mcp", map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]interface{}{},
	}, tok.AccessToken)
	if code4 != http.StatusOK {
		t.Fatalf("initialize: %d %v", code4, resp4)
	}
	if result, _ := resp4["result"].(map[string]interface{}); result == nil || result["serverInfo"] == nil {
		t.Fatalf("initialize result: %v", resp4)
	}

	// 7. tools/list（超管可见全部工具）
	_, resp5, _ := doJSON(t, r, http.MethodPost, "/mcp", map[string]interface{}{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list",
	}, tok.AccessToken)
	result5, _ := resp5["result"].(map[string]interface{})
	tools, _ := result5["tools"].([]interface{})
	if len(tools) == 0 {
		t.Fatalf("tools/list empty: %v", resp5)
	}

	// 8. tools/call
	args, _ := json.Marshal(map[string]interface{}{
		"name":      "mcp_test_echo",
		"arguments": map[string]interface{}{"text": "你好"},
	})
	_, resp6, _ := doJSON(t, r, http.MethodPost, "/mcp", map[string]interface{}{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": json.RawMessage(args),
	}, tok.AccessToken)
	result6, _ := resp6["result"].(map[string]interface{})
	if result6 == nil || result6["isError"] == true {
		t.Fatalf("tools/call: %v", resp6)
	}
	content, _ := result6["content"].([]interface{})
	if len(content) == 0 {
		t.Fatalf("tools/call no content: %v", resp6)
	}

	// 9. refresh_token 轮换
	rf := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tok.Refresh},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	code7, body7, _ := doForm(t, r, "/oauth/token", rf)
	if code7 != http.StatusOK {
		t.Fatalf("refresh: %d %s", code7, body7)
	}
	var tok2 struct {
		AccessToken string `json:"access_token"`
		Refresh     string `json:"refresh_token"`
	}
	_ = json.Unmarshal([]byte(body7), &tok2)
	if tok2.AccessToken == "" || tok2.Refresh == "" || tok2.Refresh == tok.Refresh {
		t.Fatalf("refresh rotate failed")
	}
	// 旧 refresh 已作废
	code8, _, _ := doForm(t, r, "/oauth/token", rf)
	if code8 != http.StatusBadRequest {
		t.Fatalf("old refresh must be revoked: %d", code8)
	}
	// 新 access 可用
	code9, resp9, _ := doJSON(t, r, http.MethodPost, "/mcp", map[string]interface{}{
		"jsonrpc": "2.0", "id": 9, "method": "ping",
	}, tok2.AccessToken)
	if code9 != http.StatusOK || resp9["result"] == nil {
		t.Fatalf("new token ping: %d %v", code9, resp9)
	}
}

func TestMCPGatewayRejectsUnauthenticated(t *testing.T) {
	r := setupOAuthMcpTest(t)
	code, _, _ := doJSON(t, r, http.MethodPost, "/mcp", map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	}, "")
	if code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /mcp: %d, want 401", code)
	}
}

func TestMCPCallDeniedWithoutPermission(t *testing.T) {
	r := setupOAuthMcpTest(t)

	// 非超管用户（无任何权限）
	hash, _ := utils.HashPassword("pw123456")
	limitUser := model.SysUser{Username: "restricted", Password: hash, Status: 1, IsSuperAdmin: false}
	if err := core.GetDB().Create(&limitUser).Error; err != nil {
		t.Fatal(err)
	}
	// 给受限工具加权限要求
	mgr := core.GetMCPManager()
	mgr.UnregisterPluginTools("")
	mgr.RegisterTool(&mcp.ToolEntry{
		Name:                "admin_only_tool",
		Description:         "需要权限的工具",
		RequiredPermissions: []string{"system:setting:edit"},
		InputSchema:         map[string]interface{}{"type": "object"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			return "ok", nil
		},
	})

	access, err := core.GenerateAccessToken(&limitUser)
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]interface{}{})
	_, resp, _ := doJSON(t, r, http.MethodPost, "/mcp", map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": json.RawMessage(args),
	}, access)
	// 无权限工具不出现在 tools/list，tools/call 应拒绝
	errObj, _ := resp["error"].(map[string]interface{})
	if errObj == nil {
		t.Fatalf("unprivileged call must fail, got %v", resp)
	}
}

func TestWellKnownMetadata(t *testing.T) {
	r := setupOAuthMcpTest(t)
	code, resp, _ := doJSON(t, r, http.MethodGet, "/.well-known/oauth-protected-resource", nil, "")
	if code != http.StatusOK {
		t.Fatalf("protected-resource: %d", code)
	}
	if resp["authorization_servers"] == nil {
		t.Fatalf("metadata: %v", resp)
	}
	code, resp, _ = doJSON(t, r, http.MethodGet, "/.well-known/oauth-authorization-server", nil, "")
	if code != http.StatusOK || resp["token_endpoint"] == nil {
		t.Fatalf("auth-server metadata: %d %v", code, resp)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
