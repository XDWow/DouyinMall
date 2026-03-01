# AI 客服（Agent）微服务设计

> 这是一份面试导向的设计文档。重点不是罗列技术点，而是展示：面对"给电商系统加一个 AI 客服"这个需求，我是怎么思考的。

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

所以 AI 客服是**独立微服务**，不是在搜索服务上加功能。但两者共用 `pkg/ai`（LLMClient、Embedder 接口），因为调 LLM 本质就是 HTTP 请求，用 Go 包复用就够了，不需要 LLM 网关服务。

**项目边界？**

- **Phase 1**：多轮对话 + 意图识别 + RAG 知识问答 + 置信度降级
- **Phase 2**：Tool Calling（LLM 自主调用订单/物流服务获取实时数据）

---

## 二、核心设计：四阶段 Pipeline

核心思路：**把一次对话拆成四个阶段，每个阶段职责单一、可独立降级、可独立监控。**

```
用户消息
  │
  ▼
Stage 1: 会话加载 + 语义缓存检查
  ├─ Redis 加载会话上下文
  ├─ 语义缓存命中？→ 直接返回（<10ms）
  └─ 未命中 → 进入 Stage 2

Stage 2: 意图识别 + Query 改写（单次 LLM 调用）
  ├─ 输入：用户消息 + 最近 3 轮历史
  ├─ 输出：intent + confidence + rewritten_query
  └─ 快速路径：转人工 → 直接返回

Stage 3: RAG 检索（两阶段）
  ├─ rewritten_query → Embedding → Milvus Top-20
  └─ LLM 重排 → Top-3

Stage 4: 回复生成
  ├─ System Prompt + 知识上下文 + 滑动窗口 + 用户输入
  ├─ LLM 生成（reply + confidence）
  ├─ 置信度降级处理
  └─ 异步落库
```

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
  → LLM 压缩为一段摘要（~100 字）
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
| `agent:session:{sid}` | Hash | 24h | 会话元信息（user_id, status, summary） |
| `agent:session:{sid}:msgs` | List | 24h | 最近 20 条消息（滑动窗口） |
| `agent:user:{uid}:active` | String | 24h | 用户当前活跃会话 ID |
| `agent:rate:{uid}` | String | 1min | 消息限频（防刷） |

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

**PipelineDebug**：每次请求返回各阶段耗时，直接体现在 proto 的 `PipelineDebug` message 中：

```
[trace_id=abc123]
├── intent_classify  (12ms, intent=ORDER_INQUIRY, conf=0.92)
├── embedding        (8ms, dim=768)
├── vector_search    (15ms, candidates=20)
├── llm_rerank       (180ms, top3=[k1,k2,k5])
├── llm_generate     (320ms, tokens=156, conf=0.88)
└── total            (535ms, cache_hit=false)
```

**Prometheus 指标**（关键 5 个）：

| 指标 | 类型 | 说明 |
|------|------|------|
| `agent_request_duration_ms` | Histogram(stage) | 各阶段耗时分布，定位瓶颈 |
| `agent_confidence` | Histogram(intent) | 置信度分布，发现低质量回复 |
| `agent_fallback_total` | Counter(level) | 降级次数，判断 LLM 健康度 |
| `agent_cache_hit_total` | Counter | 语义缓存命中率 |
| `agent_llm_tokens_total` | Counter(model) | Token 消耗，控制成本 |

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
├── usecase/         # 核心业务逻辑（ChatUseCase, 四阶段 Pipeline）
├── infra/           # 基础设施（Redis 会话存储、MySQL 持久化、Milvus 检索）
│   ├── cache/       # Redis SessionRepo 实现
│   ├── persistence/ # MySQL 异步落盘
│   └── knowledge/   # Milvus KnowledgeRepo 实现
├── transport/grpc/  # gRPC handler（对应 agent.proto 的 AgentService）
└── ioc/             # 依赖注入（Wire）

pkg/ai/              # 共享 LLM/Embedding 客户端（搜索服务也用）
├── types.go         # LLMClient, Embedder 接口
└── openai_compatible.go  # OpenAI 兼容实现
```

---

## 八、面试常见追问

**Q: 上下文过长怎么处理？**
A: 滑动窗口（保留最近 10 轮）+ 早期历史 LLM 压缩为摘要。每次调用控制在 ~2000 Token。

**Q: 为什么 Milvus 不用 ES kNN？**
A: 客服知识库是纯语义召回，不需要全文检索。Milvus 在纯向量场景索引选择更多（IVF/HNSW），且独立部署不影响搜索 ES。

**Q: 如何避免 Tool Calling 幻觉？**
A: 白名单 + 参数归属校验 + 只读约束 + 轮次限制（最多 3 次）。

**Q: 和搜索共用 LLM 会互相影响吗？**
A: 不会。共享的是 `pkg/ai` 代码包，不共享连接和状态。各服务独立配置模型和连接池。

**Q: 语义缓存的失效策略？**
A: 知识库更新时按 category 清空对应缓存。TTL 1h 兜底。

**Q: 为什么置信度阈值选 0.8/0.5？**
A: 通过测试集标定。0.8 以上的回复人工评估正确率 >95%；0.5 以下正确率 <60%，不如转人工。这两个阈值应该可配置，上线后根据实际数据调整。


