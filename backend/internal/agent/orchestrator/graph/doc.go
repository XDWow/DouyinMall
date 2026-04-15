// Package graph 编译 Eino 主图：compose.Graph[map[string]any, *domain.State] + GenLocalState。
//
// 目录约定（与 Eino checkpoint/state 用法对齐）：
//
//   - builder*.go：节点注册与边、分支；Builder 注入各 Node 与子图 Build。
//   - state_handlers.go：主图线性段的 StatePre / StatePost（边上 I/O 为 *domain.State）。
//   - state_handlers.go 末尾：主图分支函数（BranchFromRoute 等）。
//   - subgraph_bridge.go：主图与子图之间的边界——Prepare*（*domain.State → 子图 GraphInput）
//     与 Apply*（子图 Output → 写回 *domain.State）；子图契约见各子包 io.go。
//
// 子图侧约定见 ../subgraph/README.md。

package graph
