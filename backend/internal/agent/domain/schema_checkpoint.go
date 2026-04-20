package domain

import "github.com/cloudwego/eino/schema"

func init() {
	schema.RegisterName[AddToCartInterruptState]("agent_addtocart_interrupt_persist_v1")
	schema.RegisterName[AftersalesApplyInterruptState]("agent_aftersales_apply_interrupt_persist_v1")
	schema.RegisterName[ResumeData]("agent_resume_data_v1")
}
