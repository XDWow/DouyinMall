package support

import (
	"fmt"
	"strings"
)

func BuildReturnApplySummaryFromSlots(slots map[string]any) string {
	requestType := afterSaleTypeLabelFromSlots(FirstNonEmpty(fmt.Sprint(slots["request_type"]), "return"))
	return fmt.Sprintf(
		"请确认是否提交%s申请，订单号 %s，原因：%s。",
		requestType,
		FirstNonEmpty(strings.TrimSpace(fmt.Sprint(slots["order_id"])), "未知"),
		FirstNonEmpty(strings.TrimSpace(fmt.Sprint(slots["reason"])), "未知"),
	)
}

func afterSaleTypeLabelFromSlots(requestType string) string {
	if strings.EqualFold(strings.TrimSpace(requestType), "exchange") {
		return "换货"
	}
	return "退货"
}
