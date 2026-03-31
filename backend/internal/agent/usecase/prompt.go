//go:build legacy_agent

package usecase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// ==================== Prompt 模板 ====================

const systemPrompt = `你是抖音商城的 AI 客服助手。请遵守以下规则：
1. 仅回答与电商相关的问题（商品、订单、物流、退款、活动等），拒绝无关问题
2. 若提供了【知识库上下文】，优先以其中的平台专属信息（政策、规则等）为准；知识库没有的内容可以用你自己的知识回答
3. 不要编造具体的订单号、价格、活动时间等业务数据
4. 回复要简洁专业，不超过 200 字
5. 输出必须严格使用以下结构：
	<自然语言回复>
	===META===
	{"confidence":0~1之间小数,"emotion":"neutral|mild_frustration|angry|urgent","suggested_questions":["问题1","问题2","问题3"]}
6. 只允许出现一个 ===META=== 分隔符，且必须放在回复末尾
7. 不要输出 markdown 代码块或额外解释`

const toolSystemPrompt = `你是抖音商城的 AI 购物助手，可以帮助用户搜索商品、查看详情、加购物车、下单和查询订单。

## 核心规则
1. 你只负责理解用户意图、选择工具、解释结果。
2. 绝不编造价格、库存、订单号等业务数据——一切数据必须来自工具返回。
3. 绝不执行价格计算、折扣判断、库存扣减等业务逻辑，这些由后端服务处理。
4. 所有 ID（product_id、user_id、order_id）由系统自动填充，你无需传递也无需记忆。

## 工具使用策略
- 用户想找商品 → search_products(query)
- 用户问"第一个"详情 → get_product_detail(product_ref="list_0")，以此类推 list_1、list_2
- 用户问"这个/当前商品"详情 → get_product_detail(product_ref="current")
- 用户想加购"第一个" → add_to_cart(product_ref="list_0", quantity=N)
- 用户想加购当前商品/「买这个」「再来一个」→ add_to_cart(product_ref="current", quantity=N)
- 用户想看购物车 / 准备结算 → get_cart
- 用户说「立即下单」「买这个」→ create_order(source="product", product_ref="current")
- 用户说「结算」「下单全部」→ create_order(source="cart")
- 用户查订单 → get_order，可填用户说的订单号，或不填查最近订单

## 输出格式
输出必须严格使用以下结构：
<自然语言回复>
===META===
{"confidence":0~1之间小数,"emotion":"neutral|mild_frustration|angry|urgent","suggested_questions":["问题1","问题2","问题3"]}
只允许出现一个 ===META=== 分隔符，且必须放在回复末尾。

## 商品展示规范
- 搜索或介绍商品时，用有序列表展示，每条包含名称、价格，并附上查看链接
- 链接格式：[查看详情](http://localhost:5173/product/{product_id})
- product_id 来自工具返回的 product_id 字段，必须原样填入，不得编造
- 示例：
  1. **索尼WH-1000XM5耳机** - ¥2,299 [查看详情](http://localhost:5173/product/42)
  2. **苹果AirPods Pro** - ¥1,799 [查看详情](http://localhost:5173/product/17)`

// formatStateContext 将对话状态格式化为 system prompt 注入块
// 只暴露商品名称，不暴露任何 ID——ID 由 resolveToolArgs 在 backend 侧处理
func formatStateContext(state *domain.EntityMemory) string {
	if state == nil {
		return ""
	}
	hasEntities := len(state.ProductList) > 0 ||
		state.CurrentProductID != "" ||
		state.LastOrderID != ""
	if !hasEntities {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("【对话状态】\n")
	if len(state.ProductList) > 0 {
		sb.WriteString("最近搜索结果（用 product_ref=\"list_N\" 引用，N 从 0 开始）：\n")
		for i, p := range state.ProductList {
			sb.WriteString(fmt.Sprintf("  list_%d: %s\n", i, p.Name))
		}
	}
	if state.CurrentProductID != "" {
		name := state.CurrentProductName
		if name == "" {
			name = "某商品"
		}
		sb.WriteString(fmt.Sprintf("当前商品 product_ref=\"current\": %s\n", name))
	}
	if state.LastOrderID != "" {
		sb.WriteString("最近有一笔订单（直接说「查我的订单」即可，无需填写订单号）\n")
	}
	return sb.String()
}

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

const handoffPrompt = `请基于以下客服对话，生成一份结构化的交接摘要，帮助人工客服快速接手。

要求输出 JSON（不要 markdown 代码块）：
{
  "core_issue": "一句话概括用户的核心诉求",
  "ai_actions": ["列举 AI 已经做了什么（关键动作）"],
  "escalation_reason": "为什么需要转人工",
  "user_emotion": "neutral / mild_frustration / angry / urgent",
  "entities": {"order_id":"", "product":"", "problem_type":""}
}

如果对话中没有订单号或商品名，entities 对应字段置空字符串。

对话记录：
%s`

const metaEvalPrompt = `你是客服质检模型。请基于对话上下文与助手最终回复，评估本轮回复质量并输出 JSON。

要求：
1) 仅输出 JSON，不要输出 markdown 代码块
2) confidence 必须是 0~1 之间小数
3) emotion 只能是：neutral / mild_frustration / angry / urgent
4) suggested_questions 给 0~3 个可继续追问的问题，尽量简洁

输出格式：
{
	"confidence": 0.0,
	"emotion": "neutral",
	"suggested_questions": ["..."]
}

最近对话：
%s

用户本轮输入：
%s

助手本轮回复：
%s`

// ==================== 解析工具 ====================

// parseReply 解析 LLM 回复为结构化结果
// 主路径：模型输出 <reply> + ===META=== + JSON
// 兼容路径：模型只输出自然语言时，confidence/emotion 使用默认值
func parseReply(content string) *domain.GenerationResult {
	content = strings.TrimSpace(content)
	result := &domain.GenerationResult{Reply: content, Confidence: 0.75, Emotion: "neutral", MetaSource: "default"}

	const sep = "===META==="
	if idx := strings.Index(content, sep); idx >= 0 {
		replyText := strings.TrimSpace(content[:idx])
		metaStr := cleanJSON(strings.TrimSpace(content[idx+len(sep):]))
		var meta struct {
			Confidence         float32  `json:"confidence"`
			Emotion            string   `json:"emotion"`
			SuggestedQuestions []string `json:"suggested_questions"`
		}
		if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
			confOK := meta.Confidence >= 0 && meta.Confidence <= 1
			emotion := strings.TrimSpace(meta.Emotion)
			emotionOK := isValidEmotion(emotion)
			if confOK {
				result.Confidence = meta.Confidence
			}
			if emotionOK {
				result.Emotion = emotion
			}
			result.Suggested = meta.SuggestedQuestions
			if confOK && emotionOK {
				result.MetaSource = "inline"
			}
		}
		if replyText != "" {
			result.Reply = replyText
		}
		// replyText 为空时保持 result.Reply = content（整体内容兜底）
	}
	return result
}

func isValidEmotion(v string) bool {
	switch strings.TrimSpace(v) {
	case "neutral", "mild_frustration", "angry", "urgent":
		return true
	default:
		return false
	}
}

// defaultHandoff 降级摘要：拼接最近对话原文
func defaultHandoff(history []domain.Message) *domain.HandoffSummary {
	var sb strings.Builder
	for _, m := range history {
		fmt.Fprintf(&sb, "%s: %s\n", m.Role, m.Content)
	}
	return &domain.HandoffSummary{
		CoreIssue:        sb.String(),
		AIActions:        []string{"已尝试 AI 自动回复"},
		EscalationReason: "AI 无法准确解答，需要人工介入",
		UserEmotion:      "unknown",
		Entities:         map[string]string{},
	}
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

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
