package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"apeadmin-gin/internal/config"
	"apeadmin-gin/internal/core"

	"github.com/gin-gonic/gin"
)

func resetLimitBuckets() {
	tbMu.Lock()
	tbBuckets = make(map[string]*tokenBucket)
	tbMu.Unlock()
}

func resetGuardRecords() {
	lgMu.Lock()
	lgRecords = make(map[string]*loginGuardEntry)
	lgMu.Unlock()
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// ─── RateLimit ───

func TestRateLimitAllowsBurst(t *testing.T) {
	resetLimitBuckets()
	r := newTestRouter()
	r.GET("/x", RateLimit("t1", 5, time.Minute), func(c *gin.Context) { c.String(200, "ok") })

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: code = %d, want 200", i+1, w.Code)
		}
	}
}

func TestRateLimitBlocksOverBurst(t *testing.T) {
	resetLimitBuckets()
	r := newTestRouter()
	r.GET("/x", RateLimit("t2", 3, time.Minute), func(c *gin.Context) { c.String(200, "ok") })

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "5.6.7.8:1"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d within burst: code = %d", i+1, w.Code)
		}
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "5.6.7.8:1"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over burst: code = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 must carry Retry-After header")
	}
}

func TestRateLimitIsolatesClients(t *testing.T) {
	resetLimitBuckets()
	r := newTestRouter()
	r.GET("/x", RateLimit("t3", 1, time.Minute), func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "9.9.9.1:1"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first client: %d", w.Code)
	}
	// 另一个 IP 不受影响
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "9.9.9.2:1"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("second client blocked by first: %d", w.Code)
	}
}

// ─── LoginGuard ───

func guardTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			LoginGuard: config.LoginGuardConfig{
				MaxAttempts:   3,
				WindowMinutes: 15,
				LockMinutes:   30,
			},
		},
	}
}

// postLogin 模拟一次登录请求（body 为 JSON），返回响应记录器
func postLogin(r *gin.Engine, ip, username, password string, status int) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	body := `{"username":"` + username + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":1234"
	r.ServeHTTP(w, req)
	if w.Code != status {
		// 由调用方断言前的宽松校验；这里不 fail，让调用方自行断言
		_ = w
	}
	return w
}

func TestLoginGuardLocksAfterMaxAttempts(t *testing.T) {
	resetGuardRecords()
	core.SetConfig(guardTestConfig())
	t.Cleanup(func() { core.ResetTest() })

	r := newTestRouter()
	r.POST("/login", LoginGuard(), func(c *gin.Context) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401})
	})

	ip := "10.0.0.1"
	for i := 0; i < 3; i++ {
		w := postLogin(r, ip, "alice", "wrong", http.StatusUnauthorized)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: code = %d, want 401", i+1, w.Code)
		}
	}
	// 达到 MaxAttempts 后：未真正校验密码前就被锁定为 429
	w := postLogin(r, ip, "alice", "wrong", http.StatusTooManyRequests)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("locked request: code = %d, want 429", w.Code)
	}
}

func TestLoginGuardIsolatesUsers(t *testing.T) {
	resetGuardRecords()
	core.SetConfig(guardTestConfig())
	t.Cleanup(func() { core.ResetTest() })

	r := newTestRouter()
	r.POST("/login", LoginGuard(), func(c *gin.Context) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401})
	})

	// alice 连续失败但未达阈值，bob 不受影响
	for i := 0; i < 2; i++ {
		postLogin(r, "10.0.0.2", "alice", "wrong", http.StatusUnauthorized)
	}
	w := postLogin(r, "10.0.0.2", "bob", "wrong", http.StatusUnauthorized)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bob should not be locked: %d", w.Code)
	}
}

// OAuth 同意页成功时返回 302 而非 200，计数同样必须被清零
func TestLoginGuardClearsOnRedirect(t *testing.T) {
	resetGuardRecords()
	core.SetConfig(guardTestConfig())
	t.Cleanup(func() { core.ResetTest() })

	r := newTestRouter()
	r.POST("/consent", LoginGuard(), func(c *gin.Context) {
		if c.PostForm("password") == "right" {
			c.Redirect(http.StatusFound, "https://example.com/cb")
			return
		}
		c.String(http.StatusUnauthorized, "bad creds")
	})

	ip := "10.0.0.4"
	// 先制造 2 次失败（未达阈值 3）
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/consent", strings.NewReader("username=dave&password=wrong"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = ip + ":1"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i+1, w.Code)
		}
	}
	// 成功（302）后记录应被清除
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/consent", strings.NewReader("username=dave&password=right"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = ip + ":1"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("success code = %d, want 302", w.Code)
	}
	lgMu.Lock()
	_, exists := lgRecords[ip+"|dave"]
	lgMu.Unlock()
	if exists {
		t.Fatal("302 成功响应必须清除登录防护记录")
	}
}

func TestLoginGuardLocksConsentEntry(t *testing.T) {
	resetGuardRecords()
	core.SetConfig(guardTestConfig())
	t.Cleanup(func() { core.ResetTest() })

	r := newTestRouter()
	r.POST("/consent", LoginGuard(), func(c *gin.Context) {
		c.String(http.StatusUnauthorized, "bad creds")
	})

	ip := "10.0.0.5"
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/consent", strings.NewReader("username=erin&password=x"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = ip + ":1"
		r.ServeHTTP(w, req)
	}
	// 第 4 次在进入 handler 前即被 429 拦截（同意页不能再作为爆破旁路）
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/consent", strings.NewReader("username=erin&password=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = ip + ":1"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("locked consent: code = %d, want 429", w.Code)
	}
}

func TestLoginGuardResetsOnSuccess(t *testing.T) {
	resetGuardRecords()
	core.SetConfig(guardTestConfig())
	t.Cleanup(func() { core.ResetTest() })

	r := newTestRouter()
	r.POST("/login", LoginGuard(), func(c *gin.Context) {
		if c.PostForm("ok") == "1" {
			c.JSON(http.StatusOK, gin.H{"code": 200})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401})
	})

	ip := "10.0.0.3"
	// 用表单请求带 ok=1 模拟成功
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=carol&password=x&ok=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = ip + ":1"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("success login: %d", w.Code)
	}
	// 成功后无失败记录（key 为 ip|carol）
	lgMu.Lock()
	_, exists := lgRecords[ip+"|carol"]
	lgMu.Unlock()
	if exists {
		t.Fatal("successful login must clear guard record")
	}
}
