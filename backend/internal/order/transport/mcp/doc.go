// Package mcp 实现订单域的 MCP（Streamable HTTP）适配层：只做协议与 DTO 转换，业务一律进入 usecase。
//
// 工具：get_order（单笔，须与运行时 user 一致）、list_user_orders（当前用户列表第一页，固定 10 条）。
// 配置键 query_order 与 list_user_orders 等价（列表）；若仅配置 query_order，服务端会自动补上 get_order，保证两个 tool 均暴露。
package mcp
