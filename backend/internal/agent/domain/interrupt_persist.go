package domain

// SlotClarifyPersisted 缺参追问型 StatefulInterrupt 在 checkpoint 中保存的快照。
type SlotClarifyPersisted struct {
	Intent   Intent
	Subgraph string
	Slots    map[string]any
	Entities map[string]string
}

// ReturnExchangeSubmitPersisted 售后提交确认型中断保存的槽位快照。
type ReturnExchangeSubmitPersisted struct {
	Slots map[string]any
}
