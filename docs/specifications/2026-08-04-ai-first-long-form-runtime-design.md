# AI 主创作与百万字长篇运行时技术方案

- 日期：2026-08-04
- 状态：已确认，待实施计划
- 产品规格：`docs/superpowers/specs/2026-08-04-ai-first-long-form-authoring-design.md`
- 交互规格：`docs/superpowers/specs/2026-08-04-ai-first-authoring-interaction-design.md`
- 技术基线：Go 1.25、Wails v2.13.0、React、TypeScript、Vite、SQLite

## 1. 技术结论

本次改造采用“保留可靠底座、增加长篇创作运行时、重排产品主流程”的渐进式方案，不新建第二套产品、不替换 SQLite、不引入 Rust，也不把核心能力拆成云服务。

现有能力继续复用：

- `.zjproject` 本地项目包、SQLite 迁移和完整性保护；
- 章节 revision、恢复草稿、ChangeSet、快照和剧情分支；
- 大纲、人物、世界、时间线、事实、知识边界和伏笔领域模型；
- OpenAI Responses、Anthropic Messages、系统凭据仓库；
- 持久化运行、取消、重试、恢复、Token 预算和上下文裁剪；
- TXT/Markdown 归档、编码识别、卷章切分和原文行证据。

新增能力分为六个边界：

1. 应用级模型设置；
2. API 与 Cursor/Codex 订阅双通道模型运行时；
3. 作品方案与全书章节路线图；
4. 有界批量创作与自动审阅；
5. 带证据的长篇记忆 harness；
6. 导入小说结构化重建与续写接入。

未来 AI 作者市场只依赖稳定的 `AuthorProfile` / `AuthorTeam` 模板协议，不进入当前在线实现范围。

## 2. 当前实现差距

### 2.1 单章运行绑定过紧

当前 `AgentRun` 强制关联一个已存在章节和 revision，步骤固定为规划、写作和连续性审阅。它适合保留为模型调用审计基础，但不能直接表达：

- 新作品的三套方向；
- 完整《作品方案》；
- 全书章节路线图；
- 一次 1–50 章的批次；
- 批次级反馈和影响分析；
- 导入小说的多章抽取与合并。

因此不继续扩大单章 `AgentRun` 的职责。新增高层 `CreativeRun` 和 `AuthoringBatch` 编排模型，底层模型请求仍复用 Provider、凭据、Artifact、Token 计量和取消机制。

### 2.2 上下文只有裁剪，没有长篇检索

当前 `context-v2` 能按域和优先级确定性裁剪，但其输入仍偏向一次性列出当前故事资产。百万字项目不能靠把全部章节、事实和实体排列后从尾部删除。

新运行时需要：

- 分层摘要；
- 实体、事件、伏笔和章节之间的显式关联；
- 近期上下文窗口；
- 基于当前章节目标的结构化检索；
- SQLite FTS5 的全文召回；
- 每条记忆的来源证据和版本；
- 上下文清单的可解释选择与遗漏原因。

### 2.3 Provider 配置位置错误

当前 Provider 连接与角色配置保存在每个作品数据库中。新产品要求它们属于应用设置，并供多部作品复用。项目只保存“选用哪个创作方案”及运行时冻结快照，不保存 API Key。

### 2.4 运行时只有 API Provider

当前 P3 只支持 OpenAI Responses 与 Anthropic Messages API。产品还需要让作者使用自己的 Cursor 订阅和 ChatGPT/Codex 订阅，但不能把“订阅登录”伪装成普通 API Key，也不能复制第三方工具的 OAuth Client ID、私有端点或未公开协议。

API 与订阅运行时必须共享上层创作协议，同时保留不同的认证、模型目录、额度来源、能力限制、错误分类和生命周期。

### 2.5 导入只完成确定性阶段

现有 P4 能安全保存和应用章节，但尚无分章模型抽取、跨章合并、冲突保留、证据确认和续写准备。

## 3. 目标架构

```text
React
├── 应用级模型设置
├── AI 创作驾驶舱
├── 作品方案审阅
├── 章节批次与定稿队列
├── 长篇记忆/影响分析
└── 导入分析中心
        ↓ StudioClient
Wails DesktopAPI（薄转发、文件/链接选择、事件桥）
        ↓
Go Application
├── appsettings.Service
├── authoring.Service
├── memory.Service
├── importing.Service
└── workspace.Service（项目生命周期与兼容门面）
        ↓ ports
Infrastructure
├── App SQLite（全局模型与作者模板）
├── Project SQLite（正文、候选、记忆、运行）
├── Model Runtime
│   ├── API Provider adapters
│   ├── Codex SubscriptionRuntime
│   ├── Cursor SubscriptionRuntime
│   └── Local Model adapters
├── Keychain / Credential Manager
├── official runtime process supervisor
└── projectfs / import archive / snapshot / branch
```

`workspace.Service` 继续管理活动项目、关闭、分支和快照互斥，但新的创作逻辑放进独立、可测试的应用服务。Wails 只转发用例，React 只依赖 `StudioClient`。

## 4. 应用级模型设置

### 4.1 App 数据库

在系统应用数据目录建立独立 `settings.db`：

- macOS：用户 Application Support 下的织境目录；
- Windows：用户 LocalAppData 下的织境目录。

项目包不包含此数据库。

主要表：

```text
model_connections
model_connection_accounts
model_catalog_entries
creative_profiles
creative_profile_roles
installed_author_profiles
installed_author_teams
legacy_connection_imports
app_schema_migrations
```

`model_connections` 根据连接类型保存供应商、Base URL、显示名称、credential reference，或官方运行时类型、受支持版本和可执行文件路径指纹。API Key 继续只存在系统凭据仓库，订阅 OAuth 凭据继续只由官方运行时管理。

连接按来源分类：

```text
api                  官方 API、兼容 API 或网关
subscription_runtime 用户已登录的 Cursor/Codex 官方本地运行时
local_runtime        Ollama、LM Studio 等本地模型服务
```

`model_connection_accounts` 只保存非敏感账号标识、登录状态摘要和最近验证时间。OAuth access token、refresh token、Cookie 和官方运行时本地认证文件均不得复制到 `settings.db`。

`model_catalog_entries` 是带来源、运行时版本和 `verified_at` 的缓存，不是永久权益声明。过期目录必须重新发现或验证后才能用于新运行。

### 4.2 创作方案

`CreativeProfile` 表示一套可复用模型组合：

```text
planner
writer
continuity_reviewer
character_reviewer
causality_reviewer
pace_reviewer
memory_extractor
impact_analyst
```

普通作者只看到推荐方案名称。高级模式才显示角色、模型、温度和输出上限。

每次创建运行时，把所选方案完整快照保存进项目运行记录，避免全局设置变化改写历史。

角色不直接引用供应商字符串，而是引用稳定的 `ModelTarget`：

```text
connection_id
model_id
source_kind
provider_family
capability_snapshot
optional fallback targets
```

`provider_family` 表示底层模型家族，`source_kind` 表示额度来源。例如 Claude 经 Cursor 订阅运行时使用时，两者分别是 `anthropic` 与 `subscription_runtime`，不能显示成 Anthropic API。

### 4.3 统一模型运行端口

新增 `internal/application/modelruntime` 端口：

```text
ModelRuntime
├── ValidateConnection
├── DiscoverModels
├── Capabilities
├── StartStream
├── Cancel
└── UsageSnapshot
```

所有 API、订阅和本地模型都通过同一请求信封进入现有 Artifact、Token 计量、取消和持久化运行机制。请求信封不暴露具体 SDK 类型，至少包含：

- 运行、步骤和角色标识；
- 模型目标和冻结的能力快照；
- system/user 内容与结构化输出 Schema；
- 上下文、输出和时间预算；
- 幂等键、取消信号和重试策略。

`Capabilities` 至少声明：

- 是否支持流式输出；
- 是否支持 JSON Schema 或只能做提示式 JSON；
- 是否能精确报告输入/输出用量；
- 是否支持模型目录发现；
- 上下文和最大输出限制的可信来源；
- 是否支持图片、工具调用、远端压缩和会话恢复；
- 额度来源、实验状态和当前连接健康度。

运行时不能因为多个来源最终调用同一模型家族，就假设它们具有相同协议和能力。

### 4.4 API 连接

OpenAI、Anthropic 和后续兼容 Provider 继续作为主要生产通道：

- 用户录入 API Key 时只写入 Keychain/Credential Manager；
- App 数据库保存 credential reference；
- Base URL、自定义 Header 和模型目录属于应用设置；
- API 连接可以提供精确 Token 计量、结构化输出和可预测的重试语义；
- 长批次默认推荐 API 连接，订阅连接可以由作者主动选用或加入回退链。

已有 P3 OpenAI/Anthropic Provider 适配到 `ModelRuntime`，不重写其流式协议与错误分类。

### 4.5 Codex 订阅运行时

Codex 第一版使用用户安装并已登录的官方 Codex 本地运行时：

1. 设置页检测受支持的 Codex 可执行文件与版本；
2. 用户在官方登录流程中选择 ChatGPT/Codex 订阅账号；
3. Go 进程通过官方 Codex App Server 的版本化 JSON-RPC 接口创建会话；官方 SDK 只作为协议验证和后续替代方案；
4. 只使用官方运行时返回或官方文档声明的可用模型；
5. 小说软件不读取、解析、导入或同步 Codex 本地 OAuth 凭据；
6. 登录失效时返回 `login_required`，由用户重新完成官方登录。

Codex 订阅只代表该账号当前拥有的 OpenAI/Codex 权益，不能被描述为 Claude、Gemini 等其他供应商的授权来源。

Codex 是面向编程的运行时。适配器必须在隔离的空工作目录中运行，关闭不必要的文件与 Shell 工具，只发送本次由 `longform-v1` 组装的有界上下文。若官方运行时无法提供所需的纯生成、结构化输出或安全隔离能力，该模型目标不得被标记为“推荐用于连续创作”。

### 4.6 Cursor 订阅运行时

Cursor 第一版优先使用用户安装并已登录的官方 Cursor CLI Headless 模式；官方 SDK 只有在其分发、版本和双平台运行成本通过独立验证后才作为替代实现。

```text
Go CursorSubscriptionRuntime
→ 启动受支持的 cursor-agent
→ 通过 stdin/stream-json 发送有界任务
→ 解析流事件、用量、终态和错误
→ 转换为统一 ModelRuntime 事件
```

约束：

- 使用用户自己的 Cursor 账号、订阅或官方 User API Key；
- 只展示官方运行时向该账号开放或验证成功的模型；
- Cursor 中的 GPT、Claude、Gemini 等模型显示为“来自 Cursor 订阅”；
- 不调用 `api2.cursor.sh` 等内部端点；
- 不复制 Cursor OAuth Client ID、protobuf 或客户端版本伪装逻辑；
- 不把一个账号的订阅额度提供给其他用户；
- 账号额度、模型上下架和限流由 Cursor 决定，运行时必须按动态能力处理。

如果官方 CLI 不能枚举模型，`DiscoverModels` 返回 `unsupported`。UI 可以展示经过版本验证的候选目录，但必须逐个执行轻量能力测试后才允许加入创作方案，不能把静态目录当成账号真实权益。

Cursor 同样是编程 Agent。进程在应用私有的隔离临时工作目录运行；织境不向它传入 `.zjproject` 路径、无关 Provider 的 API Key 或作者未选入上下文的正文。官方运行时可以自行访问其正常登录所必需的认证存储，但织境不读取或转发其中内容。进程退出、取消和子进程清理由 supervisor 负责。

### 4.7 模型选择与回退

每个创作角色拥有一个主 `ModelTarget` 和可选回退链。回退规则：

1. 只在完整步骤边界切换，不在同一次模型流中途拼接两个模型输出；
2. 登录失效默认暂停并等待作者处理；
3. 临时网络错误、429、额度耗尽或模型下架可以按作者预先启用的链切换；
4. 从订阅切到付费 API 前必须有明确授权，不能产生静默费用；
5. 回退后重新从冻结输入执行当前步骤，旧的部分输出保留为 Artifact 但不进入候选稿；
6. 实际连接、模型、来源、用量可信度和回退原因写入运行记录。

异构审阅规则继续按底层提供方与模型家族判断。写作模型经 Cursor 调用 Claude 时，审阅模型不能因为连接 ID 不同就把另一个 Cursor Claude 错判为异构。

### 4.8 订阅运行时安全边界

- 不读取 `~/.codex`、`~/.cursor` 或第三方工具数据库中的原始令牌；
- 不把第三方 OAuth 凭据转存到系统凭据仓库；
- 不复用不属于织境的 OAuth Client ID 实现自有登录页；
- 不记录 CLI 环境、请求 Header、Token、Cookie 或认证回调正文；
- 只允许配置并校验过的官方可执行文件路径；
- 子进程使用清理后的环境变量、隔离工作目录、有界 stdin/stdout 和超时；保留官方运行时完成自身登录所必需的环境，移除无关 Provider API Key；
- 发送模型前继续展示数据范围和连接来源，遵循作者的外发授权；
- 订阅连接默认是当前电脑、当前系统用户私有设置，不随项目包导出；
- 第三方条款或协议发生变化时，由本地兼容门禁或软件更新禁用不兼容的适配器版本，不引入云端必需开关，也不得影响 API 通道和本地项目数据。

`oh-my-pi` 不作为生产必需依赖。未来若增加外部 Agent RPC 兼容层，只能作为用户自行安装和登录的可选连接器，并复用本节安全边界；不得借其绕过官方运行时限制。

### 4.9 兼容迁移

旧项目中的 Provider 连接和三个角色配置不删除。首次打开旧项目时：

1. 读取旧配置；
2. 按供应商、Base URL、名称和 credential reference 去重导入 App 数据库；
3. 创建一套兼容 `CreativeProfile`；
4. 在项目中保存新全局方案引用；
5. 记录 `legacy_connection_imports`，保证幂等；
6. 旧表保持只读，便于旧版本打开和故障回滚。

## 5. 作品方案与章节路线图

### 5.1 领域对象

新增 `internal/domain/authoring`：

```text
CreativeRun
StoryDirection
WorkProposal
WorkProposalRevision
WorkProposalSection
ChapterRoadmap
ChapterRoadmapItem
AuthorDecision
```

`CreativeRun` 是高层运行；它可以关联项目、作品方案、批次或导入任务，不强制关联章节。

`WorkProposal` 状态：

```text
generating → awaiting_author → accepted
                           ↘ rejected
                           ↘ superseded
```

三套 `StoryDirection` 属于同一次运行。作者选定后，AI 生成整合《作品方案》。作品方案分区内容使用稳定 JSON Schema 输出，并保存原始 Artifact、验证后的结构和作者修改版本。

### 5.2 正式应用

作者接受《作品方案》时，在一个事务中：

1. 固化作品方案 revision；
2. 通过现有 ChangeSet 协议创建或更新大纲、人物、世界、关系和事实；
3. 创建 `ChapterRoadmap`；
4. 生成应用前恢复点；
5. 写入作者决策记录。

任何失败都不得产生半套正式故事资产。

## 6. 批量创作运行时

### 6.1 批次模型

新增：

```text
AuthoringBatch
BatchChapter
BatchChapterRevision
BatchDependency
BatchReview
ChangeRequest
ImpactPlan
ImpactPlanItem
```

`AuthoringBatch` 包含：

- 起始 roadmap item；
- 章节数量，范围 1–50；
- 作者自然语言指令；
- 创作方案快照；
- canonical head hash；
- Token 和上下文预算；
- 状态与进度；
- 创建、开始、暂停、完成和关闭时间。

批次状态：

```text
queued
running
paused
recoverable
awaiting_review
revising
closed
cancelled
failed
```

同一分支同时只允许一个非终态批次。当前批次未关闭时禁止启动下一批。

### 6.2 顺序执行

批次内章节按顺序运行，不并行生成正文：

```text
assemble context
→ chapter planner
→ writer
→ independent reviewers
→ revision editor
→ validate
→ persist candidate + temporary state delta
→ next chapter
```

后续候选可读取同批前序候选的临时状态，但这些状态带 `provisional` 标记，不能进入正式长期记忆。

每完成一个步骤立即持久化。应用崩溃后，启动恢复器把遗留 `running` 标为 `recoverable`；已完成且校验通过的步骤不重复调用。

### 6.3 自动审阅

第一版自动审阅至少覆盖：

- 连续性；
- 人物行为与知识边界；
- 因果；
- 节奏；
- 世界规则和伏笔约束。

审阅模型不读取其他审阅者的结论；修订编辑才汇总。所有问题包含类别、严重度、证据、建议和处理结果。

### 6.4 定稿

作者可以逐章定稿，但批次只有全部章节被定稿、替换或明确废弃后才能关闭。

定稿事务：

1. 校验 candidate 基线和依赖未失效；
2. 将候选正文应用为正式章节 revision；
3. 固化自动审阅摘要；
4. 写入章节摘要和带证据状态增量；
5. 更新 canonical head；
6. 标记后续候选是否需要重新验证；
7. 生成版本记录。

## 7. 修改意见与影响分析

作者可以针对一章、选择的多章或整批提交 `ChangeRequest`。AI 只能先生成 `ImpactPlan`，不能直接重写。

`ImpactPlan` 包含：

- 作者原始意见；
- 受影响候选章节；
- 不受影响候选章节；
- 是否与已定稿事实冲突；
- 拟修改的章节目标、人物状态、伏笔或正文；
- 预计 Token；
- 生成时使用的 canonical head。

作者确认后，运行时只为受影响候选创建新 revision。旧候选保留，`BatchDependency` 指向新版本。

如果 canonical head 在确认前改变，影响方案失效，必须重新分析。

已定稿历史默认不进入修改范围。若意见与历史矛盾，系统提示冲突并将调整集中到后续未定稿章节；只有作者明确选择“创建历史修订候选”时才使用现有 ChangeSet 流程处理旧正文。

## 8. 百万字长篇记忆 harness

### 8.1 不变量

1. 原始定稿正文是最高层事实来源，永不被摘要替代。
2. 作者锁定的作品方案、事实和规则是硬约束。
3. AI 派生记忆必须带来源章节、revision 和证据。
4. 摘要可以重建；原文和作者决策不能丢失。
5. 候选记忆与正式记忆隔离。
6. 检索结果、裁剪和遗漏必须可解释。

### 8.2 分层记忆

```text
L0 作品硬约束
   已确认作品方案、作者锁定规则、目标章节路线图

L1 当前状态
   人物位置/伤势/知识、关系、物品持有、世界状态、未解冲突

L2 结构记忆
   当前卷/故事弧摘要、主线与支线进度、伏笔状态

L3 情节记忆
   每章摘要、关键事件、事实和证据

L4 原文
   完整章节正文、revision、精确证据位置
```

L0–L2 是常用压缩层；L3 用于检索；L4 是最终追溯来源。

### 8.3 新增持久化读模型

复用现有 facts、events、character_knowledge、hooks、relationships 和 story structure，新增：

```text
chapter_summaries
chapter_state_deltas
memory_evidence
arc_state_snapshots
memory_snapshots
memory_snapshot_items
batch_provisional_state
chapter_search_fts
```

`chapter_state_deltas` 表示定稿章节导致的状态变化，不直接覆盖人物档案。例如“第 86 章结束时角色 A 身处北境、左臂受伤”是带证据的当前状态，而不是改写角色的基础简介。

### 8.4 记忆提取

章节定稿后，`memory_extractor` 输出结构化增量：

- 章节摘要；
- 关键事件；
- 新增/失效事实；
- 人物、关系和物品状态变化；
- 伏笔埋设、推进、回收；
- 后续必须承接项；
- 每项证据位置。

验证器检查：

- 证据章节和 revision 是否存在；
- 引文/偏移是否匹配定稿正文；
- 引用实体是否存在；
- 状态转移是否合法；
- 与作者锁定事实是否冲突。

通过验证的派生记忆自动生效；失败项进入记忆审计队列，不阻塞正文定稿。作者可以修正或锁定派生记忆。

### 8.5 上下文检索

新 `longform-v1` Context Manifest 按以下顺序装配：

1. 必选：项目、作品硬约束、目标 roadmap、当前章计划、作者指令；
2. 必选：所有直接相关的已确认事实、人物当前状态和未回收伏笔；
3. 高优先：当前卷/故事弧快照；
4. 高优先：最近若干章正文或摘要；
5. 中优先：按实体、事件、伏笔关系检索的早期记忆；
6. 中优先：FTS5 召回的相关章节摘要与证据；
7. 低优先：其他背景资产；
8. 临时域：同批前序候选及 provisional state。

检索不依赖向量数据库才能工作。第一版使用显式实体关联、时间线、伏笔链接和 FTS5；向量检索作为测量后增强。

每项记录：

- 来源 ID、revision、哈希；
- 选择原因；
- 重要级别；
- 是否正式或临时；
- 字符/Token 占用；
- 被遗漏原因。

### 8.6 压缩与重建

每章保存不可变摘要；每个卷或故事弧结束时生成聚合快照。聚合快照引用组成它的章节摘要，不删除原摘要。

当提示版本、提取 Schema 或模型变化时，可从定稿正文重新生成派生记忆并对比差异。重建在旁路完成，验证通过后原子切换 active snapshot。

## 9. 导入重建

在现有 `import_jobs` 和 `import_chapters` 上新增：

```text
import_runs
import_run_steps
import_candidates
import_candidate_evidence
import_candidate_links
import_merge_decisions
```

每个章节独立抽取，立即持久化，避免整本发送：

```text
chapter extraction
→ local candidates
→ cross-chapter entity resolution
→ conflict-preserving merge
→ candidate WorkProposal
→ author evidence review
→ apply confirmed assets
→ build memory snapshot
→ continue writing
```

候选固定状态为 `inferred`、`confirmed`、`conflicted`、`rejected`。没有证据的内容不能成为正式事实。

## 10. API 与事件

Wails API 只暴露用例：

```text
AppModelSettings / SaveModelConnection / RemoveModelConnection
DetectSubscriptionRuntime / ConnectSubscriptionRuntime / DisconnectSubscriptionRuntime
TestModelConnection / DiscoverConnectionModels / ConnectionCapabilities
CreateStoryDirectionRun / StoryDirections
CreateWorkProposal / UpdateWorkProposal / AcceptWorkProposal
ChapterRoadmap
CreateAuthoringBatch / Start / Pause / Resume / Cancel
Batch / BatchChapters / FinalizeBatchChapter / CloseBatch
CreateChangeRequest / AnalyzeImpact / ConfirmImpactPlan
RebuildMemory / MemoryAudit
CreateImportExtractionRun / ConfirmImportCandidates
```

长任务通过带 `runId`、`batchId`、`sequence` 的事件更新前端。查询始终可以从 SQLite 重建当前 UI，不依赖只存在于内存的事件。

## 11. 错误、恢复与并发

- Provider、结构化输出、预算、取消、持久化和冲突错误继续分类。
- 订阅运行时新增 `runtime_missing`、`runtime_version_unsupported`、`login_required`、`account_unavailable`、`model_unavailable`、`quota_exhausted`、`protocol_error` 和 `child_process_failed`。
- 模型响应失败只使当前步骤失败，不删除已完成 Artifact。
- 订阅运行时异常退出时 supervisor 必须回收子进程、保存最后已确认事件并把当前步骤标为可重试；不得自动重新提交可能已经产生费用的未知请求。
- 模型目录或账号权益变化不得重写历史运行；只使尚未开始的新步骤重新验证其 `ModelTarget`。
- 登录失效不得自动降级到 API；只有作者预先批准的回退链可以切换额度来源。
- 批次运行期间阻止分支切换、快照恢复、导入应用和项目关闭数据库替换；关闭项目先取消并等待运行释放读锁。
- 作者编辑章节导致基线变化时，候选应用使用 revision 和 canonical head 双重校验。
- 磁盘不足时停止新模型调用并保留可恢复状态。
- 任何安全错误摘要不得包含正文、Prompt、模型响应、SQL 或 API Key。

## 12. 测试与可证明性

### 12.1 领域和应用测试

- 状态机、批次 1–50 边界和单活动批次；
- 批次顺序执行、取消、恢复、独立重试；
- API、订阅和本地模型使用相同运行信封与事件合同；
- 回退只发生在步骤边界，且订阅切 API 需要显式授权；
- 实际模型来源与底层模型家族参与异构性判断；
- 候选与正式事实隔离；
- 影响方案在 canonical head 变化后失效；
- 定稿事务与记忆更新原子性；
- 模型失败不修改正式正文。

### 12.2 百万字记忆金标

建立无版权风险的合成长篇夹具，至少覆盖：

- 500 章、约 200 万中文字符；
- 第 1 章承诺在第 500 章目标中被正确检索；
- 早期人物知识边界在后期不被错误突破；
- 已回收伏笔不再进入未回收列表；
- 修改第 93 章只使真正依赖它的候选失效；
- 相同项目状态生成字节稳定的 Context Manifest；
- 必选记忆超过预算时在 Provider 调用前失败；
- 重建派生记忆不改变正文和作者决策。

测试使用 fake Provider、固定 IDs/clock 和合成文本，不访问真实模型 API。

订阅运行时使用伪 Codex/Cursor 可执行文件与固定 JSON-RPC/stream-json transcript，覆盖：

- 运行时发现、版本不兼容和登录失效；
- 模型目录发现支持与不支持两种情况；
- 流式输出、取消、超时、子进程崩溃和残留进程清理；
- 429、额度耗尽、模型下架和回退；
- stdout/stderr 与日志中的凭据、Header 和正文脱敏；
- 织境不传入项目包路径、无关 Provider API Key 或原始认证内容，官方运行时仍能使用自己的登录状态。

### 12.3 性能

以 500 章、200 万字符、2,000 条记忆记录为基线：

- 目标章节上下文装配：目标 500 ms 内；
- 相关记忆首批查询：目标 200 ms 内；
- 打开待审阅批次：目标 500 ms 内；
- 内存不加载全书正文；
- UI 只接收有界预览，正文按章加载。

### 12.4 平台

Windows/macOS 分别验证：

- App 数据库位置和升级；
- Keychain/Credential Manager；
- 官方 Codex/Cursor 运行时检测、登录状态、路径含空格与非 ASCII 字符；
- 订阅子进程启动、取消、应用退出清理和运行时升级后的协议兼容；
- 长任务休眠、网络切换和应用重启恢复；
- 项目包跨平台复制后，缺失全局创作方案时的安全降级；
- production build 与人工长任务验收。

## 13. 实施拆分

本设计跨越多个独立子系统，不用一个巨型提交实现。拆为以下计划：

### 计划 A1：应用级模型设置与统一运行端口

- App SQLite、系统路径和迁移；
- 全局连接、创作方案和旧项目兼容导入；
- `ModelRuntime`、`ModelTarget`、能力快照和回退策略；
- 现有 OpenAI/Anthropic Provider 适配；
- 设置 UI；
- 项目选择创作方案；
- 保留现有 P3 运行兼容。

退出门槛：打开任何项目都可使用全局 OpenAI/Anthropic 配置，项目包不保存 API Key。

### 计划 A2：Codex/Cursor 订阅运行时

- 官方运行时发现、版本门禁和进程 supervisor；
- Codex App Server JSON-RPC 适配；
- Cursor CLI Headless stream-json 适配；
- 账号状态、模型目录、能力测试和来源标识；
- 登录失效、额度耗尽、模型下架和回退；
- 隔离工作目录、织境不接触原始凭据和双平台进程测试；
- 设置 UI 中独立的“订阅连接”区域。

退出门槛：用户可在当前电脑登录自己的 Codex 或 Cursor 官方运行时，从软件设置中选择账号实际可用的模型完成一次受审计的创作步骤；小说软件不保存第三方 OAuth Token，也不调用私有接口。

### 计划 B：作品方案与 AI 创作驾驶舱

- 三套故事方向；
- 《作品方案》版本和作者确认；
- 正式资产原子应用；
- 全书章节路线图；
- AI-first 首页和驾驶舱。

退出门槛：一句创意可到确认作品方案与章节路线图。

### 计划 C：批量创作与长篇记忆 harness

- 1–50 章批次调度器；
- 自动多维审阅；
- 候选定稿队列；
- 影响分析与局部重写；
- 分层记忆、FTS5、`longform-v1`；
- 500 章金标与恢复测试。

退出门槛：可以在合成长篇项目中连续完成多批创作，早期事实可被后期准确检索。

### 计划 D：导入结构化重建与续写

- 持久化分章抽取；
- 跨章候选合并和冲突保留；
- 证据审阅；
- 形成作品方案和记忆快照；
- 接入同一批量续写运行时。

退出门槛：导入小说经作者确认后可从最后一章继续写作。

### 计划 E：AI 作者模板接口

- `AuthorProfile` / `AuthorTeam` 版本化 Schema；
- 本地安装、导出、选择和方案合成；
- 模板不包含密钥和作品数据；
- 在线市场只预留接口，不实现目录或交易。

退出门槛：本地模板可应用到创作方案并冻结进运行快照。

## 14. 架构门禁

1. 不删除或重写现有项目数据。
2. 不将候选稿或模型推断直接升级为作者事实。
3. 不让 React 直接访问 Provider、SQLite 或系统凭据。
4. 不把全书正文装入单次模型上下文。
5. 不为记忆检索提前引入 Rust、外部向量数据库或云端必需服务。
6. 不在一个迁移中同时搬迁全局设置、批次、记忆和导入数据。
7. 每个计划独立迁移、独立回滚测试、独立双平台门禁。
8. 不将 Cursor/Codex 订阅描述为通用 API，也不承诺订阅可使用所有模型。
9. 不复制第三方 OAuth Client ID、认证文件、私有端点或未公开协议。
10. 不允许订阅连接静默产生 API 费用、跨用户共享额度或绕过限流。

## 15. 外部接口依据与非依赖

实现前必须重新核对官方文档、分发许可和服务条款；以下页面是本方案当前采用的接口依据：

- OpenAI Codex Authentication：`https://learn.chatgpt.com/docs/auth`
- OpenAI Codex App Server：`https://learn.chatgpt.com/docs/app-server`
- OpenAI Codex SDK：`https://learn.chatgpt.com/docs/codex-sdk`
- Cursor CLI Headless：`https://docs.cursor.com/en/cli/headless`
- Cursor CLI Authentication：`https://docs.cursor.com/en/cli/reference/authentication`
- Cursor SDK：`https://cursor.com/changelog/sdk-release`
- Cursor Terms of Service：`https://cursor.com/en-US/terms-of-service`
- OpenAI Terms of Use：`https://openai.com/policies/terms-of-use/`

`can1357/oh-my-pi` 仅作为多供应商路由、能力目录和外部运行时协议设计的研究参考。它的 MIT 代码许可不等于 Cursor、OpenAI 或其他供应商后端的商业访问授权；本项目不依赖其二进制、认证存储或私有 Provider 实现。
