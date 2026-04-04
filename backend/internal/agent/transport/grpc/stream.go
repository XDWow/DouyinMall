package grpc

import (
	"context"
	"fmt"

	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	agentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1"
)

type StreamWriter struct {
	stream agentv1.AgentService_SendMessageStreamServer
}

func NewStreamWriter(stream agentv1.AgentService_SendMessageStreamServer) *StreamWriter {
	return &StreamWriter{stream: stream}
}

func (w *StreamWriter) Send(ctx context.Context, event orchestratorstate.StreamEvent) error {
	switch event.Event {
	case "node":
		payload, _ := event.Data.(map[string]any)
		if stringValue(payload["status"]) != "start" {
			return nil
		}
		stage := normalizeStage(stringValue(payload["node"]))
		if stage == "" {
			return nil
		}
		return w.stream.Send(&agentv1.ChatStreamChunk{
			Type:  agentv1.ChunkType_STAGE_UPDATE,
			Stage: stage,
		})
	case "token":
		payload, _ := event.Data.(map[string]any)
		text := stringValue(payload["text"])
		if text == "" {
			return nil
		}
		return w.stream.Send(&agentv1.ChatStreamChunk{
			Type: agentv1.ChunkType_TEXT_DELTA,
			Text: text,
		})
	default:
		return nil
	}
}

func normalizeStage(node string) string {
	switch node {
	case "L0ExactCacheNode":
		return "cache"
	case "IntentClassifyChain", "BuildIntentPromptInputNode", "IntentPromptNode", "IntentModelNode", "ApplyIntentNode":
		return "intent"
	case "QueryRewriteNode", "RewriteEvaluateNode", "RewriteIdentityNode", "BuildRewritePromptInputNode", "RewritePromptNode", "RewriteModelNode", "ApplyRewriteNode":
		return "rewrite"
	case "KnowledgeRetrieverNode", "PrepareRetrieveQueryNode", "RetrieverNode", "ApplyRetrieveNode", "OptionalRerankNode":
		return "retrieval"
	case "CallOrderServiceNode", "CallInventoryServiceNode", "CallProductServiceNode", "CallReturnOrderServiceNode", "CallAfterSaleServiceNode", "PrepareSerialToolMessageNode", "PrepareParallelReadonlyToolMessageNode", "ToolsNode", "ApplyToolMessagesNode":
		return "tool"
	case "ResponseRenderNode":
		return "generating"
	case "ConfirmSummaryNode":
		return "handoff"
	default:
		return ""
	}
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}
