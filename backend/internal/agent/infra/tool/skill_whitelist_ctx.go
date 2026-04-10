package tool

import (
	"context"
	"strings"
)

type skillWhitelistCtxKey struct{}

// WithSkillWhitelist 将本轮业务允许的 skill 名放入 ctx，供 fetch_skill 校验。
func WithSkillWhitelist(ctx context.Context, names []string) context.Context {
	if ctx == nil || len(names) == 0 {
		return ctx
	}
	cp := append([]string(nil), names...)
	return context.WithValue(ctx, skillWhitelistCtxKey{}, cp)
}

func skillWhitelistFromContext(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(skillWhitelistCtxKey{}).([]string)
	return v
}

func skillNameAllowed(ctx context.Context, name string) bool {
	name = trimSkillName(name)
	if name == "" {
		return false
	}
	for _, n := range skillWhitelistFromContext(ctx) {
		if strings.TrimSpace(n) == name {
			return true
		}
	}
	return false
}

func trimSkillName(s string) string {
	return strings.TrimSpace(s)
}
