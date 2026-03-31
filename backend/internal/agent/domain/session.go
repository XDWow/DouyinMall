package domain

import "time"

// 会话聚合根，一个用户同一时间只有一个活跃会话
type Session struct {
	ID                 string
	UserID             int64
	Channel            string // web / app / miniprogram
	Status             SessionStatus
	LowConfidenceTurns int          // 连续低置信度轮数（3 轮自动转人工；angry/urgent 情绪时 2 轮即触发）
	ConvFlow           EntityMemory `json:"conv_flow,omitempty"` // 对话上下文（工具调用实体引用，与 session 共生命周期）
	Messages           []Message    `json:"-"`                   // 滑动窗口内的消息，不随元信息 JSON 序列化，由 :msgs Redis list 单独管理
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Clone 深拷贝 Session（含 Messages 切片），用于传递给异步 goroutine 避免数据竞争
func (s *Session) Clone() *Session {
	cp := *s
	cp.Messages = make([]Message, len(s.Messages))
	copy(cp.Messages, s.Messages)
	return &cp
}

// 返回最近 n 条消息（滑动窗口）
func (s *Session) RecentMessages(n int) []Message {
	if len(s.Messages) <= n {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-n:]
}

// 返回最后一条消息的预览（截取前 50 字符）
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
	SessionHuman // 已转人工
)

// 消息值对象
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

// 用户意图
type IntentType int32

const (
	IntentUnknown         IntentType = iota
	IntentFAQ                        // 常见问题
	IntentProductInquiry             // 商品咨询
	IntentOrderInquiry               // 订单查询
	IntentLogistics                  // 物流查询
	IntentPayment                    // 支付问题
	IntentReturn                     // 退货退款
	IntentComplaint                  // 投诉建议
	IntentPromotion                  // 营销活动
	IntentChitchat                   // 闲聊
	IntentTransferToHuman            // 转人工
)

// 返回意图名称（用于 Prometheus label、日志等）
func (i IntentType) String() string {
	switch i {
	case IntentFAQ:
		return "faq"
	case IntentProductInquiry:
		return "product_inquiry"
	case IntentOrderInquiry:
		return "order_inquiry"
	case IntentLogistics:
		return "logistics"
	case IntentPayment:
		return "payment"
	case IntentReturn:
		return "return"
	case IntentComplaint:
		return "complaint"
	case IntentPromotion:
		return "promotion"
	case IntentChitchat:
		return "chitchat"
	case IntentTransferToHuman:
		return "transfer_to_human"
	default:
		return "unknown"
	}
}
