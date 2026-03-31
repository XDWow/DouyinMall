package ai

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// 最小化指标接口，由 usecase.PipelineMetrics 隐式实现
type FallbackMetrics interface {
	IncTemplateFallback()
	IncLLMError()
}

// FallbackLLMClient 降级装饰器：只负责"节点A失败→试节点B→试模板"这一件事
// 每个节点已由 ResilientClient 装饰（限流 + 熔断 + 超时），FallbackLLMClient 不感知具体原因
type FallbackLLMClient struct {
	nodes    []CSLLMClient // 装饰 ResilientClient
	template *TemplateEngine
	metrics  FallbackMetrics
	logger   logger.LoggerV1
}

func NewFallbackLLMClient(log logger.LoggerV1, metrics FallbackMetrics, nodes ...CSLLMClient) *FallbackLLMClient {
	return &FallbackLLMClient{
		nodes:    nodes,
		template: NewTemplateEngine(),
		metrics:  metrics,
		logger:   log,
	}
}

// 带降级的 LLM 调用：依次尝试每个节点，全部失败走模板兜底
func (f *FallbackLLMClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	for i, node := range f.nodes {
		f.logger.Info("LLM 节点调用开始", logger.Int("node_index", i))
		resp, err := node.ChatCompletion(ctx, req)
		if err == nil {
			if i > 0 {
				f.logger.Info("降级成功", logger.Int("node_index", i))
			}
			return resp, nil
		}
		f.metrics.IncLLMError()
		f.logger.Warn("节点调用失败，尝试下一层",
			logger.Int("node_index", i),
			logger.Error(err))
	}

	f.logger.Error("所有 LLM 节点不可用，走模板兜底")
	f.metrics.IncTemplateFallback()

	// 模板兜底
	content := f.template.GenerateContent(req.Messages)
	f.logger.Info("模板兜底回复", logger.String("content_prefix", content[:min(len(content), 50)]))

	return &ChatResponse{
		ID:      "template-fallback",
		Created: 0,
		Choices: []pkgai.Choice{{
			Index: 0,
			Message: pkgai.Message{
				Role:    "assistant",
				Content: content,
			},
		}},
	}, nil
}

// 带降级的流式调用：依次尝试每个节点，全部失败走模板兜底
func (f *FallbackLLMClient) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	for i, node := range f.nodes {
		f.logger.Info("LLM 流式节点调用开始", logger.Int("node_index", i))
		ch, err := node.ChatCompletionStream(ctx, req)
		if err == nil {
			if i > 0 {
				f.logger.Info("降级成功（stream）", logger.Int("node_index", i))
			}
			return ch, nil
		}
		f.metrics.IncLLMError()
		f.logger.Warn("流式调用失败，尝试下一层",
			logger.Int("node_index", i),
			logger.Error(err))
	}

	f.logger.Error("所有 LLM 节点不可用，走模板兜底（stream）")
	f.metrics.IncTemplateFallback()

	// 模板兜底：返回单个 chunk 的流式响应
	content := f.template.GenerateContent(req.Messages)

	ch := make(chan ChatResponse, 1)
	finishReason := "stop"
	ch <- ChatResponse{
		ID:      "template-fallback",
		Created: 0,
		Choices: []pkgai.Choice{{
			Index:        0,
			FinishReason: &finishReason,
			Message: pkgai.Message{
				Role:    "assistant",
				Content: content,
			},
		}},
	}
	close(ch)
	return ch, nil
}

// 基于意图的模板回复，所有 LLM 节点不可用时兜底
// 先识别意图，再 意图 -> 模板答复+缓存好的问题推荐
type TemplateEngine struct {
	templates map[domain.IntentType]string
}

func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		templates: map[domain.IntentType]string{
			domain.IntentReturn:         "退货流程：请在订单详情页点击「申请退货」，选择退货原因后提交。7天无理由退货商品请在签收后7天内申请。\n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"退货运费谁承担\",\"退款多久到账\"]}",
			domain.IntentLogistics:      "物流查询：请在「我的订单」页面点击对应订单查看物流信息。如物流长时间未更新，建议联系人工客服。\n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"物流多久能到\",\"可以修改收货地址吗\"]}",
			domain.IntentPayment:        "支付问题：请检查支付方式是否正常，确认账户余额充足。如仍无法支付，建议更换支付方式或联系人工客服。\n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"支持哪些支付方式\",\"付款后多久发货\"]}",
			domain.IntentOrderInquiry:   "订单查询：请在「我的订单」页面查看订单状态。如有疑问，建议联系人工客服。\n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"如何取消订单\",\"订单状态说明\"]}",
			domain.IntentProductInquiry: "商品咨询：建议您查看商品详情页了解规格参数，或联系人工客服获取更多信息。\n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"有没有优惠活动\",\"支持七天无理由退货吗\"]}",
			domain.IntentFAQ:            "感谢您的咨询。如需帮助，请联系人工客服获取详细解答。\n===META===\n{\"confidence\":0.6,\"emotion\":\"neutral\",\"suggested_questions\":[\"如何联系客服\",\"常见问题在哪\"]}",
			domain.IntentComplaint:      "非常抱歉给您带来不便，您的问题我们已记录。建议您联系人工客服，我们会尽快为您处理。\n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"投诉处理要多久\",\"可以申请赔偿吗\"]}",
			domain.IntentPromotion:      "活动咨询：请关注首页活动专区了解最新优惠信息。如有具体问题，建议联系人工客服。\n===META===\n{\"confidence\":0.6,\"emotion\":\"neutral\",\"suggested_questions\":[\"当前有什么优惠\",\"优惠券怎么领取\"]}",
		},
	}
}

func (t *TemplateEngine) GenerateContent(messages []pkgai.Message) string {
	intent := t.detectIntent(messages)
	if tpl, ok := t.templates[intent]; ok {
		return tpl
	}
	return "抱歉，系统繁忙，请稍后重试或联系人工客服。\n===META===\n{\"confidence\":0.3,\"emotion\":\"neutral\",\"suggested_questions\":[]}"
}

func (t *TemplateEngine) detectIntent(messages []pkgai.Message) domain.IntentType {
	var userMsg string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			userMsg = messages[i].Content
			break
		}
	}
	if userMsg == "" {
		return domain.IntentUnknown
	}

	keywords := map[domain.IntentType][]string{
		domain.IntentReturn:         {"退货", "退款", "退换", "七天无理由", "退回"},
		domain.IntentLogistics:      {"物流", "快递", "配送", "发货", "运费", "到了吗"},
		domain.IntentPayment:        {"支付", "付款", "微信支付", "支付宝", "付不了"},
		domain.IntentOrderInquiry:   {"订单", "下单", "取消订单", "订单状态"},
		domain.IntentProductInquiry: {"商品", "产品", "规格", "尺码", "颜色", "库存"},
		domain.IntentComplaint:      {"投诉", "差评", "不满", "举报", "太差"},
		domain.IntentPromotion:      {"优惠", "活动", "折扣", "促销", "券", "满减"},
		domain.IntentFAQ:            {"怎么", "如何", "什么是", "帮助", "在哪"},
	}
	for intent, kws := range keywords {
		for _, kw := range kws {
			if strings.Contains(userMsg, kw) {
				return intent
			}
		}
	}
	return domain.IntentUnknown
}
