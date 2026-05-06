# Subgraph Layout

当前客服主图只负责通用编排：

`AccessGuard -> SessionLoad -> Understanding -> Route -> Subgraph -> Finalize`

真正的业务行为都收敛在子图里。子图分两类：

## Read Flows

- `productservice/`
- `orderservice/`
- `promotionservice/`
- `aftersalespolicy/`

读型子图统一使用 `readflow.Build`：

1. 在子图内部先查读缓存。
2. 命中缓存时直接返回 `ChatResult`，不再走 RAG、工具或 Agent。
3. 未命中时继续执行子图自己的能力：商品和订单走 ADK Agent，优惠和售后政策先 RAG 再交给 ADK Agent。
4. 每个子图只声明 intent、node 名、工具白名单、skill 白名单、RAG 域、prompt 和 slots 上下文。

缓存不放在主图做全局判断。原因是“是否能读缓存”属于读型业务能力，而写型子图如加购、售后申请不能被缓存短路。

## Write Flows

- `addtocart/`
- `aftersalesapply/`

写型子图保持确定性编排：

1. 消费 `UnderstandingNode` 已提取的 slots。
2. 缺参数时用 interrupt/resume 追问。
3. 参数齐全后由业务代码构造受控 `tool_calls`。
4. 最终交给统一工具执行链路，不让模型自由提交写操作。

## Fallback

- `unknown/`

兜底子图只负责友好收口，不调用工具，不走 RAG。

## Shared Packages

- `common/`：agent JSON 解析、slots 渲染、interrupt、引用解析等小工具。
- `readflow/`：读型客服子图模板，封装“缓存 -> 可选 RAG -> ADK Agent”。

## Recommended Reading Order

1. `orchestrator/graph/builder.go`
2. `orchestrator/node/global/understanding/`
3. `orchestrator/subgraph/readflow/`
4. `orchestrator/node/shared/subgraph_agent.go`
5. `orchestrator/subgraph/productservice/`
6. `orchestrator/subgraph/orderservice/`
7. `orchestrator/subgraph/promotionservice/`
8. `orchestrator/subgraph/aftersalespolicy/`
9. `orchestrator/subgraph/addtocart/`
10. `orchestrator/subgraph/aftersalesapply/`
