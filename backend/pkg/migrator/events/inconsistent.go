package events

// ID定位数据，Direction 说明谁为基准，type 确定操作类型，
// 1.传入需要更新的内容，消费者拿着内容直接更新target，问题是这个内容已经旧了
// 2.传入id,消费者再去base中查数据，查到的数据更新target,这样更能保证一致性
type InconsistentEvent struct {
	ID int64
	// 以谁为基准
	Direction string

	// 有些时候，一些观测，或者一些第三方，需要知道，是什么引起的不一致
	// 因为他要去 DEBUG
	// 这个是可选的
	Type string
	// 事件里面带 base 的数据
	// 修复数据用这里的去修，这种做法是不行的，因为有严重的并发问题
	Columns map[string]any
}

const (
	// InconsistentEventTypeTargetMissing 校验的目标数据，缺了这一条
	InconsistentEventTypeTargetMissing = "target_missing"
	// InconsistentEventTypeNEQ 不相等
	InconsistentEventTypeNEQ         = "neq"
	InconsistentEventTypeBaseMissing = "base_missing"
)
