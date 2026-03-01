# AI 搜索设计文档

## 面试口述版本

> 下面是向面试官介绍这个模块时的完整思路，按"整体架构 → 每步实现 → 亮点"展开。

---

### 整体介绍

我在搜索服务里实现了一套 AI 增强的搜索管线，用户输入自然语言（比如"找个便宜点的跑步鞋，最好销量高的"），系统通过五个阶段把它变成商品列表并附带 AI 摘要：

```
自然语言 Query
    ↓
① Query 理解（LLM）       ← 提取关键词、价格意图、类目、排序意图
    ↓
② 多路召回（并行）         ← 关键词召回（ES 倒排）+ 向量召回（ES8 kNN）
    ↓
③ 两阶段排序                ← 相关性门控(RRF) → 业务重排(销量/广告)
    ↓
④ 获取完整商品详情
    ↓
⑤ RAG 摘要（可选）        ← 模糊查询时 LLM 基于搜索结果生成推荐摘要
```

整个管线全部在 search 微服务内实现，没有单独拆 Query 理解服务——因为它和搜索强耦合、没有复用场景，独立部署反而多一跳网络延迟。

---

### 四个亮点

#### 亮点一：LLMClient 与 Embedder 接口分离

很多项目把"对话"和"向量化"塞在同一个接口里，但它们职责完全不同：
- `LLMClient` 用于 Query 理解和 RAG 摘要，调用的是 Chat 模型（Qwen / GPT-4o）
- `Embedder` 用于文本向量化，调用的是 Embedding 模型（bge-large-zh / text-embedding-3）

两者经常是不同的模型、甚至不同的提供商，合在一个接口里会导致替换其中一个时也要动另一个。我把它们拆成两个独立接口：

```go
type LLMClient interface {
    ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}
```

底层的 `ChatClient` 和 `EmbeddingClient` 各自持有独立配置，通过配置文件可以指向不同提供商。Wire 注入时分别提供，面向接口、解耦清晰。

#### 亮点二：两阶段排序的架构演进（Cascade Ranking）

融合排序经历了三个版本的迭代：

**V1 直接加权**：`final = rrf_score + α × sales_count`。问题很快暴露——高销量商品"购买"了相关性，不相关但畅销的商品混入结果。

**V2 log 压缩**：`final = rrf_score × (1 + α × log₂(1 + sales))`。log 避免了头部碾压，但相关性和业务信号仍然耦合在同一个公式里，调参困难。

**V3 两阶段（当前方案）**：把一个复杂问题拆成两个简单问题，彻底解耦——

| 阶段 | 职责 | 输入 → 输出 |
|------|------|-------------|
| 阶段一：相关性门控 | 纯 RRF（意图感知权重），选出候选集 | 200条召回 → ~100条相关候选 |
| 阶段二：业务重排 | 在候选集内按业务信号排序 | 候选集 → 最终排序 |

核心收益是**结果准确性**——不相关的商品无论销量多高都无法通过阶段一的相关性门控，从根本上避免了"销量购买相关性"的问题。附带收益是**解耦可扩展**：
- 阶段一可以独立优化相关性算法（换 cross-encoder、learning-to-rank）
- 阶段二可以独立接入更多业务信号（广告竞价、商家权重、库存优先级），互不影响

意图感知权重在阶段一生效——精确查询侧重关键词(1.2/0.8)，模糊查询侧重向量(0.8/1.2)。阶段二当前用 `rrf × (1 + 0.1 × log₂(1+sales))` 作为业务重排，后续可替换为更复杂的排序模型。

#### 亮点三：管线全链路可观测（PipelineMetrics）

搜索性能问题通常很难定位——到底是 LLM 慢了、还是 ES 慢了、还是向量召回慢了？

我在每个阶段打点计时，最终通过 proto 返回给调用方：

```protobuf
message PipelineMetrics {
    int64 query_understanding_ms = 1;  // LLM Query 理解耗时
    int64 keyword_recall_ms      = 2;  // 关键词召回耗时
    int64 vector_recall_ms       = 3;  // 向量召回耗时
    int64 ranking_ms             = 4;  // 融合排序耗时
    int64 fetch_ms               = 5;  // 获取详情耗时
    int64 rag_ms                 = 6;  // RAG 摘要耗时
    int64 total_ms               = 7;
    int32 keyword_recall_count   = 8;  // 关键词召回数量
    int32 vector_recall_count    = 9;  // 向量召回数量
}
```

服务端同时打 structured log，方便接进 Prometheus / Grafana 做 P99 监控。

#### 亮点四：RAG 的"R"就是搜索管线本身

很多人说 RAG 就是"先向量检索再生成"，但在电商搜索里，搜索管线的结果本身就是最好的检索上下文——不需要单独建向量知识库。

我的做法是：取融合排序 Top-8 商品，把名称、价格、销量、类目整理成结构化文本，同时预计算价格区间、销量最高商品、类目分布作为统计摘要，一起交给 LLM：

```
用户搜索：推荐一款性价比高的蓝牙耳机
意图：对比推荐类

搜索结果：
1. Sony WH-1000XM5 ¥1999.00 销量3200 耳机/蓝牙耳机
2. AirPods Pro ¥1799.00 销量8100 耳机/苹果周边
...
统计：价格 ¥299~¥1999，销量最高「AirPods Pro」(8100件)，涉及类目：耳机、蓝牙耳机
```

给 LLM 预计算好统计信息，减少它自己做数学的幻觉风险，生成的摘要更准确。

---

## 架构

```
User Query → ① Query 理解 (LLM) → ② 多路召回 → ③ 两阶段排序 → ④ 返回商品 → ⑤ RAG 摘要
                                    ├─ Keyword (ES 倒排)
                                    └─ Vector  (ES8 kNN)
```

**不单独拆微服务**：Query 理解与搜索强耦合、无复用场景、多一跳延迟不值得。在 search 服务内部通过 usecase 模式组织。

## 目录结构

```
internal/search/
├── domain/
│   ├── search.go          # 搜索领域模型（商品/商家/聚合/同步事件）
│   ├── ai_search.go       # AI 搜索模型（QueryIntent/RecallResult/PipelineMetrics）
│   ├── repo.go            # 仓储接口（端口，面向 domain 定义）
│   └── error.go           # 领域错误
├── infra/
│   ├── es/
│   │   ├── client.go      # ES8 客户端（支持 kNN）
│   │   ├── init.go        # 索引初始化（含 dense_vector 字段）
│   │   ├── product.go     # 商品仓储实现（ES8 + kNN 向量搜索）
│   │   └── merchant.go    # 商家仓储实现
│   └── llm/
│       ├── types.go        # LLMClient + Embedder 接口定义（分离）
│       └── openai_compatible.go  # ChatClient + EmbeddingClient 实现
├── usecase/
│   ├── search_uc.go       # 普通搜索/商家搜索/建议/聚合
│   ├── ai_search_uc.go    # AI 搜索管线（含 PipelineMetrics 打点）
│   └── sync_uc.go         # 数据同步（写入时生成向量）
├── transport/grpc/
│   ├── handler.go         # gRPC handler
│   └── adapters.go        # Proto ↔ Domain 转换
├── events/                # Kafka 消费者
├── ioc/                   # DI 初始化（InitLLMClient + InitEmbedder 独立）
└── config/                # 配置
```

## 核心流程

### ① Query 理解

通过 LLM 分析用户自然语言查询，输出结构化 `QueryIntent`：

| 字段 | 说明 | 示例 |
|------|------|------|
| `rewritten_query` | 优化后关键词 | "便宜跑步鞋" → "跑步鞋" |
| `categories` | 类目识别 | ["运动鞋"] |
| `min/max_price` | 价格意图（分） | max_price=30000 |
| `sort_by` | 排序意图 | PRICE_ASC |
| `need_rag` | 是否需要 RAG | 模糊/推荐类查询 → true |

降级策略：LLM 调用失败时使用原始 query，`need_rag=true`。

### ② 多路召回（意图感知）

| 通道 | 技术 | Top-K |
|------|------|-------|
| Keyword | ES multi_match (name^3, description) | 100 |
| Vector | ES8 kNN (name_vector, cosine, 1024维) | 100 |

`_source` 只返回 `id` + `sales_count`，减少网络传输。100 条召回结果若带完整字段会产生大量无效 IO，而排序只需这两个字段。完整字段在步骤④中只对最终分页结果精确拉取。

### ③ 两阶段排序（Cascade Ranking）

```
阶段一 · 相关性门控（纯 RRF）：
  精确查询：kw_weight=1.2, vec_weight=0.8
  模糊查询：kw_weight=0.8, vec_weight=1.2
  rrf_score = Σ weight_i / (60 + rank_i)
  → 按 rrf_score 排序，输出候选集

阶段二 · 业务重排（在候选集内）：
  final = rrf_score × (1 + 0.1 × log₂(1 + sales_count))
  → 销量只能在相关结果内竞争，无法"购买"相关性
```

### ④ 获取完整商品详情

步骤②的召回结果只有 `id` 和 `sales_count`，排序后仅对当前分页（如 10 条）发起一次 ES `ids` 精确查询，拉取名称、图片、价格、描述等全量字段。这样整个管线中重字段只传输一次，且数量最小。

### ⑤ RAG 摘要（可选）

`need_rag=true` 时，取 Top-8 商品构造结构化上下文（含预计算统计摘要），LLM 生成 50-150 字推荐摘要。

### 向量写入时机

商品同步时（Kafka 事件 / 全量同步），`SyncUseCase` 自动调用 `Embedder.Embed` 生成 `name_vector` 写入 ES。

## AI 能力接口设计

`LLMClient`（对话）与 `Embedder`（向量化）定义在 `infra/ai` 包下，统一管理 AI 推理能力，两者可分别切换提供商：

```go
type LLMClient interface {
    ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}
```

修改 `config/dev.yaml` 即可切换，无需改代码：

```yaml
llm:                                          # 对话模型
  base_url: "https://api.siliconflow.cn/v1"
  api_key: "your_key"
  chat_model: "Qwen/Qwen2.5-7B-Instruct"
  timeout_seconds: 30

embedding:                                    # 向量模型（可与 LLM 用不同提供商）
  base_url: "https://api.siliconflow.cn/v1"
  api_key: "your_key"
  model: "BAAI/bge-large-zh-v1.5"            # 1024 维中文向量
  timeout_seconds: 30
```

可选提供商：硅基流动（免费）、心流开放平台、DeepSeek、通义千问、Ollama 本地、OpenAI。

## ES8 索引变更

商品索引新增字段：

```json
"name_vector": {
  "type": "dense_vector",
  "dims": 1024,
  "index": true,
  "similarity": "cosine"
},
"sales_count": { "type": "long" }
```

## Proto 变更

新增 RPC：`AISearchProducts(AISearchProductsReq) → AISearchProductsResp`

新增 messages：`QueryIntent`、`PipelineMetrics`

## 配置项

```yaml
llm:
  base_url: ""
  api_key: ""
  chat_model: ""
  timeout_seconds: 30

embedding:
  base_url: ""
  api_key: ""
  model: ""
  timeout_seconds: 30
```
