//go:build legacy_agent

package domain

import "time"

// Session 浼氳瘽鑱氬悎鏍?
// 涓€涓敤鎴峰悓涓€鏃堕棿鍙湁涓€涓椿璺冧細璇?
type Session struct {
	ID         string
	UserID     int64
	Channel    string // web / app / miniprogram
	Status     SessionStatus
	Summary    string // 鏃╂湡鍘嗗彶鍘嬬缉鎽樿锛堟粦鍔ㄧ獥鍙ｆ窐姹板悗鐨勫唴瀹癸級
	TotalTurns int
	Messages   []Message // 婊戝姩绐楀彛鍐呯殑娑堟伅锛堟渶杩?N 鏉★級
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// RecentMessages 杩斿洖鏈€杩?n 鏉℃秷鎭紙婊戝姩绐楀彛锛?
func (s *Session) RecentMessages(n int) []Message {
	if len(s.Messages) <= n {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-n:]
}

// LastMessagePreview 杩斿洖鏈€鍚庝竴鏉℃秷鎭殑棰勮锛堟埅鍙栧墠 50 瀛楃锛?
func (s *Session) LastMessagePreview() string {
	if len(s.Messages) == 0 {
		return ""
	}
	last := s.Messages[len(s.Messages)-1].Content
	if len([]rune(last)) > 50 {
		return string([]rune(last)[:50]) + "..."
	}
	return last
}

type SessionStatus uint8

const (
	SessionActive SessionStatus = iota + 1
	SessionClosed
	SessionHuman // 宸茶浆浜哄伐
)

// Message 娑堟伅鍊煎璞?
type Message struct {
	ID         string
	SessionID  string
	Role       Role
	Content    string
	Intent     IntentType
	Confidence float32
	TokensUsed int
	LatencyMs  int64
	CreatedAt  time.Time
}

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// IntentType 鐢ㄦ埛鎰忓浘
type IntentType int32

const (
	IntentUnknown         IntentType = iota
	IntentFAQ                        // 甯歌闂
	IntentProductInquiry             // 鍟嗗搧鍜ㄨ
	IntentOrderInquiry               // 璁㈠崟鏌ヨ
	IntentLogistics                  // 鐗╂祦鏌ヨ
	IntentPayment                    // 鏀粯闂
	IntentReturn                     // 閫€璐ч€€娆?
	IntentComplaint                  // 鎶曡瘔寤鸿
	IntentPromotion                  // 钀ラ攢娲诲姩
	IntentChitchat                   // 闂茶亰
	IntentTransferToHuman            // 杞汉宸?
)
