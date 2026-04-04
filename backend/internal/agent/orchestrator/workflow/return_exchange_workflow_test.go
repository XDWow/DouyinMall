package workflow

import (
	"context"
	"testing"

	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
)

func TestBuildReturnExchangeWorkflowWithoutRegistry(t *testing.T) {
	builder := &Registry{
		Nodes: orchestratornode.NewSuite(orchestratornode.Dependencies{}),
	}

	graph, err := builder.BuildReturnExchangeWorkflow(context.Background())
	if err != nil {
		t.Fatalf("build workflow failed without registry: %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil workflow graph")
	}
}
