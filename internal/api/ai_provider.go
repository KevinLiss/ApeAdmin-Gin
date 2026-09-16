package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/response"
	"apeadmin-gin/internal/pkg/utils"
)

// AiHandler AI 供应商 + 会话 + 对话 Handler
type AiHandler struct{}

// getSecret 获取加密密钥（JWT Secret 派生）
func getSecret() string {
	cfg := core.GetConfig()
	if cfg == nil || cfg.JWT.Secret == "" {
		return "apeadmin-gin-default-secret"
	}
	return cfg.JWT.Secret
}

// providerToDict 转前端结构（API Key 脱敏）
func providerToDict(p *model.SysAiProvider) map[string]interface{} {
	models := []string{}
	if p.Models != nil && *p.Models != "" {
		_ = jsonUnmarshal(*p.Models, &models)
	}
	masked := "****"
	if p.ApiKeyEnc != "" {
		if plain, err := utils.DecryptSecret(getSecret(), p.ApiKeyEnc); err == nil {
			masked = utils.MaskAPIKey(plain)
		}
	}
	enabled := 0
	if p.Enabled {
		enabled = 1
	}
	return map[string]interface{}{
		"id":             p.ID,
		"name":           p.Name,
		"provider_type":  p.ProviderType,
		"base_url":       p.BaseURL,
		"models":         models,
		"enabled":        enabled,
		"api_key_masked": masked,
		"sort":           p.Sort,
		"remark":         p.Remark,
		"created_at":     p.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// ListProviders 分页列表（管理页）
func (h *AiHandler) ListProviders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	db := core.GetDB()
	var total int64
	db.Model(&model.SysAiProvider{}).Count(&total)
	var items []model.SysAiProvider
	db.Order("sort ASC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items)
	rows := make([]map[string]interface{}, 0, len(items))
	for i := range items {
		rows = append(rows, providerToDict(&items[i]))
	}
	c.JSON(http.StatusOK, response.SuccessPage(rows, total, page, pageSize))
}

// ListAllProviders 列出全部启用的供应商（对话页下拉用）
func (h *AiHandler) ListAllProviders(c *gin.Context) {
	db := core.GetDB()
	var items []model.SysAiProvider
	db.Where("enabled = ?", true).Order("sort ASC, id ASC").Find(&items)
	rows := make([]map[string]interface{}, 0, len(items))
	for i := range items {
		rows = append(rows, providerToDict(&items[i]))
	}
	c.JSON(http.StatusOK, response.Success(rows))
}

// CreateProvider 新增供应商
func (h *AiHandler) CreateProvider(c *gin.Context) {
	var req struct {
		Name         string   `json:"name" binding:"required"`
		ProviderType string   `json:"provider_type" binding:"required"`
		APIKey       string   `json:"api_key" binding:"required"`
		BaseURL      string   `json:"base_url"`
		Models       []string `json:"models"`
		Enabled      int      `json:"enabled"`
		Sort         int      `json:"sort"`
		Remark       string   `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	db := core.GetDB()
	var count int64
	db.Model(&model.SysAiProvider{}).Where("name = ?", req.Name).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, response.Error(409, "供应商已存在"))
		return
	}
	enc, err := utils.EncryptSecret(getSecret(), req.APIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "密钥加密失败"))
		return
	}
	modelsJSON, _ := marshalJSON(req.Models)
	p := model.SysAiProvider{
		Name:         req.Name,
		ProviderType: req.ProviderType,
		BaseURL:      req.BaseURL,
		Models:       &modelsJSON,
		ApiKeyEnc:    enc,
		Sort:         req.Sort,
		Remark:       req.Remark,
		Enabled:      req.Enabled == 1,
	}
	db.Create(&p)
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{"id": p.ID}))
}

// UpdateProvider 更新供应商
func (h *AiHandler) UpdateProvider(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
var req struct {
		Name         string   `json:"name"`
		ProviderType string   `json:"provider_type"`
		APIKey       string   `json:"api_key"`
		BaseURL      string   `json:"base_url"`
		Models       []string `json:"models"`
		Enabled      *int     `json:"enabled"`
		Sort         *int     `json:"sort"`
		Remark       string   `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	db := core.GetDB()
	var p model.SysAiProvider
	if err := db.First(&p, id).Error; err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "供应商不存在"))
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.ProviderType != "" {
		updates["provider_type"] = req.ProviderType
	}
	if req.BaseURL != "" {
		updates["base_url"] = req.BaseURL
	}
	if req.Models != nil {
		modelsJSON, _ := marshalJSON(req.Models)
		updates["models"] = modelsJSON
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}
	if req.Remark != "" {
		updates["remark"] = req.Remark
	}
	if req.APIKey != "" {
		enc, err := utils.EncryptSecret(getSecret(), req.APIKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.Error(500, "密钥加密失败"))
			return
		}
		updates["api_key_enc"] = enc
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled == 1
	}
	db.Model(&p).Updates(updates)
	c.JSON(http.StatusOK, response.SuccessMsg("更新成功"))
}

// DeleteProvider 删除供应商（硬删除）
func (h *AiHandler) DeleteProvider(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	db := core.GetDB()
	if err := db.Unscoped().Delete(&model.SysAiProvider{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "删除失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("删除成功"))
}

// TestProvider 测试连通性（调用 /models 端点）
func (h *AiHandler) TestProvider(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	db := core.GetDB()
	var p model.SysAiProvider
	if err := db.First(&p, id).Error; err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "供应商不存在"))
		return
	}
	apiKey, err := utils.DecryptSecret(getSecret(), p.ApiKeyEnc)
	if err != nil {
		c.JSON(http.StatusOK, response.Success(map[string]interface{}{"ok": false, "error": "密钥解密失败"}))
		return
	}
	baseURL := p.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL(p.ProviderType)
	}
	models, err := fetchModels(baseURL, apiKey)
	if err != nil {
		c.JSON(http.StatusOK, response.Success(map[string]interface{}{"ok": false, "error": err.Error()}))
		return
	}
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{"ok": true, "models": models}))
}