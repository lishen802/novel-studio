# P3 Provider 与智能体章节创作纵向切片设计

- 日期：2026-07-29
- 状态：已批准实施
- 范围：P3 第一条真实章节创作闭环

## 1. 目标

本切片让正式桌面产品第一次调用真实模型完成章节规划、正文起草和连续性审查，同时保持作者主权：智能体生成的正文只能形成待审阅 `ChangeSet`，不能直接覆盖正式章节。

完成后的最小闭环为：

```text
配置 Provider 与系统安全凭据
→ 为规划师、正文写手、连续性审查绑定不同连接/模型
→ 选择章节并填写本章目标
→ 持久化运行和三个步骤
→ 装配项目故事上下文
→ 流式执行规划、写作、审查
→ 保存运行产物、Token 用量和错误
→ 生成章节正文候选变更
→ 作者在现有候选变更中心审阅和应用
```

## 2. 本次范围

### 2.1 包含

- 两类真实协议：
  - OpenAI Responses API；
  - Anthropic Messages API。
- 可配置自定义 Base URL，非本机连接必须使用 HTTPS。
- macOS Keychain 与 Windows Credential Manager 保存 API Key。
- 项目数据库只保存 Provider 连接元数据和凭据引用，不保存密钥。
- 三个首批智能体角色：剧情规划师、正文写手、连续性审查。
- 每个角色独立选择 Provider 连接和模型。
- 正文写手与连续性审查必须使用不同的“连接 + 模型”组合。
- 持久化 Run、Step、Artifact、状态、错误和 Token 用量。
- OpenAI/Anthropic SSE 流式响应解析和 Context 取消传播。
- 规划和审查使用 JSON Schema 结构化输出；正文写作使用纯文本输出。
- 应用重启后把遗留的 `running` 运行标为 `recoverable`。
- 失败、取消和可恢复运行可以基于原输入创建新的重试运行。
- 真实输出进入现有章节正文 `ChangeSet`。
- 正式 React 配置、运行和历史界面。

### 2.2 不包含

- 自动模型发现和动态价格目录。
- 工具调用、图片、文件输入或联网搜索。
- 十一种完整智能体角色和可视化工作流编辑器。
- 多路并行盲审与审稿仲裁。
- 自动事实提取和结构化故事资产更新。
- 小说导入重建。
- 云端账号、许可证、团队协作。

这些能力继续属于 P3 后续切片、P4 和 P5。

## 3. 架构边界

```text
React AgentWorkbench
  ↓ StudioClient
Wails DesktopAPI（薄转发）
  ↓
workspace.Service
  ├── Provider/Agent 配置用例
  ├── Agent Run 状态机
  ├── Context Assembler
  └── ChangeSet 创建
  ↓ ports
  ├── AgentStore
  ├── CredentialStore
  └── ModelProviderFactory
  ↓ adapters
  ├── SQLite AgentStore
  ├── OpenAI Responses
  ├── Anthropic Messages
  ├── macOS Keychain
  └── Windows Credential Manager
```

领域模型不依赖 HTTP、SQLite、Wails 或平台凭据 API。Provider Adapter 不读取项目数据库。React 不接触密钥存储或模型 HTTP。

## 4. 数据模型

### 4.1 ProviderConnection

- `id`
- `project_id`
- `name`
- `kind`: `openai_responses` / `anthropic_messages`
- `base_url`
- `credential_ref`
- `created_at`
- `updated_at`

API Key 不出现在记录、视图、日志、快照元数据或错误中。界面只能看到 `has_credential`。

### 4.2 AgentProfile

- `id`
- `project_id`
- `role`: `planner` / `writer` / `continuity_reviewer`
- `provider_connection_id`
- `model`
- `instructions`
- `max_output_tokens`
- `enabled`
- `created_at`
- `updated_at`

每个项目每种角色至多一个活动配置。

### 4.3 AgentRun

- `id`
- `project_id`
- `chapter_id`
- `goal`
- `base_revision`
- `status`
- `change_set_id`
- `retry_of_run_id`
- `error_kind`
- `error_message`
- `created_at`
- `started_at`
- `finished_at`
- `updated_at`

状态为 `queued`、`running`、`completed`、`failed`、`cancelled`、`recoverable`。

### 4.4 AgentRunStep

每次运行固定建立规划、写作和连续性审查三个步骤。步骤保存配置快照，因此后续修改角色设置不会改变历史：

- `profile_id`
- `provider_connection_id`
- `model`
- `role`
- `sequence`
- `status`
- `output_preview`
- `input_tokens`
- `output_tokens`
- `error_kind`
- `error_message`
- 时间字段

### 4.5 AgentArtifact

完整产物独立保存：

- `context_manifest`
- `plan`
- `draft`
- `continuity_review`

正文草稿产物不是正式正文。只有运行成功后创建的 `ChangeSet` 经作者接受才会写入章节。

## 5. Provider 契约

统一请求包含：

- 模型；
- system instructions；
- user input；
- 最大输出 Token；
- 可选 JSON Schema；
- 流式增量回调。

统一响应包含：

- 完整文本；
- input/output token；
- Provider request ID；
- stop reason。

统一错误分类：

- `authentication`
- `permission`
- `rate_limit`
- `quota`
- `context_too_long`
- `content_safety`
- `invalid_response`
- `network`
- `timeout`
- `cancelled`
- `internal`

HTTP 错误正文不得原样穿透 UI。只保存稳定分类、安全摘要、状态码和是否可重试。

OpenAI 使用 `POST /responses`、Bearer 凭据、`text.format.json_schema` 和 `response.output_text.delta`。

Anthropic 使用 `POST /messages`、`x-api-key`、`anthropic-version: 2023-06-01`、`output_config.format.json_schema` 和 `content_block_delta`。

未知 SSE 事件必须安全忽略，以兼容 Provider 增加新事件类型。

## 6. 上下文装配

Context Manifest 记录以下输入及内容摘要：

- 项目标题、类型和简介；
- 目标章节标题、当前正文、revision；
- 与章节关联的大纲节点和场景卡；
- 角色、势力、归属和人物关系；
- 区域、地点、世界规则和未完成伏笔；
- 时间线事件；
- 已确认事实、证据和角色知识边界；
- 作者填写的本章目标；
- 每类数据的数量和 SHA-256 内容哈希。

第一版使用确定性 JSON 装配，不引入向量数据库。只把已确认事实作为硬约束，候选事实仅作为带状态的参考，不得伪装成确认事实。

## 7. 运行流程

1. `CreateChapterDraftRun` 校验章节、三个角色配置、凭据引用和异构审查规则。
2. 在任何外部请求前，以一个事务写入 Run 和三个 pending Step。
3. `ExecuteAgentRun` 把 Run 标为 running，并注册可取消 Context。
4. 装配并保存 Context Manifest。
5. 规划师输出结构化章节计划。
6. 写手接收 Context Manifest 与计划，流式输出完整章节候选。
7. 连续性审查接收 Context Manifest、计划和草稿，输出结构化审查结论。
8. 保存全部 Artifact 和 Token 用量。
9. 若步骤全部成功，基于创建运行时捕获的章节 revision 创建正文 ChangeSet。
10. 若作者在运行期间修改了章节，ChangeSet 仍使用旧 revision 基线，应用时由现有指纹/revision 机制阻止覆盖。
11. Run 标为 completed，并关联 ChangeSet。

审查不通过不会自动丢弃草稿；审查结论和问题会进入 ChangeSet 摘要与修改理由，交给作者决定。

## 8. 取消、失败和恢复

- UI 可以取消当前运行；Context 取消传播到 HTTP 请求。
- 已完成步骤和产物保留，运行标为 cancelled。
- 网络、鉴权、限流、结构化输出或数据库错误使运行标为 failed。
- 启动打开项目时，把遗留 running 状态改为 recoverable。
- 重试创建新的 Run，并使用原运行输入和当前角色配置；旧运行保持不可变审计记录。
- 分支切换、项目快照恢复和关闭项目时不得关闭仍被运行使用的数据库。第一版在存在 running 运行时阻止这些版本操作，作者需要先取消并等待状态落盘。

## 9. UI

新增“智能体创作”工作区：

- Provider 连接：新建、更新、删除、凭据状态和连接测试。
- 智能体配置：三个角色的连接、模型、说明和最大输出。
- 章节创作：选择章节、填写本章目标、启动/取消。
- 运行历史：状态、步骤、模型、Token、错误、产物摘要和关联候选变更。

API Key 输入框保存后立即清空，任何查询和错误都不会把密钥返回前端。

## 10. 安全与隐私

- API Key 只存在于系统凭据仓库和单次请求内存。
- 项目包、数据库、分支、快照、日志和测试夹具不含真实密钥。
- Base URL 仅允许 HTTPS；`localhost`、`127.0.0.1` 和 `::1` 可使用 HTTP。
- 发送前 UI 明确显示 Provider、模型和包含的数据域。
- Provider 响应不会直接执行命令、路径或工具调用。
- 测试只使用 `httptest` 和假凭据仓库，不访问付费 API。

## 11. 验收标准

1. 同一项目可以配置 OpenAI Responses 和 Anthropic Messages 两类连接。
2. API Key 不出现在项目数据库、前端返回值或日志。
3. 三个角色可绑定不同连接和模型，写手与审查不能使用相同组合。
4. 运行记录在第一次外部调用前持久化。
5. 规划与审查输出通过 JSON Schema，正文通过 SSE 增量读取。
6. 取消、Provider 错误和无效响应不会修改正式正文。
7. 成功运行只创建待审阅正文 ChangeSet。
8. 作者修改章节后，旧运行生成的 ChangeSet 不能覆盖新 revision。
9. 重启后遗留运行显示为可恢复并可重试。
10. Go race tests、前端测试、macOS build 和 Windows/amd64 build 通过。
