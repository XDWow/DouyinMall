职责清晰：
pkg/ai：HTTP 调用，返回原始响应
infra/ai：熔断、限流、降级、参数累积、统一配置模型参数
usecase/ai：AI 业务能力（RAG、工具调用、参数转换、MCP、置信度/情绪解析）
usecase/chat：会话编排（限流、转人工、持久化、业务决策）

流式调用流程：
Handler.Chat(ChatInput)
↓
ChatUseCase.Execute
├─ validate 校验
├─ runPipeline
│   ├─ 系统限流
│   ├─ 用户限流
│   ├─ 加载 Session
│   ├─ 已转人工守卫 → 直接返回
│   ├─ 关键词转人工 → 直接返回
│   ├─ 语义缓存查询 → 命中直接返回
│   ├─ 意图识别（AIService.RecognizeIntent）
│   ├─ 意图转人工 → 直接返回
│   └─ RAG 检索（AIService.Retrieve）
│
├─ AIService.Generate
│   ├─ buildMessages（拼装 prompt）
│   ├─ 工具调用循环（最多 5 轮）
│   │   ├─ infra/ai.ChatCompletion
│   │   │   ├─ ResilientClient（熔断 + 限流 + 配置参数）
│   │   │   └─ FallbackLLMClient（多节点降级 + 模板兜底）
│   │   ├─ 拿到 tool_calls
│   │   ├─ executeToolCall（参数转换 + MCP 调用）
│   │   └─ 把结果喂回 LLM
│   └─ parseReply（解析置信度、情绪）
│
├─ EnsureMeta（二次评估兜底）
├─ updateConversationState（更新对话状态）
└─ finalize
├─ 加免责声明
├─ 写语义缓存（高置信度 + 无工具调用）
├─ 判断是否自动转人工（低置信度 || angry || urgent）
└─ 持久化消息
