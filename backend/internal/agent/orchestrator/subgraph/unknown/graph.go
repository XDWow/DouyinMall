package unknown

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type Input struct {
	UserMessage string
}

func InputFromState(st *domain.State) (Input, error) {
	if st == nil || st.Input == nil {
		return Input{}, fmt.Errorf("state input is required")
	}
	return Input{UserMessage: strings.TrimSpace(st.Input.Message)}, nil
}

func Build(_ context.Context) (compose.AnyGraph, error) {
	wf := compose.NewWorkflow[struct{}, *domain.ChatResult](compose.WithGenLocalState(domain.SharedGraphState))
	wf.AddLambdaNode("UnknownNode",
		compose.InvokableLambda(replyUnknown),
		compose.WithStatePreHandler(func(_ context.Context, in Input, st *domain.State) (Input, error) {
			return InputFromState(st)
		}),
	).AddDependency(compose.START)
	wf.End().AddInput("UnknownNode")
	return wf, nil
}

func replyUnknown(_ context.Context, in Input) (*domain.ChatResult, error) {
	reply := "我还不确定你想让我处理哪个问题域。你可以直接告诉我：商品服务、订单服务、优惠活动、售后政策、发起售后申请，或者加入购物车。"
	if strings.TrimSpace(in.UserMessage) == "" {
		reply = "请告诉我你的问题。"
	}
	return &domain.ChatResult{
		Intent: domain.IntentUnknown,
		Reply:  reply,
	}, nil
}
