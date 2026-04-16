package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type ChatUseCase struct {
	runtime runtime
}

func NewChatUseCase(runtime runtime) *ChatUseCase {
	return &ChatUseCase{runtime: runtime}
}

func (uc *ChatUseCase) Execute(ctx context.Context, input ChatInput) (*ChatOutput, error) {
	command, err := validateChatInput(input)
	if err != nil {
		return nil, err
	}
	resp, err := uc.runtime.Chat(ctx, *command)
	if err != nil {
		return nil, err
	}
	return toChatOutput(resp), nil
}

func (uc *ChatUseCase) Stream(ctx context.Context, input ChatInput, writer domain.StreamWriter) (*ChatOutput, error) {
	command, err := validateChatInput(input)
	if err != nil {
		return nil, err
	}
	resp, err := uc.runtime.ChatStream(ctx, *command, writer)
	if err != nil {
		return nil, err
	}
	return toChatOutput(resp), nil
}

func validateChatInput(input ChatInput) (*domain.ChatCommand, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	resumeToken := strings.TrimSpace(input.ResumeToken)
	interruptID := strings.TrimSpace(input.InterruptID)
	if interruptID != "" && resumeToken == "" {
		return nil, fmt.Errorf("interrupt_id requires resume_token (checkpoint id)")
	}
	var resumeData map[string]any
	if j := strings.TrimSpace(input.ResumeDataJSON); j != "" {
		if err := json.Unmarshal([]byte(j), &resumeData); err != nil {
			return nil, fmt.Errorf("resume_data_json: %w", err)
		}
		if resumeData == nil {
			resumeData = map[string]any{}
		}
	}
	command := &domain.ChatCommand{
		SessionID:   strings.TrimSpace(input.SessionID),
		UserID:      input.UserID,
		Message:     message,
		ResumeToken: resumeToken,
		InterruptID: interruptID,
		ResumeData:  resumeData,
		Metadata:    copyStringMap(input.Metadata),
	}
	return command, nil
}

func toChatOutput(resp *domain.ChatResult) *ChatOutput {
	if resp == nil {
		return &ChatOutput{}
	}
	references := make([]KnowledgeRef, 0, len(resp.References))
	for _, item := range resp.References {
		references = append(references, KnowledgeRef{
			ID:       item.ID,
			Title:    item.Title,
			Snippet:  item.Snippet,
			Category: item.Category,
			Score:    item.Score,
			Metadata: copyStringMap(item.Metadata),
		})
	}
	toolExecutions := make([]ToolExecution, 0, len(resp.ToolExecutions))
	for _, item := range resp.ToolExecutions {
		toolExecutions = append(toolExecutions, ToolExecution{
			Name:       item.Name,
			Arguments:  copyAnyMap(item.Arguments),
			Reason:     item.Reason,
			Success:    item.Success,
			Result:     item.Result,
			Error:      item.Error,
			LatencyMs:  item.LatencyMs,
			OccurredAt: item.OccurredAt,
			Metadata:   copyStringMap(item.Metadata),
		})
	}
	traceSteps := make([]TraceStep, 0, len(resp.Trace.Steps))
	for _, step := range resp.Trace.Steps {
		traceSteps = append(traceSteps, TraceStep{
			Node:      step.Node,
			Status:    step.Status,
			LatencyMs: step.LatencyMs,
			Detail:    step.Detail,
		})
	}
	out := &ChatOutput{
		SessionID:      resp.SessionID,
		TraceID:        resp.TraceID,
		Status:         resp.Status,
		Reply:          resp.Reply,
		Intent:         resp.Intent,
		Confidence:     resp.Confidence,
		NeedHandoff:    resp.NeedHandoff,
		HandoffReason:  resp.HandoffReason,
		References:     references,
		UsedToolNames:  append([]string(nil), resp.UsedToolNames...),
		ToolExecutions: toolExecutions,
		Trace: Trace{
			TraceID:              resp.Trace.TraceID,
			CheckpointID:         resp.Trace.CheckpointID,
			CacheHit:             resp.Trace.CacheHit,
			RewrittenQuery:       resp.Trace.RewrittenQuery,
			Steps:                traceSteps,
			SlowestStepNode:      resp.Trace.SlowestStepNode,
			SlowestStepLatencyMs: resp.Trace.SlowestStepLatencyMs,
		},
	}
	if resp.Interrupt != nil {
		out.Interrupt = &InterruptInfo{
			CheckpointID: resp.Interrupt.CheckpointID,
			InterruptID:  resp.Interrupt.InterruptID,
			RerunNodes:   append([]string(nil), resp.Interrupt.RerunNodes...),
		}
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
