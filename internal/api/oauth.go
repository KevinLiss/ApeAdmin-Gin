package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ────────────────────────────────────────────────────────────────────────────
// OAuth 2.0 授权服务器 + MCP 资源服务器发现（RFC 8414 / RFC 9728 / RFC 7591）
//
// 用途：外部 MCP 客户端（Claude/Cursor/自研 Agent）以业务账号授权接入本系统的
// MCP 工具。本系统同时承担授权服务器（AS）与资源服务器（RS）角色：
//   1. 客户端经 /.well-known/oauth-protected-resource 发现授权端点
//   2. 动态注册（POST /oauth/register）取得 client_id/secret
//   3. 用户在同意页登录授权（authorization code + PKCE S256）
//   4. 客户端换得 access token（即本系统 JWT，绑定用户与权限）
//   5. Bearer 调用 POST /mcp（Streamable HTTP，见 mcp_gateway.go）
// ────────────────────────────────────────────────────────────────────────────

const (
	oauthCodeTTL       = 5 * time.Minute
	oauthAccessTokenTTL = 60 * time.Minute
	oauthRefreshTTL     = 30 * 24 * time.Hour
)

func oauthBaseURL(c *gin.Context) string {
	scheme := "http"
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

// OAuthWellKnownProtectedResource RFC 9728 保护资源元数据（MCP 客户端发现入口）
func OAuthWellKnownProtectedResource(c *gin.Context) {
	base := oauthBaseURL(c)
	c.JSON(http.StatusOK, gin.H{
		"resource":               base + "/mcp",
		"authorization_servers":  []string{base + "/"},
		"scopes_supported":       []string{"mcp"},
		"bearer_methods_supported": []string{"header"},
	})
}

// OAuthWellKnownAuthorizationServer RFC 8414 授权服务器元数据
func OAuthWellKnownAuthorizationServer(c *gin.Context) {
	base := oauthBaseURL(c)
	c.JSON(http.StatusOK, gin.H{
		"issuer":                                base + "/",
		"authorization_endpoint":                base + "/api/v1/oauth/authorize",
		"token_endpoint":                        base + "/api/v1/oauth/token",
		"registration_endpoint":                 base + "/api/v1/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
	})
}

// OAuthRegister 动态客户端注册（RFC 7591 简化版；公开端点 + 限流）
func OAuthRegister(c *gin.Context) {
	var req struct {
		ClientName  string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris" binding:"required"`
		Remark      string   `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.RedirectURIs) == 0 {
		c.JSON(http.StatusBadRequest, responseBad("redirect_uris 不能为空"))
		return
	}
	for _, u := range req.RedirectURIs {
		if _, err := url.ParseRequestURI(u); err != nil || !strings.HasPrefix(u, "http") {
			c.JSON(http.StatusBadRequest, responseBad("redirect_uri 非法: "+u))
			return
		}
	}
	// 上限保护
	if len(req.RedirectURIs) > 10 {
		c.JSON(http.StatusBadRequest, responseBad("redirect_uris 最多 10 个"))
		return
	}

	clientID, err := randomToken(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responseBad("生成 client_id 失败"))
		return
	}
	clientSecret, err := randomToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responseBad("生成 client_secret 失败"))
		return
	}
	hash, err := utils.HashPassword(clientSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responseBad("hash 失败"))
		return
	}
	uris, _ := json.Marshal(req.RedirectURIs)
	client := model.SysOAuthClient{
		Name:             req.ClientName,
		ClientID:         clientID,
		ClientSecretHash: hash,
		RedirectURIs:     string(uris),
		Enabled:          true,
		Remark:           req.Remark,
	}
	if err := core.GetDB().Create(&client).Error; err != nil {
		c.JSON(http.StatusInternalServerError, responseBad("保存失败"))
		return
	}
	// client_secret 明文仅此一次返回（RFC 7591 语义）
	c.JSON(http.StatusCreated, gin.H{
		"client_id":                  clientID,
		"client_secret":              clientSecret,
		"client_name":                client.Name,
		"redirect_uris":              req.RedirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "client_secret_post",
	})
}

// OAuthAuthorize 授权端点：返回极简同意页（未带登录态；表单提交到 consent）
func OAuthAuthorize(c *gin.Context) {
	q := c.Request.URL.Query()
	if err := validateAuthorizeParams(q); err != nil {
		c.String(http.StatusBadRequest, oauthHTML("参数错误", err.Error()))
		return
	}
	clientID := q.Get("client_id")
	var client model.SysOAuthClient
	if err := core.GetDB().Where("client_id = ? AND enabled = ?", clientID, true).First(&client).Error; err != nil {
		c.String(http.StatusBadRequest, oauthHTML("客户端无效", "client_id 不存在或已停用"))
		return
	}
	// redirect_uri 必须与注册完全一致
	redirect := q.Get("redirect_uri")
	var uris []string
	_ = json.Unmarshal([]byte(client.RedirectURIs), &uris)
	if !containsString(uris, redirect) {
		c.String(http.StatusBadRequest, oauthHTML("redirect_uri 未注册", "回调地址与注册不一致"))
		return
	}

	hidden := map[string]string{
		"response_type":         "code",
		"client_id":             clientID,
		"redirect_uri":          redirect,
		"scope":                 q.Get("scope"),
		"state":                 q.Get("state"),
		"code_challenge":        q.Get("code_challenge"),
		"code_challenge_method": q.Get("code_challenge_method"),
	}
	var sb strings.Builder
	for k, v := range hidden {
		sb.WriteString(fmt.Sprintf(`<input type="hidden" name="%s" value="%s"/>`, k, htmlEscape(v)))
	}
	body := fmt.Sprintf(`
		<p>应用 <b>%s</b> 请求访问你的 ApeAdmin MCP 工具。</p>
		<form method="POST" action="/api/v1/oauth/authorize/consent">
			%s
			<p><input name="username" placeholder="账号" required style="width:100%%;padding:8px"/></p>
			<p><input name="password" type="password" placeholder="密码" required style="width:100%%;padding:8px"/></p>
			<button type="submit" style="width:100%%;padding:10px;background:#5A67F5;color:#fff;border:none;border-radius:8px">授权</button>
		</form>`, htmlEscape(client.Name), sb.String())
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(oauthHTML("授权登录", body)))
}

// OAuthAuthorizeConsent 处理同意页提交：校验账号密码 → 签发授权码 → 302 回调
func OAuthAuthorizeConsent(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	clientID := c.PostForm("client_id")
	redirect := c.PostForm("redirect_uri")
	state := c.PostForm("state")

	q := url.Values{
		"response_type":         {c.PostForm("response_type")},
		"client_id":             {clientID},
		"redirect_uri":          {redirect},
		"scope":                 {c.PostForm("scope")},
		"state":                 {state},
		"code_challenge":        {c.PostForm("code_challenge")},
		"code_challenge_method": {c.PostForm("code_challenge_method")},
	}
	// 参数复验（防绕过 consent 表单）
	if err := validateAuthorizeParams(q); err != nil {
		c.String(http.StatusBadRequest, oauthHTML("参数错误", err.Error()))
		return
	}
	var client model.SysOAuthClient
	if err := core.GetDB().Where("client_id = ? AND enabled = ?", clientID, true).First(&client).Error; err != nil {
		c.String(http.StatusBadRequest, oauthHTML("客户端无效", "client_id 不存在或已停用"))
		return
	}
	var uris []string
	_ = json.Unmarshal([]byte(client.RedirectURIs), &uris)
	if !containsString(uris, redirect) {
		c.String(http.StatusBadRequest, oauthHTML("redirect_uri 未注册", "回调地址与注册不一致"))
		return
	}
	if q.Get("response_type") != "code" || q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		c.String(http.StatusBadRequest, oauthHTML("参数错误", "仅支持 response_type=code 与 PKCE S256"))
		return
	}

	// 校验业务账号密码
	user, err := verifyUserCredentials(username, password)
	if err != nil {
		c.String(http.StatusUnauthorized, oauthHTML("登录失败", "账号或密码错误"))
		return
	}

	// 签发一次性授权码
	code, _ := randomToken(32)
	authCode := model.SysOAuthCode{
		Code:            code,
		ClientID:        clientID,
		UserID:          user.ID,
		RedirectURI:     redirect,
		Scope:           q.Get("scope"),
		CodeChallenge:   q.Get("code_challenge"),
		ChallengeMethod: "S256",
		ExpiresAt:       time.Now().Add(oauthCodeTTL),
	}
	if err := core.GetDB().Create(&authCode).Error; err != nil {
		c.String(http.StatusInternalServerError, oauthHTML("错误", "签发授权码失败"))
		return
	}

	target, _ := url.Parse(redirect)
	params := target.Query()
	params.Set("code", code)
	if state != "" {
		params.Set("state", state)
	}
	target.RawQuery = params.Encode()
	c.Redirect(http.StatusFound, target.String())
}

// OAuthToken 令牌端点：authorization_code / refresh_token 两种 grant
func OAuthToken(c *gin.Context) {
	var form struct {
		GrantType    string `form:"grant_type"`
		Code         string `form:"code"`
		RedirectURI  string `form:"redirect_uri"`
		ClientID     string `form:"client_id"`
		ClientSecret string `form:"client_secret"`
		CodeVerifier string `form:"code_verifier"`
		RefreshToken string `form:"refresh_token"`
	}
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, oauthTokenErr("invalid_request", "参数缺失"))
		return
	}

	switch form.GrantType {
	case "authorization_code":
		if form.Code == "" || form.ClientID == "" || form.ClientSecret == "" || form.RedirectURI == "" || form.CodeVerifier == "" {
			c.JSON(http.StatusBadRequest, oauthTokenErr("invalid_request", "code/client_id/client_secret/redirect_uri/code_verifier 必填"))
			return
		}
		var code model.SysOAuthCode
		if err := core.GetDB().Where("code = ?", form.Code).First(&code).Error; err != nil {
			c.JSON(http.StatusBadRequest, oauthTokenErr("invalid_grant", "授权码无效"))
			return
		}
		if code.Used || time.Now().After(code.ExpiresAt) {
			c.JSON(http.StatusBadRequest, oauthTokenErr("invalid_grant", "授权码已使用或过期"))
			return
		}
		if code.ClientID != form.ClientID || code.RedirectURI != form.RedirectURI {
			c.JSON(http.StatusBadRequest, oauthTokenErr("invalid_grant", "client/redirect 与授权时不一致"))
			return
		}
		// PKCE S256 校验
		sum := sha256.Sum256([]byte(form.CodeVerifier))
		if subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(code.CodeChallenge)) != 1 {
			c.JSON(http.StatusBadRequest, oauthTokenErr("invalid_grant", "PKCE 校验失败"))
			return
		}
		// client_secret 校验（bcrypt）
		var client model.SysOAuthClient
		if err := core.GetDB().Where("client_id = ? AND enabled = ?", form.ClientID, true).First(&client).Error; err != nil {
			c.JSON(http.StatusBadRequest, oauthTokenErr("invalid_client", "客户端无效"))
			return
		}
		if !utils.CheckPassword(form.ClientSecret, client.ClientSecretHash) {
			c.JSON(http.StatusUnauthorized, oauthTokenErr("invalid_client", "client_secret 错误"))
			return
		}

		tokenResp, err := issueMcpTokenPair(c, form.ClientID, code.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, oauthTokenErr("server_error", "签发失败"))
			return
		}
		// 标记授权码已使用（防重放）
		core.GetDB().Model(&model.SysOAuthCode{}).Where("id = ?", code.ID).Update("used", true)
		c.JSON(http.StatusOK, tokenResp)

	case "refresh_token":
		if form.RefreshToken == "" || form.ClientID == "" || form.ClientSecret == "" {
			c.JSON(http.StatusBadRequest, oauthTokenErr("invalid_request", "refresh_token/client_id/client_secret 必填"))
			return
		}
		var client model.SysOAuthClient
		if err := core.GetDB().Where("client_id = ? AND enabled = ?", form.ClientID, true).First(&client).Error; err != nil {
			c.JSON(http.StatusBadRequest, oauthTokenErr("invalid_client", "客户端无效"))
			return
		}
		if !utils.CheckPassword(form.ClientSecret, client.ClientSecretHash) {
			c.JSON(http.StatusUnauthorized, oauthTokenErr("invalid_client", "client_secret 错误"))
			return
		}
		sum := sha256.Sum256([]byte(form.RefreshToken))
		hash := hex.EncodeToString(sum[:])
		var stored model.SysOAuthRefreshToken
		if err := core.GetDB().Where("token_hash = ? AND client_id = ?", hash, form.ClientID).First(&stored).Error; err != nil ||
			stored.Revoked || time.Now().After(stored.ExpiresAt) {
			c.JSON(http.StatusBadRequest, oauthTokenErr("invalid_grant", "refresh_token 无效或已过期"))
			return
		}
		// 轮换：旧 token 作废
		core.GetDB().Model(&model.SysOAuthRefreshToken{}).Where("id = ?", stored.ID).Update("revoked", true)

		tokenResp, err := issueMcpTokenPair(c, form.ClientID, stored.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, oauthTokenErr("server_error", "签发失败"))
			return
		}
		c.JSON(http.StatusOK, tokenResp)

	default:
		c.JSON(http.StatusBadRequest, oauthTokenErr("unsupported_grant_type", "仅支持 authorization_code / refresh_token"))
	}
}

// issueMcpTokenPair 为用户签发 access（系统 JWT，绑定 token_version）+ refresh 对
func issueMcpTokenPair(c *gin.Context, clientID string, userID uint) (gin.H, error) {
	var user model.SysUser
	if err := core.GetDB().First(&user, userID).Error; err != nil || user.Status != 1 {
		return nil, fmt.Errorf("user unavailable")
	}
	access, err := core.GenerateAccessToken(&user)
	if err != nil {
		return nil, err
	}
	refresh, err := randomToken(48)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(refresh))
	rt := model.SysOAuthRefreshToken{
		TokenHash: hex.EncodeToString(sum[:]),
		ClientID:  clientID,
		UserID:    userID,
		ExpiresAt: time.Now().Add(oauthRefreshTTL),
	}
	if err := core.GetDB().Create(&rt).Error; err != nil {
		return nil, err
	}
	return gin.H{
		"access_token":  access,
		"refresh_token": refresh,
		"token_type":    "Bearer",
		"expires_in":    int(oauthAccessTokenTTL.Seconds()),
		"scope":         "mcp",
	}, nil
}

// verifyUserCredentials 业务账号密码校验（复用登录的 bcrypt 校验）
func verifyUserCredentials(username, password string) (*model.SysUser, error) {
	var user model.SysUser
	if err := core.GetDB().Where("username = ? AND status = 1", username).First(&user).Error; err != nil {
		return nil, err
	}
	if !utils.CheckPassword(password, user.Password) {
		return nil, fmt.Errorf("bad credentials")
	}
	return &user, nil
}

// validateAuthorizeParams 校验授权端点公共参数
func validateAuthorizeParams(q url.Values) error {
	if q.Get("response_type") != "code" {
		return fmt.Errorf("response_type 必须为 code")
	}
	if q.Get("client_id") == "" {
		return fmt.Errorf("client_id 必填")
	}
	if q.Get("redirect_uri") == "" {
		return fmt.Errorf("redirect_uri 必填")
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		return fmt.Errorf("必须提供 PKCE（code_challenge_method=S256）")
	}
	return nil
}

func containsString(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func oauthTokenErr(code, msg string) gin.H {
	return gin.H{"error": code, "error_description": msg}
}

func responseBad(msg string) gin.H {
	return gin.H{"code": 400, "msg": msg}
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func oauthHTML(title, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>%s - ApeAdmin</title><style>body{font-family:-apple-system,system-ui,sans-serif;max-width:420px;margin:60px auto;padding:0 16px;color:#303133}
h2{text-align:center}button{cursor:pointer}input{border:1px solid #dcdfe6;border-radius:8px}</style></head>
<body><h2>%s</h2><div class="card" style="background:#f8fafc;border-radius:12px;padding:20px">%s</div></body></html>`,
		htmlEscape(title), htmlEscape(title), body)
}