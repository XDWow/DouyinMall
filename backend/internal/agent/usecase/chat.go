package usecase

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
)

type ChatUseCase struct {
	runner Runner
}

func NewChatUseCase(runner Runner) *ChatUseCase {
	return &ChatUseCase{runner: runner}
}

func (uc *ChatUseCase) Execute(ctx context.Context, req dto.ChatRequest) (*dto.ChatResponse, error) {
	return uc.runner.Chat(ctx, req)
}

func (uc *ChatUseCase) Stream(ctx context.Context, req dto.ChatRequest, writer graphstate.StreamWriter) (*dto.ChatResponse, error) {
	return uc.runner.ChatStream(ctx, req, writer)
}
