package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gin-apeadmin/internal/core"
	"gin-apeadmin/internal/model"
	"gin-apeadmin/internal/pkg/response"
	"gin-apeadmin/internal/pkg/utils"
)

const aiSystemPrompt = `你是 ApeAdmin 智能管理助手，可以帮助用户通过自然语言操作后台管理系统。

你的能力：
1. 用户管理：查询用户列表、查看用户详情、创建用户、更新用户信息、删除用户
2. 角色管理：查询角色列表、创建角色、更新角色、删除角色
3. 部门管理：查询部门树、创建部门、更新部门、删除部门
4. 菜单管理：查询菜单树、创建菜单项、更新菜单、删除菜单
5. 系统工具：系统健康检查、查看已安装插件列表、系统统计信息

使用规则：
- 当用户的请求涉及系统操作时，调用对应的工具完成任务
- 工具调用后，用简洁的中文总结执行结果
- 如果权限不足，告知用户并说明需要什么权限
- 对于普通对话，直接友好地回答
- 创建/更新操作时，如果用户未提供必填参数，先询问用户`

// aiTools LLM function calling 工具定义（与 Python 版对齐）
var aiTools = []map[string]interface{}{
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "get_user_list",
			"description": "获取系统用户列表（分页）。返回用户ID、用户名、昵称、邮箱、手机、状态、角色信息。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"page":      map[string]interface{}{"type": "integer", "description": "页码，默认1"},
					"page_size": map[string]interface{}{"type": "integer", "description": "每页数量，默认10"},
					"keyword":   map[string]interface{}{"type": "string", "description": "搜索关键词（用户名或昵称）"},
				},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "get_user_detail",
			"description": "获取单个用户的详细信息，包括角色和部门。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user_id": map[string]interface{}{"type": "integer", "description": "用户ID"},
				},
				"required": []string{"user_id"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "create_user",
			"description": "创建新用户。需要提供用户名和密码。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"username": map[string]interface{}{"type": "string", "description": "用户名（唯一）"},
					"nickname": map[string]interface{}{"type": "string", "description": "昵称"},
					"password": map[string]interface{}{"type": "string", "description": "密码（至少6位）"},
					"email":    map[string]interface{}{"type": "string", "description": "邮箱"},
					"phone":    map[string]interface{}{"type": "string", "description": "手机号"},
				},
				"required": []string{"username", "password"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "update_user",
			"description": "更新用户信息。只传需要修改的字段。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user_id":  map[string]interface{}{"type": "integer", "description": "用户ID"},
					"nickname": map[string]interface{}{"type": "string", "description": "昵称"},
					"email":    map[string]interface{}{"type": "string", "description": "邮箱"},
					"phone":    map[string]interface{}{"type": "string", "description": "手机号"},
					"status":   map[string]interface{}{"type": "integer", "description": "状态: 0=禁用 1=启用"},
				},
				"required": []string{"user_id"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "delete_user",
			"description": "删除用户（软删除）。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user_id": map[string]interface{}{"type": "integer", "description": "用户ID"},
				},
				"required": []string{"user_id"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "get_system_stats",
			"description": "获取系统统计信息：用户总数、角色总数、菜单总数、部门总数。",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	},
}

// aiToolPermissions 工具 -> 所需权限
var aiToolPermissions = map[string][]string{
	"get_user_list":    {"system:user:list"},
	"get_user_detail":  {"system:user:list"},
	"create_user":      {"system:user:add"},
	"update_user":      {"system:user:edit"},
	"delete_user":      {"system:user:delete"},
	"get_system_stats": {},
}

// resolveProvider 解析要使用的供应商
func resolveProvider(providerID uint) (*model.SysAiProvider, error) {
	gdb := core.GetDB()
	if providerID > 0 {
		var p model.SysAiProvider
		if err := gdb.First(&p, providerID).Error; err != nil {
			return nil, fmt.Errorf("指定的模型供应商不存在")
		}
		if !p.Enabled {
			return nil, fmt.Errorf("该模型供应商已被禁用")
		}
		return &p, nil
	}
	var p model.SysAiProvider
	if err := gdb.Where("enabled = ?", true).Order("sort ASC, id ASC").First(&p).Error; err != nil {
		return nil, fmt.Errorf("未配置任何可用的模型供应商，请先在「模型密钥管理」中添加")
	}
	return &p, nil
}

// Chat 非流式对话
func (h *AiHandler) Chat(c *gin.Context) {
	var req struct {
		Messages    []map[string]string `json:"messages"`
		ProviderID  uint                `json:"provider_id"`
		Model       string              `json:"model"`
		EnableTools bool                `json:"enable_tools"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	provider, err := resolveProvider(req.ProviderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, err.Error()))
		return
	}
	// 非流式复用流式逻辑不可行（需完整工具循环），这里做简化：仅透传一次 LLM 调用
	result, err := runChatOnce(provider, req.Model, req.Messages, req.EnableTools, c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// ChatStream SSE 流式对话
func (h *AiHandler) ChatStream(c *gin.Context) {
	var req struct {
		Messages    []map[string]string `json:"messages"`
		ProviderID  uint                `json:"provider_id"`
		Model       string              `json:"model"`
		EnableTools bool                `json:"enable_tools"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	provider, err := resolveProvider(req.ProviderID)
	if err != nil {
		writeSSEError(c, err.Error())
		return
	}
	streamChat(c, provider, req.Model, req.Messages, req.EnableTools)
}

// writeSSEError 输出 SSE 错误事件
func writeSSEError(c *gin.Context, msg string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	payload, _ := json.Marshal(map[string]string{"type": "error", "message": msg})
	c.String(http.StatusOK, "data: %s\n\n", string(payload))
}

// providerBaseURL 获取供应商 base_url（默认兜底）
func providerBaseURL(p *model.SysAiProvider) string {
	if p.BaseURL != "" {
		return p.BaseURL
	}
	return defaultBaseURL(p.ProviderType)
}

// getAPIKey 解密 API Key
func getAPIKey(p *model.SysAiProvider) (string, error) {
	return utils.DecryptSecret(getSecret(), p.ApiKeyEnc)
}

// buildLLMRequest 构造 LLM 请求体
func buildLLMMessages(messages []map[string]string, toolsEnabled bool) ([]map[string]interface{}, []map[string]interface{}) {
	llmMessages := []map[string]interface{}{
		{"role": "system", "content": aiSystemPrompt},
	}
	for _, m := range messages {
		llmMessages = append(llmMessages, map[string]interface{}{"role": m["role"], "content": m["content"]})
	}
	var tools []map[string]interface{}
	if toolsEnabled {
		tools = aiTools
	}
	return llmMessages, tools
}

// callLLMOnce 单次调用 LLM（非流式）
func callLLMOnce(llmMessages []map[string]interface{}, baseURL, apiKey, model string, tools []map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"model":       model,
		"messages":    llmMessages,
		"max_tokens":  2000,
		"temperature": 0.7,
		"stream":      false,
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	payload, _ := json.Marshal(body)
	url := strings.TrimRight(baseURL, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("模型接口返回 %d: %s", resp.StatusCode, string(bodyBytes)[:minInt(300, len(bodyBytes))])
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// chatNonStreamOnce 非流式对话（工具循环最多 5 轮）
func chatNonStreamOnce(provider *model.SysAiProvider, model string, messages []map[string]string, enableTools bool, c *gin.Context) (map[string]interface{}, error) {
	baseURL := providerBaseURL(provider)
	apiKey, err := getAPIKey(provider)
	if err != nil {
		return nil, fmt.Errorf("API Key 解密失败")
	}
	modelName := model
	if modelName == "" {
		modelName = defaultModel(provider.ProviderType)
	}
	llmMessages, tools := buildLLMMessages(messages, enableTools)
	userPerms := getCurrentUserPerms(c)
	for round := 0; round < 5; round++ {
		resp, err := callLLMOnce(llmMessages, baseURL, apiKey, modelName, tools)
		if err != nil {
			return nil, err
		}
		msg := getRespMessage(resp)
		toolCalls := getToolCalls(msg)
		if len(toolCalls) == 0 {
			return map[string]interface{}{
				"content":    getString(msg, "content"),
				"tool_calls": []interface{}{},
				"usage":      getMap(resp, "usage"),
			}, nil
		}
		llmMessages = append(llmMessages, msg)
		for _, tc := range toolCalls {
			fnName := getString(getMap(tc, "function"), "name")
			fnArgs := getMap(getMap(tc, "function"), "arguments")
			argsJSON, _ := json.Marshal(fnArgs)
			result := executeAITool(fnName, string(argsJSON), userPerms, c)
			llmMessages = append(llmMessages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": getString(tc, "id"),
				"content":      result,
			})
		}
	}
	return map[string]interface{}{
		"content":  "抱歉，工具调用轮次过多，请简化您的请求。",
		"tool_calls": []interface{}{},
		"usage":    map[string]interface{}{},
	}, nil
}

// streamChat SSE 流式对话
func streamChat(c *gin.Context, provider *model.SysAiProvider, model string, messages []map[string]string, enableTools bool) {
	baseURL := providerBaseURL(provider)
	apiKey, err := getAPIKey(provider)
	if err != nil {
		writeSSEError(c, "API Key 解密失败")
		return
	}
	modelName := model
	if modelName == "" {
		modelName = defaultModel(provider.ProviderType)
	}
	llmMessages, tools := buildLLMMessages(messages, enableTools)
	userPerms := getCurrentUserPerms(c)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeSSEError(c, "SSE 不可用")
		return
	}
	sendEvent := func(v interface{}) {
		payload, _ := json.Marshal(v)
		c.Writer.Write([]byte("data: " + string(payload) + "\n\n"))
		flusher.Flush()
	}

	for round := 0; round < 5; round++ {
		collectedContent := ""
		collectedToolCalls := []map[string]interface{}{}
		hasToolCalls := false

		// 流式请求 LLM
		resp, err := streamLLMRequest(llmMessages, baseURL, apiKey, modelName, tools)
		if err != nil {
			sendEvent(map[string]string{"type": "error", "message": err.Error()})
			return
		}
		reader := bufio.NewReader(resp.Body)
		for {
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
			if err := json.Unmarshal([]byte(chunkStr), &chunk); err != nil {
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
				sendEvent(map[string]string{"type": "content", "content": content})
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

		if !hasToolCalls {
			sendEvent(map[string]string{"type": "done"})
			return
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
			fn := tc["function"].(map[string]interface{})
			fnName := getString(fn, "name")
			argsStr := getString(fn, "arguments")
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
				args = map[string]interface{}{}
			}
			sendEvent(map[string]interface{}{"type": "tool_call", "name": fnName, "arguments": args})
			result := executeAITool(fnName, args, userPerms, c)
			var resultJSON interface{}
			if err := json.Unmarshal([]byte(result), &resultJSON); err != nil {
				resultJSON = result
			}
			sendEvent(map[string]interface{}{"type": "tool_result", "name": fnName, "result": resultJSON})
			llmMessages = append(llmMessages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": getString(tc, "id"),
				"content":      result,
			})
		}
	}
	sendEvent(map[string]string{"type": "done"})
}