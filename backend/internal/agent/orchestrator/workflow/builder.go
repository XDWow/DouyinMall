package workflow

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/intentclassify"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/retrieve"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/rewrite"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/workflow/inventory"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/workflow/knowledge"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/workflow/orderquery"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/workflow/productinfo"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/workflow/returnexchange"
)

type Builder struct {
	Model     model.ToolCallingChatModel
	Retriever einoretriever.Retriever
	Tools     *agenttool.Registry
	Prompts   *orchestratorprompt.Set
	Nodes     *orchestratornode.Suite
}

func (b *Builder) BuildIntentClassificationGraph(ctx context.Context) (compose.AnyGraph, error) {
	return intentclassify.Build(ctx, b.Model, b.Prompts, b.Nodes)
}

func (b *Builder) BuildRewriteGraph(ctx context.Context) (compose.AnyGraph, error) {
	return rewrite.Build(ctx, b.Model, b.Prompts, b.Nodes)
}

func (b *Builder) BuildRetrieveGraph(ctx context.Context) (compose.AnyGraph, error) {
	return retrieve.Build(ctx, b.Retriever, b.Nodes)
}

func (b *Builder) BuildToolExecGraph(ctx context.Context, mode agenttool.ToolExecutionMode) (compose.AnyGraph, error) {
	return toolexec.Build(ctx, b.Tools, b.Nodes, mode)
}

func (b *Builder) BuildOrderQueryWorkflow(ctx context.Context) (compose.AnyGraph, error) {
	return orderquery.Build(ctx, b.Tools, b.Nodes)
}

func (b *Builder) BuildReturnPolicyWorkflow(ctx context.Context) (compose.AnyGraph, error) {
	return knowledge.BuildReturnPolicy(ctx, b.Model, b.Retriever, b.Prompts, b.Nodes)
}

func (b *Builder) BuildInventoryWorkflow(ctx context.Context) (compose.AnyGraph, error) {
	return inventory.Build(ctx, b.Tools, b.Nodes)
}

func (b *Builder) BuildProductInfoWorkflow(ctx context.Context) (compose.AnyGraph, error) {
	return productinfo.Build(ctx, b.Model, b.Retriever, b.Tools, b.Prompts, b.Nodes)
}

func (b *Builder) BuildReturnExchangeWorkflow(ctx context.Context) (compose.AnyGraph, error) {
	return returnexchange.Build(ctx, b.Tools, b.Nodes)
}

func (b *Builder) BuildFallbackWorkflow(ctx context.Context) (compose.AnyGraph, error) {
	return knowledge.BuildFallback(ctx, b.Model, b.Retriever, b.Prompts, b.Nodes)
}
