package session

import (
	"strings"

	"github.com/cloudwego/eino/schema"
)

func VisibleTranscriptSchemaMessages(msgs []*schema.Message) []*schema.Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]*schema.Message, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		switch msg.Role {
		case schema.User:
			if strings.TrimSpace(msg.Content) != "" {
				out = append(out, msg)
			}
		case schema.Assistant:
			if strings.TrimSpace(msg.Content) != "" {
				out = append(out, msg)
			}
		default:
		}
	}
	return out
}
