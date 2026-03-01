package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/search/infra/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// ==================== Prompt 模板 ====================

const systemPrompt = `你是抖音商城的 AI 客服助手。请遵守以下规则：
1. 仅回答与电商相关的问题（商品、订单、物流、退款、活动等）
2. 回答必须基于提供的【知识库上下文】，不可编造事实
3. 如果知识库中没有相关信息，诚实告知用户并建议转人工
4. 回复要简洁专业，不超过 200 字
5. 输出格式为 JSON：{"reply":"你的回复内容", "confidence":0.85}
6. confidence 低于 0.5 时，reply 中必须包含"建议您联系人工客服"`

const intentPrompt = `请分析用户的意图，同时将用户的口语化问题改写为更精准的检索查询。

返回 JSON（不要 markdown 代码块）：
{
  "intent": "INTENT_TYPE",
  "confidence": 0.0-1.0,
  "rewritten_query": "改写后适合检索的查询",
  "entities": {}
}

可选意图：FAQ, PRODUCT_INQUIRY, ORDER_INQUIRY, LOGISTICS, PAYMENT, RETURN, COMPLAINT, PROMOTION, CHITCHAT, TRANSFER_TO_HUMAN

改写规则：
- 解析指代词（"它""这个"）为具体实体
- 去掉口语化表达，保留核心语义
- 如果是简单问候/闲聊，rewritten_query 设为空字符串

对话历史（最近3轮）：
%s

用户消息：%s`

const rerankPrompt = `以下是候选知识条目，请根据与用户问题的相关性排序。
返回 JSON 数组，只包含最相关的 %d 条的序号（从 1 开始）：[1, 3, 5]

用户问题：%s

候选条目：
%s`

// ==================== 意图识别 ====================

func (uc *ChatUseCase) recognizeIntent(ctx context.Context, message string, history []domain.Message) (*domain.IntentResult, error) {
	// 拼装历史
	var historyStr string
	if len(history) > 0 {
		var sb strings.Builder
		for _, m := range history {
			fmt.Fprintf(&sb, "%s: %s\n", m.Role, m.Content)
		}
		historyStr = sb.String()
	} else {
		historyStr = "（无历史）"
	}

	resp, err := uc.llm.ChatCompletion(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: fmt.Sprintf(intentPrompt, historyStr, message)},
		},
		Temperature: 0.1,
		MaxTokens:   256,
	})
	if err != nil {
		return nil, err
	}

	content := cleanJSON(resp.Content)
	var parsed struct {
		Intent         string            `json:"intent"`
		Confidence     float32           `json:"confidence"`
		RewrittenQuery string            `json:"rewritten_query"`
		Entities       map[string]string `json:"entities"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		uc.logger.Warn("解析意图结果失败", logger.Error(err), logger.String("raw", content))
		return &domain.IntentResult{
			Type:           domain.IntentUnknown,
			RewrittenQuery: message,
		}, nil
	}

	return &domain.IntentResult{
		Type:           mapIntent(parsed.Intent),
		Confidence:     parsed.Confidence,
		RewrittenQuery: parsed.RewrittenQuery,
		Entities:       parsed.Entities,
	}, nil
}

// ==================== LLM 重排 ====================

func (uc *ChatUseCase) rerankByLLM(ctx context.Context, query string, candidates []domain.KnowledgeRef, topN int) []domain.KnowledgeRef {
	if len(candidates) <= topN {
		return candidates
	}

	// 构造候选列表
	var sb strings.Builder
	for i, c := range candidates {
		fmt.Fprintf(&sb, "%d. [%s] %s: %s\n", i+1, c.Category, c.Title, c.Snippet)
	}

	resp, err := uc.llm.ChatCompletion(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "user", Content: fmt.Sprintf(rerankPrompt, topN, query, sb.String())},
		},
		Temperature: 0.1,
		MaxTokens:   64,
	})
	if err != nil {
		uc.logger.Warn("LLM 重排失败，降级返回前 N 条", logger.Error(err))
		return candidates[:topN]
	}

	// 解析返回的序号数组
	content := cleanJSON(resp.Content)
	var indices []int
	if err := json.Unmarshal([]byte(content), &indices); err != nil {
		uc.logger.Warn("解析重排结果失败", logger.Error(err), logger.String("raw", content))
		return candidates[:topN]
	}

	result := make([]domain.KnowledgeRef, 0, topN)
	for _, idx := range indices {
		if idx >= 1 && idx <= len(candidates) {
			result = append(result, candidates[idx-1])
		}
		if len(result) >= topN {
			break
		}
	}
	if len(result) == 0 {
		return candidates[:topN]
	}
	return result
}

// ==================== 回复解析 ====================

func (uc *ChatUseCase) parseReply(content string) (reply string, confidence float32) {
	content = cleanJSON(content)
	var parsed struct {
		Reply      string  `json:"reply"`
		Confidence float32 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// 如果 LLM 没返回 JSON，直接用原始内容
		return content, 0.5
	}
	return parsed.Reply, parsed.Confidence
}

// ==================== 工具函数 ====================

// cleanJSON 清理 LLM 输出中可能包含的 markdown 代码块标记
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// mapIntent 字符串意图映射到枚举
func mapIntent(s string) domain.IntentType {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "FAQ":
		return domain.IntentFAQ
	case "PRODUCT_INQUIRY":
		return domain.IntentProductInquiry
	case "ORDER_INQUIRY":
		return domain.IntentOrderInquiry
	case "LOGISTICS":
		return domain.IntentLogistics
	case "PAYMENT":
		return domain.IntentPayment
	case "RETURN":
		return domain.IntentReturn
	case "COMPLAINT":
		return domain.IntentComplaint
	case "PROMOTION":
		return domain.IntentPromotion
	case "CHITCHAT":
		return domain.IntentChitchat
	case "TRANSFER_TO_HUMAN":
		return domain.IntentTransferToHuman
	default:
		return domain.IntentUnknown
	}
}
