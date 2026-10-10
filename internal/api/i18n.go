package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/middleware"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/response"
)

// ────────────────────────────────────────────────────────────────────────────
// 公开接口（免登录）：登录页/前端 store 启动时就要用
// ────────────────────────────────────────────────────────────────────────────

// GetI18nLocales 已启用语言列表（公开）
func GetI18nLocales(c *gin.Context) {
	locales, err := dal.ListEnabledLocales()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(locales))
}

// GetI18nMessages 某语言全量翻译（公开）
// 返回 { namespace: { key: value } }，与前端 locales 文件组织一致，可直接 mergeLocaleMessage
func GetI18nMessages(c *gin.Context) {
	localeCode := c.Param("locale")

	// 校验语言是否启用（未注册语言返回 404，防止任意 locale 扫库）
	if _, err := dal.GetLocale(localeCode); err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "common.not_found", map[string]string{"name": core.T(middleware.GetLocale(c), "i18n.locale")}))
		return
	}

	list, err := dal.ListAllTranslations(localeCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.query_fail", nil))
		return
	}

	// 组装 { ns: { key: value } }
	result := map[string]map[string]string{}
	for _, t := range list {
		if result[t.Namespace] == nil {
			result[t.Namespace] = map[string]string{}
		}
		result[t.Namespace][t.Key] = t.Value
	}

	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, response.Success(result))
}

// ────────────────────────────────────────────────────────────────────────────
// 管理端：语言管理 CRUD
// ────────────────────────────────────────────────────────────────────────────

type LocaleHandler struct{}

type localeReq struct {
	Code      string `json:"code" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Icon      string `json:"icon"`
	IsEnabled int    `json:"is_enabled"`
	IsDefault int    `json:"is_default"`
	Sort      int    `json:"sort"`
}

// List 语言列表（含停用）
func (h *LocaleHandler) List(c *gin.Context) {
	locales, err := dal.ListLocales()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorT(500, middleware.GetLocale(c), "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(locales))
}

// Create 新增语言
func (h *LocaleHandler) Create(c *gin.Context) {
	var req localeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorT(400, middleware.GetLocale(c), "common.param_error", nil))
		return
	}
	if _, err := dal.GetLocale(req.Code); err == nil {
		c.JSON(http.StatusBadRequest, response.ErrorT(400, middleware.GetLocale(c), "common.exists", map[string]string{"name": "语言"}))
		return
	}

	l := model.SysLocale{
		Code: req.Code, Name: req.Name, Icon: req.Icon,
		IsEnabled: req.IsEnabled, IsDefault: 0, Sort: req.Sort,
	}
	if err := dal.CreateLocale(&l); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorT(500, middleware.GetLocale(c), "common.create_fail", nil))
		return
	}

	// 设为默认时先清掉其他默认
	if req.IsDefault == 1 {
		_ = dal.ResetDefaultLocale(l.ID)
		dal.UpdateLocale(&model.SysLocale{BaseModel: model.BaseModel{ID: l.ID}, IsDefault: 1})
	}
	core.LoadI18nFromDB(core.GetDB())
	c.JSON(http.StatusOK, response.Success(l))
}

// Update 编辑语言
func (h *LocaleHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req localeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorT(400, middleware.GetLocale(c), "common.param_error", nil))
		return
	}
	l, err := dal.GetLocaleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorT(404, middleware.GetLocale(c), "common.not_found", map[string]string{"name": "语言"}))
		return
	}

	// code 不允许修改（翻译条目按 code 关联）
	l.Name = req.Name
	l.Icon = req.Icon
	l.IsEnabled = req.IsEnabled
	l.Sort = req.Sort
	if err := dal.UpdateLocale(l); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorT(500, middleware.GetLocale(c), "common.update_fail", nil))
		return
	}

	if req.IsDefault == 1 && l.IsDefault != 1 {
		_ = dal.ResetDefaultLocale(l.ID)
		l.IsDefault = 1
		_ = dal.UpdateLocale(l)
	} else if req.IsDefault != 1 {
		l.IsDefault = 0
		_ = dal.UpdateLocale(l)
	}

	core.LoadI18nFromDB(core.GetDB())
	c.JSON(http.StatusOK, response.Success(l))
}

// Delete 删除语言（连带清翻译；路径参数为语言 code，如 zh-CN）
func (h *LocaleHandler) Delete(c *gin.Context) {
	code := c.Param("code")
	l, err := dal.GetLocale(code)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorT(404, middleware.GetLocale(c), "common.not_found", map[string]string{"name": core.T(middleware.GetLocale(c), "i18n.locale")}))
		return
	}
	if l.IsDefault == 1 {
		c.JSON(http.StatusBadRequest, response.ErrorT(400, middleware.GetLocale(c), "i18n.default_cannot_delete", nil))
		return
	}
	if err := dal.DeleteLocale(code); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorT(500, middleware.GetLocale(c), "common.delete_fail", nil))
		return
	}
	core.LoadI18nFromDB(core.GetDB())
	c.JSON(http.StatusOK, response.SuccessT(middleware.GetLocale(c), "common.delete_ok", nil))
}

// ────────────────────────────────────────────────────────────────────────────
// 管理端：翻译维护
// ────────────────────────────────────────────────────────────────────────────

type TranslationHandler struct{}

// List 翻译查询（?locale=zh-CN&namespace=menu）
func (h *TranslationHandler) List(c *gin.Context) {
	localeCode := c.Query("locale")
	namespace := c.Query("namespace")
	if localeCode == "" {
		c.JSON(http.StatusBadRequest, response.ErrorT(400, middleware.GetLocale(c), "common.param_error", nil))
		return
	}
	list, err := dal.ListTranslations(localeCode, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorT(500, middleware.GetLocale(c), "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(list))
}

type transBatchReq struct {
	LocaleCode string              `json:"locale_code" binding:"required"`
	Items      []model.SysTranslation `json:"items" binding:"required"`
}

// BatchSave 批量保存翻译
func (h *TranslationHandler) BatchSave(c *gin.Context) {
	var req transBatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorT(400, middleware.GetLocale(c), "common.param_error", nil))
		return
	}
	if _, err := dal.GetLocale(req.LocaleCode); err != nil {
		c.JSON(http.StatusNotFound, response.ErrorT(404, middleware.GetLocale(c), "common.not_found", map[string]string{"name": "语言"}))
		return
	}
	for i := range req.Items {
		req.Items[i].LocaleCode = req.LocaleCode
	}
	if err := dal.UpsertTranslations(req.Items); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorT(500, middleware.GetLocale(c), "common.update_fail", nil))
		return
	}
	// 热刷新覆盖层（后端消息立即生效）
	core.LoadI18nFromDB(core.GetDB())
	c.JSON(http.StatusOK, response.SuccessT(middleware.GetLocale(c), "common.update_ok", nil))
}

// Import JSON 导入翻译（格式同导出：[{namespace, key, value}]）
func (h *TranslationHandler) Import(c *gin.Context) {
	var req transBatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorT(400, middleware.GetLocale(c), "common.param_error", nil))
		return
	}
	if _, err := dal.GetLocale(req.LocaleCode); err != nil {
		c.JSON(http.StatusNotFound, response.ErrorT(404, middleware.GetLocale(c), "common.not_found", map[string]string{"name": "语言"}))
		return
	}
	for i := range req.Items {
		req.Items[i].LocaleCode = req.LocaleCode
	}
	if err := dal.UpsertTranslations(req.Items); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorT(500, middleware.GetLocale(c), "common.update_fail", nil))
		return
	}
	core.LoadI18nFromDB(core.GetDB())
	c.JSON(http.StatusOK, response.SuccessT(middleware.GetLocale(c), "common.update_ok", nil))
}

// Export 导出某语言全量翻译 JSON
func (h *TranslationHandler) Export(c *gin.Context) {
	localeCode := c.Query("locale")
	if localeCode == "" {
		c.JSON(http.StatusBadRequest, response.ErrorT(400, middleware.GetLocale(c), "common.param_error", nil))
		return
	}
	list, err := dal.ListAllTranslations(localeCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorT(500, middleware.GetLocale(c), "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(list))
}
