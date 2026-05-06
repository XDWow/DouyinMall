package readflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type SlotsBuilder func(*domain.State) string

type FallbackBuilder func([]*schema.Document) string

func StaticFallback(reply string) FallbackBuilder {
	return func(_ []*schema.Document) string {
		return reply
	}
}

type Config struct {
	Intent domain.Intent

	AgentNodeName       string
	CacheLookupNodeName string
	CacheResultNodeName string
	RAGNodeName         string

	ChatModel model.ToolCallingChatModel
	Tools     *agenttool.Registry
	Skills    *agentskill.Registry

	ToolNames  []string
	SkillNames []string
	SystemHint string
	MaxTokens  int

	CacheLookup *globalnode.CacheLookupNode

	RAG        *ragnode.RAGNode
	RAGDomains []string
	RequireRAG bool

	BuildSlotsContext SlotsBuilder
	FallbackReply     FallbackBuilder
	IncludeReferences bool
}

type agentInput struct {
	UserMessage     string
	RewrittenQuery  string
	History         []*schema.Message
	SlotsContext    string
	RetrievalResult *ragnode.Result
}

func Build(ctx context.Context, cfg Config) (compose.AnyGraph, error) {
	cfg = cfg.withDefaults()
	if cfg.ChatModel == nil || (cfg.RequireRAG && cfg.RAG == nil) {
		return nil, nil
	}

	agent := sharednode.NewSubgraphAgent(cfg.ChatModel, cfg.Tools, cfg.Skills, cfg.MaxTokens)
	if cfg.RAG != nil {
		return buildRAGFlow(ctx, cfg, agent)
	}
	return buildAgentOnlyFlow(ctx, cfg, agent)
}

func (cfg Config) withDefaults() Config {
	if cfg.AgentNodeName == "" {
		cfg.AgentNodeName = "ReadAgentNode"
	}
	if cfg.CacheLookupNodeName == "" {
		cfg.CacheLookupNodeName = "ReadCacheLookupNode"
	}
	if cfg.CacheResultNodeName == "" {
		cfg.CacheResultNodeName = "ReadCacheResultNode"
	}
	if cfg.RAGNodeName == "" {
		cfg.RAGNodeName = "ReadRAGNode"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 512
	}
	cfg.ToolNames = append([]string(nil), cfg.ToolNames...)
	cfg.SkillNames = append([]string(nil), cfg.SkillNames...)
	cfg.RAGDomains = append([]string(nil), cfg.RAGDomains...)
	return cfg
}

func buildAgentOnlyFlow(_ context.Context, cfg Config, agent *sharednode.SubgraphAgent) (compose.AnyGraph, error) {
	wf := compose.NewWorkflow[struct{}, *domain.ChatResult](compose.WithGenLocalState(domain.SharedGraphState))
	wf.AddLambdaNode(cfg.AgentNodeName,
		compose.InvokableLambda(func(ctx context.Context, in agentInput) (*domain.ChatResult, error) {
			if cached, err := subgraphcommon.LookupReadCache(ctx, cfg.CacheLookup, cfg.Intent); cached != nil || err != nil {
				return cached, err
			}
			return runAgent(ctx, cfg, agent, in)
		}),
		compose.WithStatePreHandler(func(_ context.Context, in agentInput, st *domain.State) (agentInput, error) {
			return fillAgentInput(in, st, cfg.BuildSlotsContext)
		}),
	).AddDependency(compose.START)

	wf.End().AddInput(cfg.AgentNodeName)
	return wf, nil
}

func buildRAGFlow(_ context.Context, cfg Config, agent *sharednode.SubgraphAgent) (compose.AnyGraph, error) {
	wf := compose.NewWorkflow[struct{}, *domain.ChatResult](compose.WithGenLocalState(domain.SharedGraphState))

	wf.AddLambdaNode(cfg.CacheLookupNodeName, compose.InvokableLambda(
		func(ctx context.Context, _ struct{}) (*domain.ChatResult, error) {
			return subgraphcommon.LookupReadCache(ctx, cfg.CacheLookup, cfg.Intent)
		},
	)).AddDependency(compose.START)

	wf.AddLambdaNode(cfg.CacheResultNodeName, compose.InvokableLambda(
		func(_ context.Context, in *domain.ChatResult) (*domain.ChatResult, error) {
			return in, nil
		},
	)).AddInput(cfg.CacheLookupNodeName)

	wf.AddLambdaNode(cfg.RAGNodeName,
		compose.InvokableLambda(cfg.RAG.Invoke),
		compose.WithStatePreHandler(func(_ context.Context, in ragnode.Input, st *domain.State) (ragnode.Input, error) {
			if st == nil || st.Input == nil || st.Session == nil {
				return in, fmt.Errorf("state input/session is required")
			}
			return ragnode.Input{
				Message: support.FirstNonEmpty(strings.TrimSpace(st.RewrittenQuery), strings.TrimSpace(st.Input.Message)),
				History: subgraphcommon.HistoryMessages(st.Session.RecentMessages),
				Intent:  string(cfg.Intent),
				Domains: append([]string(nil), cfg.RAGDomains...),
			}, nil
		}),
	).AddInputWithOptions(compose.START, nil, compose.WithNoDirectDependency())

	wf.AddLambdaNode(cfg.AgentNodeName,
		compose.InvokableLambda(func(ctx context.Context, in agentInput) (*domain.ChatResult, error) {
			return runAgent(ctx, cfg, agent, in)
		}),
		compose.WithStatePreHandler(func(_ context.Context, in agentInput, st *domain.State) (agentInput, error) {
			return fillAgentInput(in, st, cfg.BuildSlotsContext)
		}),
	).AddInput(cfg.RAGNodeName, compose.ToField("RetrievalResult"))

	wf.AddBranch(cfg.CacheLookupNodeName, compose.NewGraphBranch(
		func(_ context.Context, in *domain.ChatResult) (string, error) {
			if in != nil {
				return cfg.CacheResultNodeName, nil
			}
			return cfg.RAGNodeName, nil
		},
		map[string]bool{
			cfg.CacheResultNodeName: true,
			cfg.RAGNodeName:         true,
		},
	))

	wf.End().
		AddInput(cfg.AgentNodeName).
		AddInput(cfg.CacheResultNodeName)
	return wf, nil
}

func fillAgentInput(in agentInput, st *domain.State, slotsBuilder SlotsBuilder) (agentInput, error) {
	if st == nil || st.Input == nil || st.Session == nil {
		return in, fmt.Errorf("state input/session is required")
	}
	in.UserMessage = strings.TrimSpace(st.Input.Message)
	in.RewrittenQuery = strings.TrimSpace(st.RewrittenQuery)
	in.History = subgraphcommon.HistoryMessages(st.Session.RecentMessages)
	if slotsBuilder != nil {
		in.SlotsContext = strings.TrimSpace(slotsBuilder(st))
	}
	return in, nil
}

func runAgent(ctx context.Context, cfg Config, agent *sharednode.SubgraphAgent, in agentInput) (*domain.ChatResult, error) {
	docs := documentsOf(in.RetrievalResult)
	agentIn := sharednode.SubgraphAgentInput{
		ToolNames:    cfg.ToolNames,
		SkillNames:   cfg.SkillNames,
		SlotsContext: in.SlotsContext,
		UserQuery:    support.FirstNonEmpty(in.RewrittenQuery, in.UserMessage),
		History:      in.History,
		SystemHint:   cfg.SystemHint,
	}
	if cfg.RAG != nil {
		agentIn.DocumentsText = support.DocumentsText(docs)
	}

	finalText, _, err := agent.Run(ctx, agentIn)
	if err != nil {
		return nil, err
	}

	decision := subgraphcommon.ParseAgentDecision(finalText)
	if decision.Type == "clarification" {
		return nil, subgraphcommon.InterruptForDecision(ctx, decision)
	}

	result := &domain.ChatResult{
		Intent:        cfg.Intent,
		Reply:         support.FirstNonEmpty(decision.Reply, fallbackReply(cfg, docs)),
		NeedHandoff:   decision.NeedHandoff,
		HandoffReason: decision.HandoffReason,
	}
	if cfg.IncludeReferences {
		result.References = support.DocumentsToReferences(docs)
	}
	return result, nil
}

func fallbackReply(cfg Config, docs []*schema.Document) string {
	if cfg.FallbackReply != nil {
		return strings.TrimSpace(cfg.FallbackReply(docs))
	}
	return "Please provide more detail."
}

func documentsOf(result *ragnode.Result) []*schema.Document {
	if result == nil {
		return nil
	}
	return result.Documents
}
