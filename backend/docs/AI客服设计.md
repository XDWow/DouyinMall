# AI 客服（Agent）微服务设计

> 这是一份面试导向的设计文档。重点不是罗列技术点，而是展示：面对"给电商系统加一个 AI 客服"这个需求，我是怎么思考的。

简单架构：
   用户输入
   → RAG (Milvus 召回知识)
   → LLM 决策 (选 tool / 直接回答)
   → MCP tool 执行
   → 流式输出
Skills 的本质：预定义的 tool 调用编排，告诉 LLM"遇到 X 业务场景，按 A→B→C 顺序调这些 tool"。本质是把业务流程硬编码成规则说明书。
什么时候加 skill？ — 当发现某个高频复杂流程 LLM 总是调错顺序或漏调 tool 时，再把那个流程固化成 skill 也不迟。现在加是过早设计。

## 一、需求分析：AI 客服到底要解决什么问题？

在动手设计之前，先想清楚三个问题。

**用户的核心诉求是什么？**

电商客服 80% 的问题集中在：订单状态、物流进度、退货流程、商品咨询、优惠活动。这些问题有两个特点：
- 答案是**确定的**（不是开放式创作），适合 RAG 而非纯 LLM 生成
- 需要**查业务数据**（我的订单到哪了？），纯知识库不够，需要调用业务服务

**AI 客服和搜索服务的 AI 有什么区别？**

搜索服务已经有 AI 能力（Query 理解 + RAG 摘要），但客服是完全不同的场景：

| 维度 | AI 搜索 | AI 客服 |
|------|---------|---------|
| 交互模式 | 单轮 | 多轮（有上下文） |
| 数据源 | ES 商品索引 | 知识库 + 业务服务 |
| 向量库 | ES kNN | Milvus（语义为主） |
| 输出 | 商品列表 + 摘要 | 自然语言回复 + 可能的操作 |
| 状态 | 无状态 | 有会话（Redis + MySQL） |

所以 AI 客服是**独立微服务**，不是在搜索服务上加功能。但两者共用 `internal/search/infra/ai`（LLMClient、Embedder 接口），因为调 LLM 本质就是 HTTP 请求，用 Go 包复用就够了，不需要 LLM 网关服务。

**项目边界？**

- **Phase 1（核心对话）**：多轮对话 + 意图识别 + RAG 知识问答 + 置信度降级 + 滑动窗口摘要压缩
- **Phase 1.5（体验增强）**：流式输出 + Fallback Chain 熔断器 + 主动推荐 + 转人工 Handoff Summary
- **Phase 2（智能升级）**：Tool Calling（LLM 自主调用订单/物流服务获取实时数据）
- **Phase 3（数据飞轮）**：对话洞察分析 + 低置信度聚类 + 知识库盲区自动发现

---

## 二、核心设计：四阶段 Pipeline

核心思路：**把一次对话拆成四个阶段，每个阶段职责单一、可独立降级、可独立监控。**

```
用户消息："退货怎么弄？"
  │
  ▼
Stage 1: 快速路径（不依赖会话上下文：缓存的目标是复用跨会话的高频通用问题（退货、运费、发票……））
  ├─ 1a. 消息限频检查（基于 user_id 限频，不加载消息历史）
  ├─ 1b. Embedder.Embed(原始消息) → 向量 v1
  ├─ 1c. 用 v1 在 Milvus 缓存集合搜索 Top-1
  │       相似度 ≥ 0.95 → 命中 → Redis 取回复文本 → 直接返回（<50ms）
  │                                ↑ 全程不加载会话历史，也不调 LLM
  └─ 未命中 → 进入 Stage 2

Stage 2: 会话加载 + 意图识别 + Query 改写
  ├─ 2a. Redis 加载会话上下文（历史消息 + 摘要）← 缓存未命中才执行
  ├─ 2b. 单次 LLM 调用：原始消息 + 最近 3 轮历史
  │       → intent="RETURN", confidence=0.92, rewritten_query="退货申请流程"
  │       ↑ 改写用到了历史，解决"它""这个"等指代词歧义
  └─ 快速路径：intent=TRANSFER_TO_HUMAN → 生成 Handoff Summary → 直接返回

Stage 3: RAG 检索（两阶段）
  ├─ 3a. Embedder.Embed(rewritten_query) → 向量 v2（改写后，比 v1 更精准）
  ├─ 3b. 用 v2 在 Milvus 知识库集合搜索 Top-20 候选
  └─ 3c. LLM 重排 → Top-3 最相关知识片段

Stage 4: 回复生成
  ├─ 拼装：System Prompt + 历史摘要 + Top-3 知识 + 最近 10 轮消息 + 用户输入
  ├─ LLM 生成 → 解析 reply + confidence
  ├─ 置信度降级（<0.5 建议转人工，<0.8 追加免责提示）
  ├─ confidence ≥ 0.8 → 把 v1 + reply 异步写入语义缓存（供下次命中）
  └─ 异步落库（MySQL + 更新 Redis 会话）
```

**为什么会话加载放 Stage 2 而不是 Stage 1？**

语义缓存完全不需要上下文：它只比较"这句话的语义"是否匹配过。如果把会话加载放到 Stage 1，缓存命中时这次 Redis 读取就白费了。把会话加载推迟到 Stage 2（缓存未命中后），是**延迟加载**的思路——只在真正需要上下文的时候才加载。

**两次 Embedding 的区别**：

| | 向量化对象 | 用途 |
|---|---|---|
| Stage 1（v1） | 用户**原始消息** | 查/存语义缓存 |
| Stage 3（v2） | LLM **改写后的 Query** | 知识库检索 |

v1 用原始消息：因为缓存的意义是"相同的问法命中相同答案"，用户下次还是用口语问。v2 用改写后的 Query：知识库里存的是规范化文本，用精确 Query 召回率更高。

**为什么语义缓存不用"原始消息 + 上下文"一起做向量？**

可以，但命中率会极低。加入上下文后，同一个问题在不同对话轮次中向量差异很大，几乎永远匹配不上缓存。语义缓存的价值在于**跨会话**复用高频答案（退货、运费、物流这类不依赖个人订单的问题），这类问题本身就不依赖上下文——用原始消息做向量就够了。

**为什么这样设计？三个关键决策：**

1. **语义缓存放最前面**：高频问题（"怎么退货""运费多少"）不需要每次走 LLM。用向量相似度匹配，"怎么退货"和"退货流程是什么"命中同一条缓存。这不是字符串缓存，是语义级的。

2. **意图识别和 Query 改写合并为一次 LLM 调用**：用户说"它什么时候到"，直接 embedding 召回效果差。改写为"用户购买的裙子物流到达时间"才精准。合并两个任务省一次网络往返，不增加延迟。

3. **每个 Stage 可独立降级**：意图识别挂了 → 降级 UNKNOWN 继续走；RAG 没召回 → 纯 LLM 回答；LLM 挂了 → 模板兜底。任何一环故障不导致整体不可用。

---

## 三、关键设计决策（面试重点展开）

### 3.1 上下文管理：滑动窗口 + 摘要压缩

**问题**：多轮对话越来越长，全部丢给 LLM 会导致 Token 成本爆炸，且超出上下文窗口。

**方案**：

```
消息列表（按时间顺序）：
  msg_1, msg_2, msg_3, ..., msg_18, msg_19, msg_20, [当前输入]
                                    ↑ 滑动窗口（保留最近 N=10 轮）

超出窗口的 msg_1~msg_10：
  → LLM 压缩为一段摘要（~150 字），保留核心信息，不至于完全丢失
  → 存入 session.summary 字段
```

实际发给 LLM 的 messages 数组：
```
[系统提示] → [历史摘要] → [知识库上下文] → [最近10轮] → [当前输入]
```

**为什么不直接截断？** 截断会丢失关键上下文。比如用户第 3 轮说"我买了一条裙子"，第 15 轮问"它能退吗"——如果截掉第 3 轮，LLM 不知道"它"指什么。摘要能保留核心信息。

**为什么不用无限长窗口模型（如 128K）？** Token 按量计费，对话越长成本越高。滑动窗口 + 摘要把每次调用控制在约 2000 Token 以内。

### 3.2 LLM 容错链（Fallback Chain）

**问题**：LLM 是外部依赖，可能超时、限流、服务端错误。客服场景不能让用户看到"系统错误"。

**方案**：三层容错 + 熔断器

```
Primary Model (Qwen-72B, 超时 10s)
    │ 失败
    ▼
Fallback Model (Qwen-7B, 超时 5s)
    │ 失败
    ▼
Template Response（基于意图的模板回复）
    │ 意图也没有
    ▼
兜底："抱歉，系统繁忙，请联系人工客服"
```

**熔断器**：连续 5 次失败 → 30s 内直接跳过该层走下一层，避免无意义重试。

**面试追问：为什么不只用一个模型？** 大模型质量好但贵且慢，小模型快但质量差。正常情况用大模型保质量，异常时降级到小模型保可用性，是质量和可用性的 trade-off。

### 3.3 语义缓存的设计细节

**命中判断**：
```
用户输入 → Embedding → 在缓存向量集合中搜索
  ├─ COSINE 相似度 ≥ 0.95 → 命中，直接返回缓存回复
  └─ < 0.95 → 未命中，走完整 Pipeline
```

**为什么阈值是 0.95 而不是 0.8？** 客服场景对准确性要求高。0.8 会导致"怎么退货"和"怎么换货"命中同一条缓存，但这两个问题的答案不同。0.95 足够严格，只匹配真正语义相同的问题。

**存储**：Redis（`agent:cache:{hash}` → 回复 JSON，TTL 1h）+ Milvus 小集合存向量。

**缓存失效**：知识库更新时清空相关 category 的缓存。

### 3.4 为什么用 Milvus 不用 ES kNN？

搜索服务用 ES kNN 是因为**同时需要全文检索和向量检索**，放在一个引擎里避免双写。

客服知识库不需要全文检索（不需要布尔过滤、分词匹配），纯语义召回。Milvus 在纯向量检索场景下：
- 支持 IVF_FLAT / HNSW 多种索引，调优灵活
- 大规模向量检索性能比 ES kNN 好
- 独立部署，不影响搜索 ES 集群的稳定性

### 3.5 Tool Calling 安全设计（Phase 2）

LLM 自主决定调用哪个工具，这意味着要防止**幻觉导致的错误调用**。

四道防线：
1. **白名单**：只允许调用预定义的 4 个工具（get_order / search_product / get_logistics / check_refund_policy）
2. **参数校验**：order_id 必须属于当前 user_id，防止越权查询
3. **只读约束**：所有工具只能查询，不能执行写操作（退款、下单需人工确认）
4. **轮次限制**：单次对话最多调用 3 次工具，防止死循环

### 3.5.1 Tool Calling + 流式输出的实现原理

Tool Calling 和流式输出结合时，需要理解 OpenAI 协议的工作方式。

**核心协议行为**

LLM 的单次响应只有两种结果（不会混合）：
- `finish_reason=stop`：纯文本输出，逐 token 流式推出
- `finish_reason=tool_calls`：工具调用，参数分散在多个 chunk 中逐步到达

**Streaming + Tool Calling 完整流程**

```
1. Agent 发请求（带 tools 定义 + stream=true）
         ↓
2. 收 SSE chunks：
   - delta.content 非空   → 推前端（用户看到打字效果）
   - delta.tool_calls 非空 → 在 goroutine 内部静默累积参数（用户不可见）
   - finish_reason="tool_calls" → 参数累积完毕，打包发出
         ↓
3. Agent 同步执行工具（调 MCP Server）
         ↓
4. 把工具结果塞进 messages，再发一次请求（stream=true）
         ↓
5. LLM 读懂工具结果，生成自然语言回复，逐 token 流式推前端
   finish_reason="stop" → 等 [DONE] 到来，channel 自然关闭
```

**为什么工具参数需要累积？**

流式模式下，`arguments` 是分片到达的：
```
chunk1: tool_calls[0].function.arguments = '{"qu'
chunk2: tool_calls[0].function.arguments = 'ery"'
chunk3: tool_calls[0].function.arguments = ':"耳机"}'
```
必须等到 `finish_reason="tool_calls"` 才能确认参数完整，不能提前执行工具。

**[DONE] 与 finish_reason 的关系**

```
...（文本或工具参数 chunks）
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}  ← 参数完毕
data: [DONE]                                                     ← 流结束
```

- `finish_reason="tool_calls"` → 工具参数完整的信号，立即触发工具执行
- `[DONE]` → HTTP 流结束信号，goroutine 收到后 `return`，`defer close(ch)` 关闭 channel
- `finish_reason="stop"` → 不需要额外处理，等 `[DONE]` 关闭 channel 即可

**三层职责划分**

```
pkg/ai（通用 LLM 通信层）
  → 处理 HTTP/SSE/JSON，累积分片参数
  → 只往 channel 发两种 token：Delta(文本) 或 ToolCalls(完整参数)
  → 不知道有 Agent、有会话

AIService（AI 能力层）
  → 消费 channel：Delta → 推前端，ToolCalls → 调 MCP Server → 继续下一轮
  → 对 ChatUseCase 完全封闭工具调用细节
  → 不知道有 HTTP 请求、有前端

ChatUseCase（业务编排层）
  → 只调 AIService.GenerateStream(...)
  → 拿到最终 reply，不知道中间调了几个工具
```

**GenerateStream 伪代码**

```go
for round := 0; round <= maxToolRounds; round++ {
    streamCh, _ = llm.ChatCompletionStream(ctx, req with tools)

    for tok := range streamCh {
        if tok.Delta != ""         { send(TEXT_DELTA) }   // 推前端
        if len(tok.ToolCalls) > 0  { 执行工具; 塞入 messages; break } // 调工具
    }
    // channel 关闭 = [DONE] 到来 = 这轮结束
    if 没有工具调用 { return }  // finish_reason=stop，结束
}
```

### 3.6 流式输出：Pipeline 阶段可视化 + 逐字生成

**问题**：同步 API 要等整个 Pipeline 跑完（500ms~2s）才返回，用户看到的是"发送 → 长时间空白 → 突然出现一大段"。所有大厂 AI 产品（豆包、ChatGPT、Kimi）都是流式输出，这已经是用户的基本预期。

**方案**：gRPC Server-Side Streaming，把一次回复拆成多个 Chunk 推送。

```protobuf
// 新增 Streaming RPC
rpc SendMessageStream(ChatRequest) returns (stream ChatStreamChunk);

message ChatStreamChunk {
  ChunkType type       = 1;
  string    text       = 2;   // TEXT_DELTA 时为文本增量
  string    stage      = 3;   // STAGE_UPDATE 时标识当前阶段
  ChatResponse final   = 4;   // DONE 时携带完整响应（含 handoff/suggested_questions）
}

enum ChunkType {
  STAGE_UPDATE = 0;  // Pipeline 阶段状态推送
  TEXT_DELTA   = 1;  // 回复文本增量（逐字/逐句）
  DONE         = 2;  // 结束标记
}
```

**前端效果时序**：

```
t=0ms    → [STAGE_UPDATE] stage="cache"      🔍 正在查找相似问题...
t=30ms   → [STAGE_UPDATE] stage="intent"     🧠 正在理解您的问题...
t=80ms   → [STAGE_UPDATE] stage="retrieval"  📚 正在检索知识库...
t=230ms  → [STAGE_UPDATE] stage="generating" ✍️  正在生成回复...
t=280ms  → [TEXT_DELTA]   text="退货流程"
t=310ms  → [TEXT_DELTA]   text="如下：\n1."
t=340ms  → [TEXT_DELTA]   text="打开订单详情页"
...逐字推送...
t=800ms  → [DONE]         final={reply, intent, suggested_questions, ...}
```

**实现要点**：

```
Pipeline 内部                              gRPC Stream
───────────                               ──────────
Stage 1 缓存检查 ─────────────────────────→ push STAGE_UPDATE("cache")
Stage 1 完成（未命中）────────────────────→ push STAGE_UPDATE("intent")
Stage 2 完成 ─────────────────────────────→ push STAGE_UPDATE("retrieval")
Stage 3 开始生成 ─────────────────────────→ push STAGE_UPDATE("generating")
LLM SSE 返回 token_1 ────────────────────→ push TEXT_DELTA(token_1)
LLM SSE 返回 token_2 ────────────────────→ push TEXT_DELTA(token_2)
...
LLM SSE [DONE] ──────────────────────────→ push DONE(final)
```

**关键设计决策**：

1. **LLM 调用从同步改为 SSE Streaming**：`StreamClient` 接口的 `ChatCompletionStream` 方法返回 `<-chan ai.StreamToken` token 流。`StreamToken` 含三个字段：

   ```go
   type StreamToken struct {
       Delta     string     // 文本增量 → 直接推前端
       ToolCalls []ToolCall // 非空 → 工具参数累积完毕，去调 MCP Server
       Err       error
   }
   ```

   `Delta` 和 `ToolCalls` 互斥：同一个 token 只会是其中一种。`openai.go` 内部用 goroutine-local 的 `toolAccs` 静默累积分片参数，直到 `finish_reason="tool_calls"` 才打包成一个 `StreamToken{ToolCalls:[...]}` 发出；文本 chunk 则每收到一个就立即发出 `StreamToken{Delta:...}`。`FinishReason` 字段不在 StreamToken 里——channel 关闭本身就代表"这轮流结束"，ToolCalls 非空即代表"需要调工具"，无需额外信号。

2. **非流式 RPC 保留**：`SendMessage`（同步）和 `SendMessageStream`（流式）并存。同步 API 内部复用同一套 Pipeline，只是最后收集所有 token 拼接后一次返回。方便前端灰度切换和接口兼容。

3. **token 攒批推送**：不逐 token 推（太碎，网络开销大），攒到 6 个字符或遇到标点时推一次，平衡实时性和网络效率。流式路径在末尾保留 `len("===META===")` 字节的安全缓冲区，避免分隔符跨多个 token 时被误作回复文本推出。

4. **Stage 语义缓存命中时的特殊处理**：如果语义缓存命中，没有生成阶段，直接推 `STAGE_UPDATE("cache_hit")` → `TEXT_DELTA(完整回复)` → `DONE`，响应时间 <50ms。

5. **LLM 输出格式采用分隔符而非 JSON**：LLM 按 `<自然语言回复>\n===META===\n{"confidence":...,"emotion":...,"suggested_questions":[...]}` 格式输出。流式路径只推分隔符之前的文本 token，前端看到的是干净的自然语言而非 JSON 原文（避免用户看到 `{"reply":"这款手机..."}`）。分隔符后的元数据缓存在内存中供 `parseReply` 后处理，不推给前端。向后兼容：如果 LLM 返回 JSON 整体格式（`{"reply":"...","confidence":...}`），`parseReply` 会以 JSON 兜底解析。

6. **阶段进度实时推送**：`runPipelineStages` 接受 `stageNotifier` 回调参数，每个阶段完成时调用回调推送 `STAGE_UPDATE`。同步路径传 `nil`（不推送），流式路径传实际的 channel 写入函数，实现真正的实时进度而非"结果出来后补推"。

**面试追问：为什么不用 WebSocket？**

gRPC Streaming 比 WebSocket 优势：① 强类型 protobuf 消息，不用自己定义分帧协议；② 天然支持 metadata / deadline / cancellation；③ 和项目已有的 RPC 框架统一，不引入新的通信协议。前端如果是 Web 端，通过 grpc-gateway 转为 SSE 即可。

### 3.7 LLM 容错链 + 熔断器（Fallback Chain + Circuit Breaker）

**问题**：LLM 是外部依赖，可能超时、限流、服务端 500。Pipeline 中有三处调用 LLM（意图识别+query rewrite、重排、回复生成），任何一处挂掉都不能让用户看到"系统错误"。

**方案**：`FallbackLLMClient` 装饰器 + 每个节点独立熔断器

```
请求到达
  │
  ▼
┌─────────────────────────────────────────────────────┐
│ FallbackLLMClient（实现 ai.LLMClient 接口）         │
│                                                     │
│  Node 1: Qwen-72B (Primary)                        │
│  ┌──────────┐                                       │
│  │ Breaker  │── Closed ──→ 正常调用 ──→ 成功 → 返回 │
│  │ state    │── Open ────→ 跳过，试下一层            │
│  │          │── HalfOpen → 放行1个试探请求           │
│  └──────────┘       │ 失败                          │
│                     ▼                               │
│  Node 2: Qwen-7B (Fallback)                        │
│  ┌──────────┐                                       │
│  │ Breaker  │── 同上逻辑                            │
│  └──────────┘       │ 失败                          │
│                     ▼                               │
│  Node 3: TemplateEngine（无 LLM 依赖）              │
│  根据 intent 返回预置模板回复                        │
│       │ intent 也没有                               │
│       ▼                                             │
│  兜底: "抱歉，系统繁忙，请联系人工客服"              │
└─────────────────────────────────────────────────────┘
```

**熔断器状态机**：

```
         连续 N 次失败
Closed ─────────────────→ Open
  ↑                         │
  │ 试探成功                │ cooldown 超时
  │                         ▼
  └─────────────────── HalfOpen
         试探失败 ──→ 回到 Open
```

| 参数 | 值 | 说明 |
|------|-----|------|
| `failureThreshold` | 5 | 连续失败 5 次触发熔断 |
| `cooldown` | 30s | Open 状态持续 30s 后进入 HalfOpen |
| `halfOpenMax` | 1 | HalfOpen 只放行 1 个请求试探 |

**核心数据结构**：

```go
// FallbackLLMClient 实现 ai.LLMClient 接口
// 内部维护有序 LLM 节点列表，每个节点有独立熔断器
type FallbackLLMClient struct {
    nodes    []llmNode
    template *TemplateEngine
    metrics  *PipelineMetrics
    logger   logger.LoggerV1
}

type llmNode struct {
    name    string         // "qwen-72b" / "qwen-7b"
    client  ai.LLMClient   // 底层 HTTP 客户端
    breaker *CircuitBreaker
    timeout time.Duration  // 每层超时独立配置
}

// CircuitBreaker 轻量级熔断器（无第三方依赖）
type CircuitBreaker struct {
    mu          sync.Mutex
    state       BreakerState   // Closed / Open / HalfOpen
    failures    int32          // 连续失败计数
    threshold   int32          // 触发熔断的失败次数
    lastFailure time.Time
    cooldown    time.Duration  // Open → HalfOpen 的冷却时间
}

// Allow 判断是否允许请求通过
func (cb *CircuitBreaker) Allow() bool
// RecordSuccess 记录成功，重置计数器，状态回 Closed
func (cb *CircuitBreaker) RecordSuccess()
// RecordFailure 记录失败，累加计数器，可能触发 Open
func (cb *CircuitBreaker) RecordFailure()
```

**`FallbackLLMClient.ChatCompletion` 核心逻辑**：

```go
func (f *FallbackLLMClient) ChatCompletion(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
    for i, node := range f.nodes {
        // 熔断器检查
        if !node.breaker.Allow() {
            f.logger.Warn("熔断跳过", logger.String("model", node.name))
            continue
        }

        // 带独立超时的调用
        nodeCtx, cancel := context.WithTimeout(ctx, node.timeout)
        resp, err := node.client.ChatCompletion(nodeCtx, req)
        cancel()

        if err == nil {
            node.breaker.RecordSuccess()
            if i > 0 { // 降级成功也要记录
                f.logger.Info("降级成功", logger.String("model", node.name))
            }
            return resp, nil
        }

        // 记录失败
        node.breaker.RecordFailure()
        f.logger.Warn("LLM 调用失败，尝试下一层",
            logger.String("model", node.name), logger.Error(err))
    }

    // 所有 LLM 节点都失败 → 模板兜底
    f.logger.Error("所有 LLM 节点不可用，走模板兜底")
    f.metrics.IncTemplateFallback()
    return f.template.Generate(req)
}
```

**TemplateEngine 模板引擎**：

当所有 LLM 都不可用时，基于之前识别的意图返回预置回复：

```go
var intentTemplates = map[IntentType]string{
    IntentReturn:       "退货流程：请在订单详情页点击「申请退货」，...\n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"退货运费谁承担\",\"退款多久到账\"]}",
    IntentLogistics:    "物流查询：请在「我的订单」页面查看物流信息，...\n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"物流多久能到\",\"可以修改收货地址吗\"]}",
    IntentPayment:      "支付问题：请检查支付方式是否正常，...\n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"支持哪些支付方式\",\"付款后多久发货\"]}",
    IntentOrderInquiry: "订单查询：请在「我的订单」页面查看订单状态，...\n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"如何取消订单\",\"订单状态说明\"]}",
    // ... 每个意图一条兜底模板，统一使用 ===META=== 分隔符格式
}
```

**面试关键点**：

1. **为什么自研熔断器而不用 Hystrix / sentinel-go？** 需求极简（只需要连续失败计数 + 冷却时间），引入重量级框架不值得。50 行代码就能实现，且完全可控。

2. **为什么大模型和小模型各配独立超时？** 72B 推理慢但质量好，给 10s；7B 推理快但质量差，给 5s。如果小模型也要 10s 才能返回，说明底层有问题，不如直接走模板。

3. **Prometheus 指标怎么用？** `douyinmall_agent_template_fallback_total` 记录模板兜底次数。如果持续增长，说明所有 LLM 节点都不可用，需要运维介入。配合 `douyinmall_agent_llm_error_total` 判断是单节点还是全局故障。

### 3.8 转人工 Handoff Summary：AI → 人工的无缝交接

**问题**：用户转人工后，人工客服要从头翻聊天记录才能理解上下文，平均每次多花 1~2 分钟。大厂的做法是让 AI 自动生成一份结构化的交接摘要，人工客服一眼就能接手。

**触发时机**：三种场景会触发 Handoff：

| 场景 | 触发条件 | 说明 |
|------|---------|------|
| 用户关键词转人工 | `quickDetectTransfer()` 命中 | 消息包含"转人工""人工客服"等 8 个关键词之一，直接触发，跳过 LLM（节省 ~200ms RTT） |
| LLM 意图识别转人工 | intent == TRANSFER_TO_HUMAN | 隐晦表达（如"你帮不了我"）通过 LLM 意图识别捕获 |
| 系统自动升级（低置信度） | 连续 3 轮 confidence < 0.5 | AI 连续回答不了，主动建议转人工 |
| 系统自动升级（情绪+低置信度） | emotion=angry/urgent 且 低置信度 ≥ 1 轮 | 用户情绪激动 + AI 不确定 → 加速升级（不等 3 轮） |

**复合自动升级规则**（`postGenerate` 中判断）：

```go
needEscalate := false
switch {
case session.LowConfidenceTurns >= 3:
    // 连续 N 轮低置信度：AI 持续无法解决
    needEscalate = true
case (emotion == "angry" || emotion == "urgent") && session.LowConfidenceTurns >= 1:
    // 用户情绪激动 + 至少一次低置信度：加速升级
    needEscalate = true
}
```

**emotion 字段**来源：LLM 回复中 `===META===` 后的 JSON 包含 `"emotion"` 字段，取值 `neutral / mild_frustration / angry / urgent`，由 LLM 根据用户消息和上下文判断。

**转人工消息持久化**：`persistTransfer()` 方法确保用户的转人工消息和系统回复写入历史记录（不走 `persistAsync`，因为没有 LLM 生成结果），避免对话历史中丢失转人工请求。

**Handoff Summary 结构**：

```json
{
  "session_id": "sess_12345_1709000000",
  "user_id": 12345,
  "summary": {
    "core_issue": "用户3天前购买的连衣裙（订单号 202503280001）已签收但实物颜色与商品图不符，要求退货退款",
    "ai_actions": [
      "已告知用户7天无理由退货政策",
      "已引导用户在订单详情页申请退货",
      "用户反馈找不到退货入口（可能是 App 版本问题）"
    ],
    "escalation_reason": "用户操作遇阻，AI 无法远程协助定位 App 界面问题",
    "user_emotion": "mild_frustration",
    "entities": {
      "order_id": "202503280001",
      "product": "连衣裙",
      "problem_type": "色差"
    }
  },
  "conversation_turns": 8,
  "ai_confidence_trend": [0.92, 0.85, 0.78, 0.65, 0.52, 0.45, 0.40, 0.38]
}
```

**生成方式**：一次 LLM 调用，用专用 Prompt 压缩整个对话：

```go
const handoffPrompt = `请基于以下客服对话，生成一份结构化的交接摘要，帮助人工客服快速接手。

要求输出 JSON：
{
  "core_issue": "一句话概括用户的核心诉求",
  "ai_actions": ["AI 已经做了什么（列举关键动作）"],
  "escalation_reason": "为什么需要转人工",
  "user_emotion": "neutral / mild_frustration / angry / urgent",
  "entities": {"从对话中提取的关键实体，如订单号、商品名等"}
}

对话记录：
%s`
```

**实现流程**：

```
quickDetectTransfer 命中 / intent == TRANSFER_TO_HUMAN / 复合自动升级
    │
    ▼
LLM 生成 Handoff Summary（复用 FallbackLLMClient）
    │
    ├─ 成功 → 摘要写入 session.handoff_summary 字段
    │         更新 session.status = SESSION_HUMAN
    │         调用 persistTransfer 持久化用户消息 + 转人工回复
    │         返回给前端展示 + 推送到人工客服工作台
    │
    └─ 失败 → 降级：拼接最近 5 轮对话原文作为摘要
              仍然完成转人工流程
```

**Proto 扩展**：

```protobuf
// ChatResponse 新增字段
message ChatResponse {
  // ... 已有字段 ...
  HandoffSummary handoff = 7;  // 转人工时携带交接摘要
}

message HandoffSummary {
  string core_issue        = 1;
  repeated string ai_actions = 2;
  string escalation_reason = 3;
  string user_emotion      = 4;
  map<string,string> entities = 5;
  repeated float confidence_trend = 6;  // 置信度变化趋势
}
```

**面试怎么讲**：这个功能体现的不是技术难度，而是**产品思维**——你不只是做一个 Chat API，而是关注了 AI 客服和人工客服之间的**工作流衔接**。`confidence_trend` 字段可以让人工客服一眼看到 AI 在哪一轮开始"答不上来"，快速定位问题所在。

### 3.9 主动推荐：Suggested Questions

**问题**：用户问完一个问题后，往往还有关联问题，但不知道怎么问，或者不知道 AI 能回答什么。如果用户找不到继续问的入口，就会直接转人工——这就浪费了 AI 能解决的机会。

**方案**：每次 AI 回复后，附带 2~3 个关联推荐问题。

```
用户：怎么退货？
AI：退货流程如下：1. 进入订单详情 2. 点击"申请退货" 3. 选择原因并提交
    
    💡 您可能还想知道：
    ├─ 退货运费由谁承担？
    ├─ 退款多久到账？
    └─ 可以只换货不退款吗？
```

**Proto 扩展**：

```protobuf
message ChatResponse {
  // ... 已有字段 ...
  repeated string suggested_questions = 6;  // 推荐追问（2~3个）
}
```

**生成策略**：在 System Prompt 的 `===META===` 元数据 JSON 中要求 LLM 输出 `suggested_questions`：

```
6. suggested_questions 给出 2~3 个与当前话题相关的追问建议，必须是 AI 有能力回答的，不要重复当前问题

严格按以下格式输出（不要任何 markdown 代码块，分隔符必须单独成行）：
<自然语言回复内容>
===META===
{"confidence":0.85,"emotion":"neutral","suggested_questions":["退货运费谁承担？","退款多久到账？"]}
```

**为什么不用独立的推荐模型？** 关联问题和当前回复强相关，用同一次 LLM 调用生成最自然。独立模型需要额外一次网络调用，且缺少当前对话的上下文，推荐质量反而差。

**降级处理**：如果 LLM 没返回 `suggested_questions`、`===META===` 解析失败、或 JSON 兜底解析也失败，就不展示推荐——这是一个锦上添花功能，不影响核心对话。

**面试追问：推荐问题会不会引导用户到 AI 答不了的方向？**

不会。Prompt 明确要求"推荐问题必须是 AI 有能力回答的"。更进一步，可以在后处理中用语义检索验证：推荐问题 → Embedding → Milvus 检索，如果知识库中没有高相关结果，就过滤掉这条推荐。这是一个"以检索验生成"的自洽机制。

---

## 四、数据存储方案

三类数据，三种存储，各司其职：

```
                    热数据               冷数据              语义数据
存储引擎：          Redis               MySQL               Milvus
存什么：        活跃会话+消息窗口      全量会话+消息历史      知识向量+缓存向量
访问模式：      每次对话都读写         异步落盘+分页回查      Embedding检索
TTL：           24h 自动过期          永久保留              永久保留
```

**读写策略**：
- **写路径**：先写 Redis → 异步写 MySQL（消息落盘不阻塞回复，用 goroutine）
- **读路径**：Redis 优先 → miss 时回源 MySQL 并回填 Redis
- **为什么不用 MySQL 做主存储？** 客服对话对延迟敏感（用户在等回复），Redis 读写 <1ms，MySQL 要 5-10ms。用 Redis 做热层，MySQL 只做持久化。

**Redis 数据结构**：

| Key | 类型 | TTL | 说明 |
|-----|------|-----|------|
| `agent:session:{sid}` | String(JSON) | 24h | 会话元信息（JSON 序列化整个 Session 对象） |
| `agent:session:{sid}:msgs` | List | 24h | 最近 20 条消息（滑动窗口，LTrim 保留） |
| `agent:user:{uid}:active` | String | 24h | 用户当前活跃会话 ID |
| `agent:rate:{uid}` | String | 1min | 消息限频（Lua 脚本原子 INCR+EXPIRE，防进程崩溃导致无 TTL） |

**MySQL 表设计**：

```sql
CREATE TABLE agent_sessions (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    session_id  VARCHAR(64)  NOT NULL,
    user_id     BIGINT UNSIGNED NOT NULL,
    channel     VARCHAR(32)  NOT NULL DEFAULT 'web',
    status      TINYINT      NOT NULL DEFAULT 1 COMMENT '1活跃 2结束 3转人工',
    summary     TEXT         COMMENT '历史摘要',
    total_turns INT          NOT NULL DEFAULT 0,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_session_id (session_id),
    KEY idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE agent_messages (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    session_id  VARCHAR(64)  NOT NULL,
    role        VARCHAR(16)  NOT NULL COMMENT 'user/assistant/system/tool',
    content     TEXT         NOT NULL,
    intent      TINYINT      NOT NULL DEFAULT 0,
    confidence  FLOAT        NOT NULL DEFAULT 0,
    tokens_used INT          NOT NULL DEFAULT 0,
    latency_ms  INT          NOT NULL DEFAULT 0,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_session_created (session_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE knowledge_items (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    title       VARCHAR(256) NOT NULL,
    content     TEXT         NOT NULL,
    category    VARCHAR(64)  NOT NULL COMMENT 'faq/product/policy/promotion',
    vector_id   VARCHAR(64)  DEFAULT '',
    status      TINYINT      NOT NULL DEFAULT 1,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_category (category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**Milvus Collection**：
- `knowledge_vectors`：知识库向量（id, vector[768], category），IVF_FLAT 索引，COSINE 度量
- `query_cache_vectors`：语义缓存向量（id, vector[768]），用于缓存命中检测

---

## 五、可观测性：面试加分的工程细节

**为什么单独说可观测性？** 因为 AI 系统最大的问题是"黑盒"——出了问题不知道哪个环节出的。Pipeline 设计的一个核心目的就是让每个阶段可独立观测。

**Prometheus 指标**（纯 Prometheus 埋点，不在 API 返回调试信息，避免内部数据泄露到客户端）：

| 指标 | 类型 | 说明 |
|------|------|------|
| `douyinmall_agent_pipeline_stage_duration_ms` | Histogram(stage) | 各阶段耗时分布（cache/intent/embed/vector/rerank/generate/total），定位瓶颈 |
| `douyinmall_agent_cache_hit_total` | Counter | 语义缓存命中次数 |
| `douyinmall_agent_cache_miss_total` | Counter | 语义缓存未命中次数 |
| `douyinmall_agent_rate_limited_total` | Counter | 限频触发次数 |
| `douyinmall_agent_intent_total` | Counter(intent) | 各意图识别计数 |
| `douyinmall_agent_llm_error_total` | Counter | LLM 调用失败总次数 |
| `douyinmall_agent_auto_escalation_total` | Counter | 自动转人工触发次数（含低置信度和情绪触发） |
| `douyinmall_agent_emotion_escalation_total` | Counter(emotion) | 因用户情绪触发自动转人工计数 |
| `douyinmall_agent_fallback_sync_total` | Counter | 流式降级为同步生成次数 |
| `douyinmall_agent_template_fallback_total` | Counter | 模板兜底触发次数 |
| `douyinmall_agent_active_sessions` | Gauge | 当前活跃会话数 |

**为什么不在 API 返回 PipelineDebug？** 原始设计有一个 `PipelineDebug` message 每次请求返回各阶段耗时。实际考虑后移除了：① 内部调试信息暴露到客户端不安全；② Prometheus + Grafana 已经提供了更强的聚合分析能力；③ 减少响应体大小。

---

## 六、知识入库 Pipeline

知识不是凭空出现在 Milvus 里的。完整链路：

```
运营后台录入 / CSV 批量导入
    │
    ▼
knowledge_items 表（MySQL）
    │ 新增/更新
    ▼
Embedding Job（异步）
    │ 文本分段（chunk 200-500 字，50 字重叠）
    │ → 调用 pkg/ai.Embedder 批量向量化
    ▼
Milvus 写入 → 更新 MySQL.vector_id
```

**为什么要分段（Chunking）？** 一篇 2000 字的退货政策，整篇做一个向量太粗——用户问"7天无理由退货"，匹配整篇文档的向量不如匹配那一段的向量精准。分段后每段有独立语义，召回精度更高。

**重叠窗口**：段与段之间保留 50 字重叠，防止关键信息恰好被切断。

---

## 七、项目结构

```
internal/agent/
├── config/          # 配置（LLM 地址、Milvus 连接、Redis 配置等）
├── domain/          # 领域模型（Session, Message, KnowledgeItem, Intent 等）
├── usecase/         # 核心业务逻辑（ChatUseCase, 四阶段 Pipeline, FallbackLLMClient, CircuitBreaker, PipelineMetrics）
├── handler/         # Kitex gRPC handler（对应 agent.proto 的 AgentService）
├── infra/           # 基础设施（Redis 会话存储、MySQL 持久化、Milvus 检索）
│   ├── cache/       # Redis SessionRepo 实现 + 限频（Lua 原子 INCR+EXPIRE）
│   ├── persistence/ # MySQL 异步落盘（GORM DAO）
│   ├── repository/  # 组合 Redis 热层 + MySQL 冷层的 SessionRepo 实现
│   └── knowledge/   # Milvus KnowledgeRepo + SemanticCache 实现
└── ioc/             # 依赖注入（Wire）

pkg/ai/              # 共享 LLM/Embedding 客户端（实际路径 internal/search/infra/ai，搜索服务也用）
├── types.go         # LLMClient, StreamClient, Embedder 接口
└── openai_compatible.go  # OpenAI 兼容实现
```

---

## 八、对话洞察分析（数据飞轮）

> AI 客服不只是"回答问题"，更大的价值是从海量对话中**自动发现业务问题、驱动知识库迭代**。这就是"数据飞轮"——AI 越用越好。

### 8.1 为什么需要对话洞察？

```
                   ┌──────────────────────────────────┐
                   │        数据飞轮闭环               │
                   │                                  │
     用户对话 ──→ AI 客服回复 ──→ 异步分析 Pipeline    │
         ↑                           │                │
         │         ┌─────────────────┤                │
         │         ▼                 ▼                │
         │    运营仪表盘        知识库补充             │
         │    （发现问题）      （填补盲区）           │
         │         │                 │                │
         └─────────┴─────────────────┘                │
               AI 回答质量提升                        │
                   └──────────────────────────────────┘
```

没有洞察分析，知识库永远靠人工猜测去更新。有了洞察分析，**系统自动告诉你哪些问题答不好、哪些知识缺失**。

### 8.2 五个核心洞察指标

| 指标 | 数据来源 | 业务价值 |
|------|---------|---------|
| **高频意图 TOP-10** | `agent_messages.intent` 聚合统计 | 发现用户最关心什么，指导运营优先级 |
| **低置信度 Topic 聚类** | confidence < 0.5 的消息做 embedding → 聚类 | 发现知识库盲区，精准补充 |
| **平均解决轮数** | `agent_sessions.total_turns`（非转人工会话） | 衡量 AI 效率，轮数越少越好 |
| **转人工率 + 原因分布** | `session.status == HUMAN` 的比例 + handoff_summary.escalation_reason | 优化 AI 覆盖率的核心指标 |
| **用户满意度推断** | 对话最后一轮的情感分析（正面/中性/负面） | 粗粒度满意度，无需用户主动评价 |

### 8.3 异步分析 Pipeline

分析不在请求链路上，不影响对话延迟：

```
MySQL agent_messages 表
    │
    │ 定时任务（每小时 / 每天）
    ▼
InsightJob（Go 离线任务）
    │
    ├─ Step 1: 扫描最近 N 小时的低置信度消息
    │          → 批量 Embedding
    │          → K-Means 聚类（按语义分组）
    │          → 输出："这批问题都在问XX，但知识库没有覆盖"
    │
    ├─ Step 2: 统计意图分布、转人工率、平均轮数
    │          → 写入 agent_insights 表
    │
    ├─ Step 3: 对 handoff_summary.escalation_reason 做聚合
    │          → 输出：转人工 TOP-5 原因
    │
    └─ Step 4: 生成日报/周报摘要（可选：用 LLM 生成自然语言报告）
          → 推送到运营群
```

### 8.4 低置信度 Topic 聚类（最有价值的洞察）

这是整个洞察模块最核心的能力——**自动发现知识库的盲区**。

```go
// InsightService 对话洞察分析
type InsightService struct {
    dao      *persistence.AgentDAO
    embedder ai.Embedder
    logger   logger.LoggerV1
}

// DiscoverBlindSpots 发现知识库盲区
// 1. 查询近 24h confidence < 0.5 的 assistant 消息
// 2. 取对应的 user 消息（用户原始问题）做 embedding
// 3. 向量聚类 → 每个簇代表一类"AI 答不好的问题"
// 4. 每个簇取距质心最近的 3 条作为代表性问题
func (s *InsightService) DiscoverBlindSpots(ctx context.Context) ([]BlindSpotCluster, error)

type BlindSpotCluster struct {
    ClusterID     int
    Size          int       // 该簇包含多少条低质回复
    TopQuestions  []string  // 代表性问题（距质心最近的3条）
    SuggestedTopic string  // LLM 为该簇生成的主题摘要
}
```

**运营看到的效果**：

```
📊 知识库盲区日报（2026-03-01）

Cluster 1 (47 条)：会员积分相关
  - "积分怎么兑换？"
  - "积分有有效期吗？"
  - "积分能抵扣多少？"
  → 建议：补充「会员积分规则」知识条目

Cluster 2 (31 条)：跨境商品相关
  - "海外商品能退吗？"
  - "跨境商品关税谁付？"
  - "保税仓发货多久到？"
  → 建议：补充「跨境购物政策」知识条目

Cluster 3 (18 条)：发票相关
  ...
```

### 8.5 MySQL 洞察表

```sql
CREATE TABLE agent_insights (
    id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    report_date  DATE         NOT NULL COMMENT '统计日期',
    metric_type  VARCHAR(32)  NOT NULL COMMENT 'intent_dist / human_rate / avg_turns / blind_spot',
    metric_value JSON         NOT NULL COMMENT '指标数据（JSON）',
    created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_date_type (report_date, metric_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**面试怎么讲**：大部分人做 AI 客服只做到"能对话"就停了。数据飞轮体现的是**系统思维**——不是一次性做完，而是设计了一个自动发现问题、持续改进的闭环。低置信度聚类本质上是把 AI 的"不确定性"转化为运营的"确定性行动"。

---

## 九、面试常见追问

**Q: 上下文过长怎么处理？**
A: 滑动窗口（保留最近 10 轮）+ 早期历史 LLM 压缩为摘要。每次调用控制在 ~2000 Token。

**Q: 为什么 Milvus 不用 ES kNN？**
A: 客服知识库是纯语义召回，不需要全文检索。Milvus 在纯向量场景索引选择更多（IVF/HNSW），且独立部署不影响搜索 ES（解耦）

**Q: 如何避免 Tool Calling 幻觉？**
A: 白名单 + 参数归属校验 + 只读约束 + 轮次限制（最多 3 次）。

**Q: 和搜索共用 LLM 会互相影响吗？**
A: 不会。共享的是 `pkg/ai` 代码包，不共享连接和状态。各服务独立配置模型和连接池。

**Q: 语义缓存的失效策略？**
A: 知识库更新时按 category 清空对应缓存。TTL 1h 兜底。防止旧答案污染系统，附带价值是：缓存热点数据

**Q: 为什么置信度阈值选 0.8/0.5？**
A: 通过测试集标定。0.8 以上的回复人工评估正确率 >95%；0.5 以下正确率 <60%，不如转人工。这两个阈值应该可配置，上线后根据实际数据调整。

**Q: 流式输出和同步 API 怎么共存？**
A: 同一套 Pipeline，流式走 `SendMessageStream`（gRPC Server Streaming），同步走 `SendMessage`（内部收集所有 token 拼接后返回）。前端可灰度切换。

**Q: 熔断器为什么自研？**
A: 需求极简——连续失败计数 + 冷却时间，50 行代码搞定。引入 sentinel-go 是 over-engineering。

**Q: Handoff Summary 生成失败怎么办？**
A: 降级为拼接最近 5 轮对话原文。转人工流程不因摘要失败中断。

---

## 十、测试策略

> AI 系统测试的核心难点：LLM 输出不确定，不能 `assert output == expected`。所以分三层，各用不同策略。

### 三层测试金字塔

**第一层：单元测试（确定性逻辑）**

这些模块输入输出确定，标准 Go test：

| 模块 | 测什么 |
|------|--------|
| `CircuitBreaker` | 状态机转换：连续 5 次失败 → Open → cooldown 后 HalfOpen → 成功 → Closed |
| `Session.RecentMessages` | 20 条消息取最近 10 条 |
| `cleanJSON` / `parseReply` | LLM 输出解析：分隔符格式（`===META===`）、JSON 兜底格式、非 JSON 降级 |
| `mapIntent` | 字符串 → 枚举映射 |
| `quickDetectTransfer` | 关键词匹配：包含"转人工""人工客服"等关键词 → true |
| `applyConfidenceFallback` | ≥0.8 → 直接返回；≥0.5 → 追加参考提示；<0.5 → 追加建议联系人工 |

**第二层：集成测试（Mock LLM + Pipeline 编排）**

用 `MockLLMClient`（根据 prompt 关键词返回预设 JSON）替代真实 LLM，验证 Pipeline 编排逻辑：

| 场景 | 验证点 |
|------|--------|
| 语义缓存命中 | 跳过 Stage 2~4，earlyResp 非空，cacheHit == true |
| 正常四阶段 | 各阶段耗时 > 0，回复非空 |
| 意图识别失败 | 降级 UNKNOWN，Pipeline 继续 |
| 全部 LLM 挂掉 | TemplateEngine 兜底（===META=== 格式），不报错 |
| 转人工（关键词） | `quickDetectTransfer` 命中 → HandoffSummary + persistTransfer |
| 转人工（情绪自动升级） | emotion=angry + 低置信度 → 加速升级 |
| 限频 | 超频请求被拒绝（Lua 原子 INCR+EXPIRE） |

**第三层：Prompt 回归测试（AI 特有）**

修改 Prompt 后可能导致回答质量下降。用 **Golden Test Set** 做模糊评估（不是精确匹配）：

```json
{
  "id": "return_001",
  "input": "买了一条裙子不想要了，怎么退？",
  "expected_intent": "RETURN",
  "expected_keywords": ["订单详情", "申请退货", "7天"],
  "min_confidence": 0.7
}
```

评估标准：意图正确率 > 90%、关键词覆盖率 > 80%、置信度达标率 > 85%。不达标 → 人工审核 Prompt 修改。

**面试怎么讲**：「AI 系统的测试分三层：确定性逻辑用单元测试；Pipeline 编排用 Mock LLM 集成测试；Prompt 质量用 Golden Test Set 回归。重点是第三层——传统系统不需要，但 AI 系统的 Prompt 改一个字都可能影响输出质量，需要独立的回归评估机制。」
