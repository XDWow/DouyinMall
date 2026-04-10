# subgraph：业务子图（Eino 多节点）

每个业务子目录对应主图的一条路由；**`graph.go`** 的 `Build` 注册 **`compose.Graph[struct{}, Output]`**（入口占位，节点内 **`ProcessState` 读 State**），**`workflow.go`** 放中间类型与分支逻辑。

总览见 **[`docs/面试-AI客服工作流.md`](../../docs/面试-AI客服工作流.md)**（主图 0→1、子图 DAG、同步/异步）。子图只产出 **`Output`**；对外话术与落库在主图 **`FinalizeNode`**。

---

## 各包节点与边（概览）

### `orderquery/`

`START` → `OrderQueryPrepareSlotsNode` → `OrderQueryModelAgentNode` → **分支** → `OrderQueryAgentAnswerNode` | `OrderQueryRulePlanNode` → `END`

- 模型有非空答复则走 `AgentAnswer`；否则 `OrderReadNode` + `ToolExec`。

### `inventory/`

`START` → `InventoryCheckSlotsNode` → **分支** → `InventoryMissingSlotsNode` | `InventoryModelAgentNode` →（后者再 **分支**）→ `InventoryAgentAnswerNode` | `InventoryRulePlanNode` → `END`

### `addtocart/`

与 `inventory` 同形：`AddToCartCheckSlotsNode` → 追问 | `AddToCartModelAgentNode` → 模型答复 | `AddToCartRulePlanNode`；规则/模型路径末尾统一 **Recorder hydrate + 成功模板**（`workflow.go`）。

### `productinfo/`

`START` → `ProductInfoL1TryNode` → **分支**（L1 命中 `ProductInfoL1OutputNode`）| `ProductInfoPrepareSlotsNode` → **分支**（缺槽 `ProductInfoMissingSlotsNode`）| `ProductInfoRAGNode` → `ProductInfoModelAgentNode` → **分支** → `ProductInfoAgentAnswerNode` | `ProductInfoRulePlanNode` → `END`

- RAG 仅在咨询类问法且配置了 RAG 时执行（与原逻辑一致）。

### `returnpolicy/`

`START` → `ReturnPolicyL1TryNode` → **分支** → `ReturnPolicyL1OutputNode` | `ReturnPolicyRAGNode` → `ReturnPolicyModelAgentNode` → `ReturnPolicyBuildOutputNode` → `END`

- 无模型或模型无输出时 `FinalAnswer` 可为空，由主图 Finalize 模板兜底。

### `fallback/`

`START` → `FallbackL1TryNode` → **分支** → `FallbackL1OutputNode` | `FallbackRAGNode` → `FallbackModelAgentNode` → **分支** → `FallbackAgentOutputNode` | `FallbackBaseQANode` → `END`

### `returnexchange/`

`START` → `ReturnExchangeInitSlotsNode` → **分支** → `ReturnExchangeMissingSlotsNode` | `ReturnExchangeQueryNode` → `ReturnExchangeEligibilityNode` → `ReturnExchangeConfirmNode` → **分支** → `ReturnExchangeSubmitToolsNode` → `ReturnExchangeSubmitInvokeNode` |（直达）`ReturnExchangeAssembleOutputNode` → `END`

- 两条路径在 `ReturnExchangeAssembleOutputNode` 汇合（提交路径经 `SubmitInvoke` 再汇总）。

### `toolexec/`

独立小图：仅负责按 `ToolCallPlan` 执行工具，可被主图或其它子图复用。

---

## 公共能力（不在 `subgraph/` 根下）

- `orchestrator/node/shared/subgraph_agent.go`：子图内模型 + 白名单工具多轮
- `orchestrator/node/domain/*`：各领域规则节点（`ToolCallPlan` 等）
- `subgraph/metadata/resolver.go`：按路由解析白名单 tool/skill 名
