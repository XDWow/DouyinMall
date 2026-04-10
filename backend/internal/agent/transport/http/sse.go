package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type SSEWriter struct {
	writer  http.ResponseWriter
	flusher http.Flusher
}

func NewSSEWriter(writer http.ResponseWriter, flusher http.Flusher) *SSEWriter {
	return &SSEWriter{
		writer:  writer,
		flusher: flusher,
	}
}

func (w *SSEWriter) Send(ctx context.Context, event domain.StreamEvent) error {
	payload, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w.writer, "event: %s\ndata: %s\n\n", event.Event, payload); err != nil {
		return err
	}
	w.flusher.Flush()
	return nil
}

