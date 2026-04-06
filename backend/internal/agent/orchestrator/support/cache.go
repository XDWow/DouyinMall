package support

import (
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

const (
	CacheHitExact    = "exact"
	CacheHitSemantic = "semantic"
)

// DetectCacheIntent 是缓存前的轻量中文分类
// 它只服务缓存分流，不替代后面的正式意图识别
func DetectCacheIntent(message string) domain.Intent {
	msg := normalizeChineseQuery(message)
	if msg == "" {
		return domain.IntentFallback
	}

	switch {
	case isAfterSaleApplyQuery(msg):
		return domain.IntentReturnExchangeApply
	case isReturnPolicyQuery(msg):
		return domain.IntentReturnPolicy
	case isAddToCartQuery(msg):
		return domain.IntentAddToCart
	case isInventoryQuery(msg):
		return domain.IntentInventoryQuery
	case isOrderQuery(msg):
		return domain.IntentOrderQuery
	case isProductInfoQuery(msg):
		return domain.IntentProductInfo
	default:
		return domain.IntentFallback
	}
}

// CacheIntentBucket 把缓存前识别出的意图映射成语义缓存分桶
// 这里只保留真正适合做公共语义复用的知识类桶
func CacheIntentBucket(intent domain.Intent) string {
	switch intent {
	case domain.IntentReturnPolicy:
		return string(domain.IntentReturnPolicy)
	case domain.IntentProductInfo:
		return string(domain.IntentProductInfo)
	case domain.IntentFallback:
		return string(domain.IntentFallback)
	default:
		return string(domain.IntentFallback)
	}
}

// CacheScopeForIntent 决定语义缓存的复用范围
// tenant_public: 同租户共享，适合政策、FAQ、通用商品咨询
// tenant_user: 只给当前用户自己复用，适合带用户上下文的结果
func CacheScopeForIntent(intent domain.Intent) cache.CacheScope {
	switch intent {
	case domain.IntentReturnPolicy, domain.IntentProductInfo, domain.IntentFallback:
		return cache.CacheScopeTenantPublic
	default:
		return cache.CacheScopeTenantUser
	}
}

// AllowSemanticCache 只让稳定知识问答进入语义缓存
func AllowSemanticCache(intent domain.Intent, message string) bool {
	switch intent {
	case domain.IntentReturnPolicy, domain.IntentProductInfo:
		return strings.TrimSpace(message) != ""
	case domain.IntentFallback:
		return strings.TrimSpace(message) != "" && DigitsOnlyID(message) == ""
	default:
		return false
	}
}

func normalizeChineseQuery(message string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(message)), "")
}

func isAfterSaleApplyQuery(message string) bool {
	return containsAny(message,
		"帮我申请退货", "帮我申请退款", "帮我申请换货",
		"我要申请退货", "我要申请退款", "我要申请换货",
		"发起退货", "发起退款", "发起换货",
		"提交退货", "提交退款", "提交换货",
		"售后申请", "申请售后",
	)
}

func isReturnPolicyQuery(message string) bool {
	if containsAny(message,
		"退货政策", "退款政策", "换货政策", "售后政策",
		"退货规则", "退款规则", "换货规则",
		"退货流程", "退款流程", "换货流程",
		"怎么退货", "怎么退款", "怎么换货",
		"如何退货", "如何退款", "如何换货",
		"退货怎么操作", "退款怎么操作", "换货怎么操作",
		"退货怎么弄", "退款怎么弄", "换货怎么弄",
		"退货要多久", "退款要多久", "换货要多久",
		"退款多久到账", "退货多久到账", "换货多久到账",
		"退货运费谁承担", "退款多久到账", "七天无理由",
	) {
		return true
	}

	// “我要退货，具体怎么做”
	return containsAny(message, "我要退货", "我要退款", "我要换货") &&
		containsAny(message, "怎么做", "怎么办", "怎么操作", "怎么处理", "流程", "步骤", "具体")
}

func isAddToCartQuery(message string) bool {
	return containsAny(message, "加入购物车", "加购", "放购物车", "加入购物袋")
}

func isInventoryQuery(message string) bool {
	return containsAny(message, "库存", "有货吗", "还有货吗", "现货", "还有没有", "能不能买")
}

func isOrderQuery(message string) bool {
	return containsAny(message, "订单", "物流", "发货", "配送", "快递", "运单", "什么时候到", "到哪了")
}

func isProductInfoQuery(message string) bool {
	return containsAny(message, "商品", "价格", "参数", "规格", "详情", "介绍", "材质", "颜色", "尺码")
}

func containsAny(message string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}
