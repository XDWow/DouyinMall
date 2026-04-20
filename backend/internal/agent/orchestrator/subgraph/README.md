# subgraph：当前生效的业务子图

这里现在只保留主图真正会路由到的业务子图。

主图入口在：
- [builder.go](/D:/workspace/go/DouyinMall/backend/internal/agent/orchestrator/graph/builder.go)

主图当前只会接入这些子图：
- `addtocart/`
- `aftersalesapply/`
- `aftersalespolicy/`
- `orderservice/`
- `productservice/`
- `promotionservice/`
- `unknown/`

公共辅助目录：
- `common/`
  放子图共用的小工具，例如 agent 输出解析、slots 文本渲染、引用转换。

## 子图分层

### 1. 确定性子图
- `addtocart/`
- `aftersalesapply/`

特点：
- 直接消费主图 `UnderstandingNode` 已提取好的 `session.Slots`
- 缺参走 interrupt/resume
- 参数齐全后直接构造 `tool_calls`
- 统一交给 `ToolsNode` 执行

### 2. agent 子图
- `orderservice/`
- `productservice/`
- `promotionservice/`
- `aftersalespolicy/`

特点：
- 读型、说明型、歧义型问题
- 由 `shared/subgraph_agent.go` 驱动模型多轮
- 模型在白名单能力内决定直接回答、调工具或 clarification
- `promotionservice`、`aftersalespolicy` 先 RAG，再进入 Agent

### 3. 兜底子图
- `unknown/`

特点：
- 只负责无法稳定判断时的兜底澄清
- 不调工具，不走 RAG

## 建议阅读顺序

如果你现在想快速看懂代码，按这个顺序看最省力：

1. `graph/builder.go`
2. `node/global/understanding/`
3. `node/shared/subgraph_agent.go`
4. `subgraph/addtocart/`
5. `subgraph/aftersalesapply/`
6. `subgraph/orderservice/`
7. `subgraph/productservice/`
8. `subgraph/promotionservice/`
9. `subgraph/aftersalespolicy/`
