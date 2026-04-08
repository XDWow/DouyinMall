package global

import (
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

const cacheHitExact = "exact"

func detectCacheIntent(message string) domain.Intent {
	msg := normalizeCacheQuery(message)
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
	case isStableProductKnowledgeQuery(msg):
		return domain.IntentProductInfo
	default:
		return domain.IntentFallback
	}
}

func cacheIntentBucket(intent domain.Intent) string {
	switch intent {
	case domain.IntentReturnPolicy:
		return string(domain.IntentReturnPolicy)
	case domain.IntentProductInfo:
		return string(domain.IntentProductInfo)
	default:
		return string(domain.IntentFallback)
	}
}

func cacheScopeForIntent(intent domain.Intent) cache.CacheScope {
	switch intent {
	case domain.IntentReturnPolicy, domain.IntentProductInfo:
		return cache.CacheScopeTenantPublic
	default:
		return cache.CacheScopeTenantUser
	}
}

func allowCacheLookup(intent domain.Intent, message string) bool {
	msg := normalizeCacheQuery(message)
	if msg == "" || isDynamicOrActionQuery(msg) {
		return false
	}

	switch intent {
	case domain.IntentReturnPolicy:
		return isReturnPolicyQuery(msg)
	case domain.IntentProductInfo:
		return isStableProductKnowledgeQuery(msg)
	default:
		return false
	}
}

func allowExactCache(intent domain.Intent, message string) bool {
	return allowCacheLookup(intent, message)
}

func normalizeCacheQuery(message string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(message)), ""))
}

func isAfterSaleApplyQuery(message string) bool {
	return containsAny(message,
		"帮我申请退货", "帮我申请退款", "帮我申请换货",
		"我要申请退货", "我要申请退款", "我要申请换货",
		"发起退货", "发起退款", "发起换货",
		"提交退货", "提交退款", "提交换货",
		"售后申请", "申请售后",
		"帮我退货", "帮我退款", "帮我换货",
		"申请退货", "申请退款", "申请换货",
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
		"退款多久到账", "退货多久到货", "换货多久到货",
		"退货运费谁承担", "七天无理由",
		"returnpolicy", "refundpolicy", "exchangepolicy",
		"returnflow", "refundflow", "exchangeflow",
	) {
		return true
	}

	return containsAny(message, "我要退货", "我要退款", "我要换货") &&
		containsAny(message, "怎么做", "怎么办", "怎么操作", "怎么处理", "流程", "步骤", "具体")
}

func isAddToCartQuery(message string) bool {
	return containsAny(message,
		"加入购物车", "加购", "放购物车", "加到购物车",
		"addtocart", "putincart",
	)
}

func isInventoryQuery(message string) bool {
	return containsAny(message,
		"库存", "有货吗", "还有货吗", "现货", "还有没有", "能不能买",
		"instock", "available",
	)
}

func isOrderQuery(message string) bool {
	return containsAny(message,
		"订单", "物流", "发货", "配送", "快递", "运单", "什么时候到", "到哪里",
		"order", "shipping", "delivery", "tracking",
	)
}

func isDynamicOrActionQuery(message string) bool {
	if isAddToCartQuery(message) || isInventoryQuery(message) || isOrderQuery(message) || isAfterSaleApplyQuery(message) {
		return true
	}

	return containsAny(message,
		"查订单", "查一下订单", "查下订单",
		"查物流", "查一下物流", "查下物流",
		"看物流", "看一下物流", "看下物流",
		"查快递", "催发货", "发货了吗",
		"当前状态", "进度", "什么时候发货",
		"补货", "还能买", "能买吗", "卖完了吗",
		"提交申请", "发起申请", "取消订单", "改地址", "修改地址",
		"trackorder", "orderstatus", "shipmentstatus",
	)
}

func isStableProductKnowledgeQuery(message string) bool {
	if containsAny(message,
		"价格", "多少钱", "优惠", "活动", "折扣",
		"库存", "有货", "现货", "补货", "发货", "到货",
		"price", "discount", "promotion", "stock",
	) {
		return false
	}

	return containsAny(message,
		"商品", "参数", "规格", "详情", "介绍", "材质", "面料", "成分",
		"颜色", "尺码", "尺寸", "重量", "用法", "怎么用", "适合", "区别",
		"product", "spec", "detail", "material", "size", "color",
	)
}

func containsAny(message string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}
