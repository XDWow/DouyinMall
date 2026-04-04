//go:build legacy_agent

package domain

import "time"

// Session 娴兼俺鐦介懕姘値閺?
// 娑撯偓娑擃亞鏁ら幋宄版倱娑撯偓閺冨爼妫块崣顏呮箒娑撯偓娑擃亝妞跨捄鍐х窗鐠?
type Session struct {
	ID         string
	UserID     int64
	Channel    string // web / app / miniprogram
	Status     SessionStatus
	Summary    string // 閺冣晜婀￠崢鍡楀蕉閸樺缂夐幗妯款洣閿涘牊绮﹂崝銊х崶閸欙絾绐愬Ч鏉挎倵閻ㄥ嫬鍞寸€圭櫢绱?	TotalTurns int
	Messages   []Message // 濠婃垵濮╃粣妤€褰涢崘鍛畱濞戝牊浼呴敍鍫熸付鏉?N 閺夆槄绱?	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// RecentMessages 鏉╂柨娲栭張鈧潻?n 閺夆剝绉烽幁顖ょ礄濠婃垵濮╃粣妤€褰涢敍?
func (s *Session) RecentMessages(n int) []Message {
	if len(s.Messages) <= n {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-n:]
}

// LastMessagePreview 鏉╂柨娲栭張鈧崥搴濈閺夆剝绉烽幁顖滄畱妫板嫯顫嶉敍鍫熷焻閸欐牕澧?50 鐎涙顑侀敍?
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
	SessionHuman // 瀹歌尪娴嗘禍鍝勪紣
)

// Message 濞戝牊浼呴崐鐓庮嚠鐠?
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

// IntentType 閻劍鍩涢幇蹇撴禈
type IntentType int32

const (
	IntentUnknown         IntentType = iota
	IntentFAQ                        // 鐢瓕顫嗛梻顕€顣?	IntentProductInquiry             // 閸熷棗鎼ч崪銊嚄
	IntentOrderInquiry               // 鐠併垹宕熼弻銉嚄
	IntentLogistics                  // 閻椻晜绁﹂弻銉嚄
	IntentPayment                    // 閺€顖欑帛闂傤噣顣?	IntentReturn                     // 闁偓鐠愌団偓鈧▎?
	IntentComplaint                  // 閹舵洝鐦斿楦款唴
	IntentPromotion                  // 閽€銉╂敘濞茶濮?	IntentChitchat                   // 闂傝尪浜?	IntentTransferToHuman            // 鏉烆兛姹夊?
)


