package domain

// Kafka 消息持久化事件
// 每轮对话产生一个事件，包含本轮新增的用户消息和助手回复（通常 2 条）
// 消费端按批次聚合后执行批量 INSERT，显著降低 MySQL 写入频次
type ChatMessageEvent struct {
	SessionID string    `json:"session_id"`
	Messages  []Message `json:"messages"`
}
