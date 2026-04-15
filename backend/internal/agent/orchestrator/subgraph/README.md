# subgraph：业务子图（Eino）

每个业务子目录对应主图 `RouteNode` 的一条出边。子图对外类型为 **`compose.Graph[GraphInput, Output]`**：

- **`GraphInput`**：子图入口显式入参（Eino 边上类型），由主图 **`Prepare*InputNode`** 调用各包 **`InputFromState(*domain.State, …)`** 组装。
- **`Output`**：子图对主图的契约（话术、工具消息、追问标记等），经主图 **`Apply*ResultNode`** 写回 `*domain.State`。

主图线性段仍以 `*domain.State` 为主，配合 **`StatePre` / `StatePost`**（见 `../graph/state_handlers.go`）。主图与子图之间的节点注册集中在 **`../graph/subgraph_bridge.go`**。

总览见 **[`docs/面试-AI客服工作流.md`](../../docs/面试-AI客服工作流.md)**。子图只产出 **`Output`**；对外模板与落库在主图 **`FinalizeNode`**。

---

## 单包文件约定（学习/Eino 对齐时建议统一）

| 文件 | 职责 |
|------|------|
| **`graph.go`** | `Build`：仅注册节点、边、分支；`compose.NewGraph[GraphInput, Output]`。 |
| **`io.go`** | **`GraphInput`**、**`Output`**、**`InputFromState`**（子图 I/O 契约；入口与 Session 解耦：`InputFromState` 用新 map / 解引用再取址等方式避免误共享；需读 `Recorder` 等仍在 `ProcessState`）。 |
| **`workflow.go`** | 子图内 Lambda、中间 wire 类型、分支函数；**优先用边上类型传递**，仅在需要读 Recorder 等共享资源时使用 `domain.ProcessState`。 |
| **`metadata/`** | 工具/技能白名单等与路由相关的静态配置。 |
| **`config/`**（若有） | 本子图可调参数或规格说明。 |

---

## 各包节点与边（概览）

### `orderquery/`

`START` → `OrderQueryPrepareSlotsNode` → `OrderQueryModelAgentNode` → `OrderQueryRulePlanNode`（槽位；`order_ref`→CurrentRefs→`order_id`）→ `END`

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

### `fallback/`（主图中 **BaseQAGraph**）

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
