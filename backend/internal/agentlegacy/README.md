# Agent Legacy

这里存放旧版 AI 客服实现，仅用于历史参考和必要的兼容排查。

约定：
- 旧代码统一放在 `backend/internal/agentlegacy`
- 默认通过 `legacy_agent` build tag 隔离，不参与当前生产构建
- 新版生产实现位于 `backend/internal/agent`

如果需要启动旧链路，请显式使用 `legacy_agent` 构建标签。
