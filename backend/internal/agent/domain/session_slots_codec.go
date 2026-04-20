package domain

import (
	"encoding/json"
)

const usrCtxKey = "_usr_ctx"

// PackUserSessionIntoSlots 把用户会话字段写入槽位 map（落 slots_json）；与工具槽位共存。
func PackUserSessionIntoSlots(slots map[string]any, user Session) map[string]any {
	out := CloneAnyMap(slots)
	if out == nil {
		out = make(map[string]any, 1)
	}
	if b, err := json.Marshal(user); err == nil && len(b) > 2 && string(b) != "{}" {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err == nil && len(m) > 0 {
			out[usrCtxKey] = m
		}
	}
	return out
}

// UnpackUserSessionFromSlots 从持久化槽位 map 拆出用户 Session，返回剩余工具态 map。
func UnpackUserSessionFromSlots(slots map[string]any) (Session, map[string]any) {
	if len(slots) == 0 {
		return Session{}, nil
	}
	rest := CloneAnyMap(slots)
	raw, ok := rest[usrCtxKey]
	delete(rest, usrCtxKey)
	var user Session
	if ok && raw != nil {
		b, err := json.Marshal(raw)
		if err == nil {
			_ = json.Unmarshal(b, &user)
		}
	}
	if len(rest) == 0 {
		rest = nil
	}
	return user, rest
}
