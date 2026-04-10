package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
)

type stubStreamServer struct {
	items []*agentv1.ChatStreamChunk
}

func (s *stubStreamServer) Context() context.Context { return context.Background() }
func (s *stubStreamServer) SendMsg(any) error        { return nil }
func (s *stubStreamServer) RecvMsg(any) error        { return nil }
func (s *stubStreamServer) SetHeader(metadata.MD) error {
	return nil
}
func (s *stubStreamServer) SendHeader(metadata.MD) error {
	return nil
}
func (s *stubStreamServer) SetTrailer(metadata.MD)       {}
func (s *stubStreamServer) Header() (metadata.MD, error) { return nil, nil }
func (s *stubStreamServer) Trailer() metadata.MD         { return nil }
func (s *stubStreamServer) Close() error                 { return nil }
func (s *stubStreamServer) Send(item *agentv1.ChatStreamChunk) error {
	s.items = append(s.items, item)
	return nil
}

func TestStreamWriterSend(t *testing.T) {
	stream := &stubStreamServer{}
	writer := NewStreamWriter(stream)

	err := writer.Send(context.Background(), domain.StreamEvent{
		Event: "node",
		Data: map[string]any{
			"node":   "FinalizeNode",
			"status": "start",
		},
	})
	require.NoError(t, err)

	err = writer.Send(context.Background(), domain.StreamEvent{
		Event: "token",
		Data:  map[string]any{"text": "hello"},
	})
	require.NoError(t, err)

	require.Len(t, stream.items, 2)
	require.Equal(t, agentv1.ChunkType_STAGE_UPDATE, stream.items[0].GetType())
	require.Equal(t, "generating", stream.items[0].GetStage())
	require.Equal(t, agentv1.ChunkType_TEXT_DELTA, stream.items[1].GetType())
	require.Equal(t, "hello", stream.items[1].GetText())
}

