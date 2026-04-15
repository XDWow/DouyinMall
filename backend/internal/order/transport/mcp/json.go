package mcp

import "encoding/json"

func marshalListUserOrdersPayload(p listUserOrdersPayload) string {
	data, _ := json.Marshal(p)
	return string(data)
}

func marshalGetOrderPayload(p getOrderPayload) string {
	data, _ := json.Marshal(p)
	return string(data)
}
