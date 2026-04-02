package workflow

import (
	"context"
	"testing"

	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/graph/node"
)

func TestBuildReturnExchangeApplyWorkflowWithoutRegistry(t *testing.T) {
	builder := &Builder{
		Nodes: orchestratornode.NewSuite(orchestratornode.Dependencies{}),
	}

	graph, err := builder.BuildReturnExchangeApplyWorkflow(context.Background())
	if err != nil {
		t.Fatalf("build workflow failed without registry: %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil workflow graph")
	}
}
