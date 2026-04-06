package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// SubmitAfterSaleInput 描述售后提交结果整理节点的输入。
type SubmitAfterSaleInput struct {
	ConfirmStatus string
	RequestType   string
	SubmitResult  map[string]any
}

// SubmitAfterSaleResult 描述售后提交后的结构化结果。
type SubmitAfterSaleResult struct {
	FinalAnswer     string
	NeedHandoff     bool
	HandoffReason   string
	ReadOnly        bool
	AwaitingConfirm bool
	ConfirmStatus   string
}

// SubmitAfterSaleNode 负责根据工具结果整理售后提交结果。
type SubmitAfterSaleNode struct{}

func NewSubmitAfterSaleNode() *SubmitAfterSaleNode {
	return &SubmitAfterSaleNode{}
}

// Invoke 完成售后提交结果整理。
func (n *SubmitAfterSaleNode) Invoke(_ context.Context, input SubmitAfterSaleInput) (*SubmitAfterSaleResult, error) {
	if strings.EqualFold(strings.TrimSpace(input.ConfirmStatus), "cancelled") {
		return &SubmitAfterSaleResult{
			FinalAnswer:     "已取消本次售后申请。",
			AwaitingConfirm: false,
			ReadOnly:        true,
		}, nil
	}

	result := input.SubmitResult
	if len(result) == 0 {
		return &SubmitAfterSaleResult{
			NeedHandoff:   true,
			HandoffReason: "after_sale_submit_failed",
			FinalAnswer:   "售后申请提交失败，已为你转人工处理。",
		}, nil
	}

	requestNo := strings.TrimSpace(fmt.Sprint(result["request_no"]))
	status := strings.TrimSpace(fmt.Sprint(result["status"]))
	requestType := strings.TrimSpace(fmt.Sprint(result["request_type"]))
	if requestType == "" {
		requestType = support.FirstNonEmpty(input.RequestType, "return")
	}

	return &SubmitAfterSaleResult{
		AwaitingConfirm: false,
		ReadOnly:        true,
		FinalAnswer: fmt.Sprintf(
			"%s申请已提交成功，申请单号：%s，当前状态：%s。",
			support.AfterSaleTypeLabel(requestType),
			support.FirstNonEmpty(requestNo, "pending"),
			support.FirstNonEmpty(status, "pending_review"),
		),
	}, nil
}
