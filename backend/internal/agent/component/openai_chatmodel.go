package component

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type OpenAIChatModelConfig struct {
	BaseURL     string
	APIKey      string
	Model       string
	Timeout     time.Duration
	Temperature float32
	MaxTokens   int
}

type OpenAIChatModel struct {
	baseURL        string
	apiKey         string
	model          string
	httpClient     *http.Client
	defaultOptions model.Options
	boundTools     []*schema.ToolInfo
}

func NewOpenAIChatModel(cfg OpenAIChatModelConfig) *OpenAIChatModel {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &OpenAIChatModel{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		defaultOptions: model.Options{
			Temperature: floatPtr(cfg.Temperature),
			MaxTokens:   intPtr(cfg.MaxTokens),
		},
	}
}

func (m *OpenAIChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	options := model.GetCommonOptions(&m.defaultOptions, opts...)
	reqBody := map[string]any{
		"model":    valueOrDefault(options.Model, m.model),
		"messages": toOpenAIMessages(input),
	}

	if options.Temperature != nil {
		reqBody["temperature"] = *options.Temperature
	}
	if options.MaxTokens != nil && *options.MaxTokens > 0 {
		reqBody["max_tokens"] = *options.MaxTokens
	}

	tools := m.boundTools
	if len(options.Tools) > 0 {
		tools = options.Tools
	}
	if len(tools) > 0 {
		toolPayload, err := toOpenAITools(tools, options.AllowedToolNames)
		if err != nil {
			return nil, err
		}
		reqBody["tools"] = toolPayload
		reqBody["tool_choice"] = toOpenAIToolChoice(options.ToolChoice)
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(m.baseURL, "/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.apiKey)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("chat model http %d: %s", resp.StatusCode, string(respBody))
	}

	var result openAIChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty chat completion response")
	}

	choice := result.Choices[0]
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: choice.Message.Content,
	}
	if len(choice.Message.ToolCalls) > 0 {
		msg.ToolCalls = make([]schema.ToolCall, 0, len(choice.Message.ToolCalls))
		for _, call := range choice.Message.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
				ID:   call.ID,
				Type: call.Type,
				Function: schema.FunctionCall{
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				},
			})
		}
	}
	msg.ResponseMeta = &schema.ResponseMeta{
		FinishReason: choice.FinishReason,
		Usage: &schema.TokenUsage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}
	return msg, nil
}

func (m *OpenAIChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *OpenAIChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	clone := *m
	clone.boundTools = append([]*schema.ToolInfo(nil), tools...)
	return &clone, nil
}

func (m *OpenAIChatModel) GetType() string {
	return "OpenAICompatibleChatModel"
}

type openAIChatCompletionResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func toOpenAIMessages(messages []*schema.Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		item := map[string]any{
			"role":    string(message.Role),
			"content": message.Content,
		}
		if len(message.ToolCalls) > 0 {
			toolCalls := make([]map[string]any, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				toolCalls = append(toolCalls, map[string]any{
					"id":   call.ID,
					"type": valueOrDefaultString(call.Type, "function"),
					"function": map[string]any{
						"name":      call.Function.Name,
						"arguments": call.Function.Arguments,
					},
				})
			}
			item["tool_calls"] = toolCalls
		}
		if message.ToolCallID != "" {
			item["tool_call_id"] = message.ToolCallID
		}
		if message.ToolName != "" {
			item["name"] = message.ToolName
		}
		out = append(out, item)
	}
	return out
}

func toOpenAITools(infos []*schema.ToolInfo, allowed []string) ([]map[string]any, error) {
	allowedSet := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = true
	}

	out := make([]map[string]any, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		if len(allowedSet) > 0 && !allowedSet[info.Name] {
			continue
		}
		jsonSchema, err := info.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        info.Name,
				"description": info.Desc,
				"parameters":  jsonSchema,
			},
		})
	}
	return out, nil
}

func toOpenAIToolChoice(choice *schema.ToolChoice) string {
	if choice == nil {
		return "auto"
	}
	switch *choice {
	case schema.ToolChoiceForbidden:
		return "none"
	case schema.ToolChoiceForced:
		return "required"
	default:
		return "auto"
	}
}

func joinURL(baseURL, path string) string {
	if baseURL == "" {
		return path
	}
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	return baseURL + path
}

func floatPtr(value float32) *float32 {
	if value == 0 {
		return nil
	}
	return &value
}

func intPtr(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func valueOrDefault(value *string, defaultValue string) string {
	if value != nil && *value != "" {
		return *value
	}
	return defaultValue
}

func valueOrDefaultString(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

