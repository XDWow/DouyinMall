创建文本对话请求

POST https://apis.iflow.cn/v1/chat/completions
Content-Type: application/json
Authorization: Bearer <your api key>（访问心流官网登陆获取API KEY）
请求参数说明
| 参数                         | 类型           | 是否必填 | 默认值                      | 描述                                                                                                          |
| -------------------------- | ------------ | ---- | ------------------------ | ----------------------------------------------------------------------------------------------------------- |
| messages                   | object[]     | 是    | -                        | 构成当前对话的消息列表。                                                                                                |
| messages.content           | string       | 是    | 中国大模型行业2025年将会迎来哪些机遇和挑战？ | 消息的内容。                                                                                                      |
| messages.role              | enum<string> | 是    | user                     | 消息作者的角色。 可选值：user, assistant, system                                                                        |
| model                      | enum<string> | 是    | tstars2.0                | 对应的模型名称。 为更好的提升服务质量，我们将不定期对本服务提供的模型做相关变更，包括但不限于模型上下线、模型服务能力调整，我们会在可行的情况下以公告、消息推送等适当的方式进行通知。 支持的模型请参考快速开始页面。 |
| frequency_penalty          | number       | 否    | 0.5                      | 调整生成 token 的频率惩罚，用于控制重复性。                                                                                   |
| max_tokens                 | integer      | 否    | 512                      | 生成的最大 token 数量。 取值范围：1 < x < 8192                                                                           |
| n                          | integer      | 否    | 1                        | 返回的生成结果数量。                                                                                                  |
| response_format            | object       | 否    | -                        | 指定模型输出格式的对象。                                                                                                |
| response_format.type       | string       | 否    | -                        | 响应格式的类型。                                                                                                    |
| stop                       | string[]     | 否    | null                     | -                                                                                                           |
| stream                     | boolean      | 否    | false                    | 如果设置为 true，token 将作为服务器发送事件（SSE）逐步返回。                                                                       |
| temperature                | number       | 否    | 0.7                      | 控制响应的随机性。值越低，输出越确定；值越高，输出越随机。                                                                               |
| tools                      | object[]     | 否    | -                        | 模型可能调用的工具列表。目前仅支持函数作为工具。使用此参数提供一个函数列表，模型可能会为其生成 JSON 输入。最多支持 128 个函数。                                       |
| tools.function             | object       | 否    | -                        | 函数对象。                                                                                                       |
| tools.function.name        | string       | 否    | -                        | 要调用的函数名称。必须由字母、数字、下划线或短横线组成，最大长度为 64。                                                                       |
| tools.function.description | string       | 否    | -                        | 函数的描述，用于模型选择何时以及如何调用该函数。                                                                                    |
| tools.function.parameters  | object       | 否    | -                        | 函数接受的参数，描述为 JSON Schema 对象。如果不指定参数，则定义了一个空参数列表的函数。                                                          |
| tools.function.strict      | boolean      | 否    | false                    | -                                                                                                           |
| tools.type                 | enum<string> | 否    | function                 | 工具的类型。目前仅支持 function。                                                                                       |
| top_k                      | number       | 否    | 50                       | 限制 token 选择范围为前 k 个候选。                                                                                      |
| top_p                      | number       | 否    | 0.7                      | 核采样参数，用于根据累积概率动态调整每个预测 token 的选择范围。                                                                         |
请求示例（CURL）
curl --request POST \
--url https://apis.iflow.cn/v1/chat/completions \
--header 'Authorization: Bearer <iflow API KEY>' \
--header 'Content-Type: application/json' \
--data '{
"model": "tstars2.0",
"messages": [
{
"role": "user",
"content": "中国大模型行业2025年将会迎来哪些机遇和挑战？"
}
],
"stream": false,
"max_tokens": 512,
"stop": [
"null"
],
"temperature": 0.7,
"top_p": 0.7,
"top_k": 50,
"frequency_penalty": 0.5,
"n": 1,
"response_format": {
"type": "text"
},
"tools": [
{
"type": "function",
"function": {
"description": "<string>",
"name": "<string>",
"parameters": {},
"strict": false
}
}
]
}'
响应参数
非流式输出
| 参数                            | 类型           | 是否必填 | 默认值 | 描述                                                                                  |
| ----------------------------- | ------------ | ---- | --- | ----------------------------------------------------------------------------------- |
| choices                       | object[]     | 是    | -   | 模型生成的选择列表。                                                                          |
| choices.finish_reason         | enum<string> | 否    | -   | 生成结束的原因。 可选值：stop（自然结束）、eos（到达句子结束符）、length（达到最大 token 长度限制）、tool_calls（调用了工具，如函数）。 |
| choices.message               | object       | 是    | -   | 模型返回的消息对象。                                                                          |
| created                       | integer      | 是    | -   | 响应生成的时间戳。                                                                           |
| id                            | string       | 是    | -   | 响应的唯一标识符。                                                                           |
| model                         | string       | 是    | -   | 使用的模型名称。                                                                            |
| object                        | enum<string> | 是    | -   | 响应类型，可选值：chat.completion（表示这是一个聊天完成响应）。                                             |
| tool_calls                    | object[]     | 否    | -   | 模型生成的工具调用，例如函数调用。                                                                   |
| tool_calls.function           | object       | 否    | -   | 模型调用的函数。                                                                            |
| tool_calls.function.arguments | string       | 否    | -   | 函数调用的参数，由模型以 JSON 格式生成。 注意：模型生成的 JSON 可能无效，或者可能会生成不属于函数定义的参数。在调用函数前，请在代码中验证这些参数。    |
| tool_calls.function.name      | string       | 否    | -   | 要调用的函数名称。                                                                           |
| tool_calls.id                 | string       | 否    | -   | 工具调用的唯一标识符。                                                                         |
| tool_calls.type               | enum<string> | 否    | -   | 工具的类型。目前仅支持 function（表示这是一个函数调用）。                                                   |
| usage                         | object       | 是    | -   | Token 使用情况统计。                                                                       |
| usage.completion_tokens       | integer      | 是    | -   | 完成部分使用的 token 数量。                                                                   |
| usage.prompt_tokens           | integer      | 是    | -   | 提示部分使用的 token 数量。                                                                   |
| usage.total_tokens            | integer      | 是    | -   | 总共使用的 token 数量。                                                                     |

流式输出
| 参数                                    | 类型            | 是否必填 | 默认值 | 描述                                                                                 |
| ------------------------------------- | ------------- | ---- | --- | ---------------------------------------------------------------------------------- |
| id                                    | string        | 是    | -   | 聊天补全的唯一标识符。每个分块具有相同的 ID。                                                           |
| choices                               | object[]      | 是    | -   | 模型生成的选择列表。                                                                         |
| choices.finish_reason                 | enum<string>  | 否    | -   | 生成结束的原因，可选值：stop（自然结束）、eos（到达句子结束符）、length（达到最大 token 长度限制）、tool_calls（调用了工具，如函数）。 |
| choices.message                       | object        | 是    | -   | 模型返回的消息对象。                                                                         |
| created                               | integer       | 是    | -   | 响应生成的时间戳（Unix 时间戳，单位为秒）。                                                           |
| model                                 | string        | 是    | -   | 使用的模型名称。                                                                           |
| object                                | enum<string>  | 是    | -   | 响应类型，可选值：chat.completion（表示这是一个聊天完成响应）。                                            |
| tool_calls                            | object[]      | 否    | -   | 模型生成的工具调用，例如函数调用。                                                                  |
| tool_calls.function                   | object        | 否    | -   | 模型调用的函数。                                                                           |
| tool_calls.function.arguments         | string        | 否    | -   | 函数调用的参数，由模型以 JSON 格式生成。注意：模型生成的 JSON 可能无效，或者可能不属于函数定义的参数。                          |
| tool_calls.function.name              | string        | 否    | -   | 要调用的函数名称。                                                                          |
| tool_calls.id                         | string        | 否    | -   | 工具调用的唯一标识符。                                                                        |
| tool_calls.type                       | enum<string>  | 否    | -   | 工具的类型。目前仅支持 function（表示这是一个函数调用）。                                                  |
| usage                                 | object        | 是    | -   | Token 使用情况统计。                                                                      |
| usage.completion_tokens               | integer       | 是    | -   | 完成部分使用的 token 数量。                                                                  |
| usage.prompt_tokens                   | integer       | 是    | -   | 提示部分使用的 token 数量。                                                                  |
| usage.total_tokens                    | integer       | 是    | -   | 总共使用的 token 数量（提示 + 完成）。                                                           |
| delta                                 | object        | 否    | -   | 流式模型响应生成的聊天补全增量。                                                                   |
| choices.logprobs                      | object 或 null | 否    | -   | 该选项的对数概率信息。                                                                        |
| choices.logprobs.content              | array 或 null  | 否    | -   | 包含对数概率信息的消息内容标记列表。                                                                 |
| choices.logprobs.refusal              | array 或 null  | 否    | -   | 包含拒绝消息的标记列表及其对数概率信息。                                                               |
| choices.logprobs.refusal.token        | string        | 否    | -   | 标记。                                                                                |
| choices.logprobs.refusal.logprob      | number        | 否    | -   | 如果该标记在最有可能的 20 个标记内，则为其对数概率；否则使用值 -9999.0 表示极不可能。                                  |
| choices.logprobs.refusal.bytes        | array 或 null  | 否    | -   | 表示标记的 UTF-8 字节表示的整数列表。用于需要组合多个标记字节表示的情况。                                           |
| choices.logprobs.refusal.top_logprobs | array         | 否    | -   | 当前标记位置上最有可能的标记及其对数概率列表。在少数情况下，返回的数量可能少于请求的数量。                                      |
| finish_reason                         | string 或 null | 否    | -   | 模型停止生成标记的原因。                                                                       |
| index                                 | integer       | 否    | -   | 选项在选项列表中的索引。                                                                       |
| service_tier                          | string 或 null | 否    | -   | 用于处理请求的服务层级。                                                                       |
| system_fingerprint                    | string        | 否    | -   | 表示模型运行时后端配置的指纹。可与请求参数 seed 结合使用，以了解可能影响确定性的后端更改。                                   |

流式响应示例
{
"id":"<string>",
"object":"chat.completion.chunk",
"created":1694268190,
"model":"<string>",
"system_fingerprint": "fp_44709d6fcb",
"choices":[
{
"index":0,
"delta":{"role":"assistant","content":""},
"logprobs":null,
"finish_reason":null
}
]
}