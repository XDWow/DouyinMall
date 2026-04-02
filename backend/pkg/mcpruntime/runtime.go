package mcpruntime

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

const (
	HeaderUserID    = "X-MCP-User-ID"
	HeaderSessionID = "X-MCP-Session-ID"
	HeaderTraceID   = "X-MCP-Trace-ID"
)

type Runtime struct {
	UserID    int64
	SessionID string
	TraceID   string
}

type runtimeKey struct{}

func WithContext(ctx context.Context, runtime Runtime) context.Context {
	return context.WithValue(ctx, runtimeKey{}, runtime)
}

func FromContext(ctx context.Context) Runtime {
	runtime, _ := ctx.Value(runtimeKey{}).(Runtime)
	return runtime
}

func Headers(runtime Runtime) map[string]string {
	headers := make(map[string]string, 3)
	if runtime.UserID > 0 {
		headers[HeaderUserID] = strconv.FormatInt(runtime.UserID, 10)
	}
	if strings.TrimSpace(runtime.SessionID) != "" {
		headers[HeaderSessionID] = runtime.SessionID
	}
	if strings.TrimSpace(runtime.TraceID) != "" {
		headers[HeaderTraceID] = runtime.TraceID
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

func FromHeaders(header http.Header) Runtime {
	if header == nil {
		return Runtime{}
	}
	userID, _ := strconv.ParseInt(strings.TrimSpace(header.Get(HeaderUserID)), 10, 64)
	return Runtime{
		UserID:    userID,
		SessionID: strings.TrimSpace(header.Get(HeaderSessionID)),
		TraceID:   strings.TrimSpace(header.Get(HeaderTraceID)),
	}
}

func WithHTTPContext(ctx context.Context, req *http.Request) context.Context {
	if req == nil {
		return ctx
	}
	return WithContext(ctx, FromHeaders(req.Header))
}
