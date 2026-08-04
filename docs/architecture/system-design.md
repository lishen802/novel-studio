# 织境 Story Studio 系统技术设计

- 状态：当前唯一权威技术设计
- 适用版本：首个可商业交付的 Windows/macOS 桌面版本
- 对应文档：[产品需求](../product/requirements.md)、[产品与交互设计](../product/design.md)、[文档索引](../README.md)

## 1. 设计目标与约束

系统必须支持一部数十万至百万字中文长篇，从创意、方案、章节路线图到分批正文、审阅、定稿和导出，且在整个过程保持可恢复、可追溯、可控成本的连续创作。

已批准的生产技术基线为 Go、Wails v2、React、TypeScript、Vite 和 SQLite。浏览器原型 `prototype/web` 仅作为交互参考，不是生产运行时。Windows 与 macOS 都是一等目标；Wails v3 在稳定前不进入基线。Rust 只可为已测量的性能瓶颈、缺失的可靠 Go 库或明确更安全的原生能力，以独立 sidecar 和版本化 stdin/stdout 或本地 IPC 合约引入，禁止以 CGO/FFI 作为默认扩展方式。

系统采用本地优先和端口适配器架构：作品与正文存放在作者电脑，UI 不直接访问数据库或模型，模型能力可以来自 API 或用户已经登录的官方订阅运行时。

## 2. 目标架构

```text
React/TypeScript workbench
        │ Wails bindings + events（仅 DTO、命令和进度）
        ▼
Go application services / workflow orchestration
        ├── Domain: novel, assets, revisions, review, memory, jobs
        ├── Persistence ports: project repository, settings repository, search
        ├── Agent runtime: planning, drafting, reviewing, impact analysis
        ├── ModelRuntime ports: API / Codex subscription / Cursor subscription
        └── Platform ports: keychain, files, process, export, notifications
        ▼
SQLite project DB + project files   OS credential store   Official local runtimes
```

生产代码按责任分层，而不是按界面页面分层：

- `apps/desktop`：Wails 入口、应用生命周期、绑定注册和打包配置；
- `internal/domain`：作品、章节、事实、记忆、revision、审阅与状态迁移规则；
- `internal/application`：用例、事务边界、工作流编排和 DTO；
- `internal/persistence`：SQLite、迁移、全文检索和项目包读写；
- `internal/agent`：确定性上下文编排、任务计划、流式执行与可恢复作业；
- `internal/providers`：各模型运行时适配器；
- `internal/platform`：密钥库、文件系统、进程、系统通知与平台差异；
- React 侧仅通过 `StudioClient` 调 Wails 绑定和订阅事件，组件不直连 SQLite、文件路径或模型 Provider。

## 3. 本地数据边界与项目包

每部小说是独立项目包（扩展名可为 `.zjproject`），可移动、备份和恢复。项目包包含项目 SQLite 数据库、正文与导出文件、可再生成的搜索索引以及必要元数据；绝不包含 API Key、订阅令牌或操作系统身份信息。

应用级 `settings.db` 位于操作系统应用数据目录，只保存非敏感设置、模型连接元数据、作者人格/工作流模板与本地市场缓存。API Key 只存操作系统凭据库（macOS Keychain、Windows Credential Manager）；订阅身份由官方本地运行时自行持有。本应用只保存连接状态和授权能力，不复制 Cookie、OAuth token 或刷新令牌。

写入采用显式事务、追加式 revision 和原子文件替换。启动时先执行恢复检查：处理未完成作业、修复临时导出，并允许从最近一致快照恢复。正文和数据库备份可在项目级开启版本化快照。

## 4. 核心领域模型

### 4.1 作品与资产

`NovelProject` 持有标题、定位、阶段、目标篇幅、当前 canonical revision 和创建配置。`StoryAsset` 是带类型、状态、来源和 revision 的统一资产：作品方案、全书/卷/章大纲、人物、势力、地点、规则、关系、时间线、事实、伏笔、导入证据和作者人格应用记录。

资产状态严格区分：`candidate`、`proposed`、`confirmed`、`superseded`、`discarded`。AI 或导入不能直接写入 `confirmed`；作者确认或直接编辑生成的 revision 才能成为正式资产。

### 4.2 正文、候选与版本

`Chapter` 描述章节位置、路线图、批次与定稿状态。`ChapterRevision` 保存不可变正文 revision、作者/AI 来源、父 revision、生成上下文摘要、差异和审阅结论。`ChapterCandidate` 是尚未定稿的草稿，可并存多个候选。

“定稿为事实”由事务保证：定稿章节将指定 revision 标为 canonical，同时生成或更新事实、时间线、人物状态、已兑现/待兑现伏笔和章节摘要；后续生成只能读取 canonical 内容及被确认资产。候选永远不能覆盖定稿正文。

### 4.3 变更与作业

`ChangeSet` 记录作者意见、建议修改或直接替换所引起的意图、影响分析、允许修改的目标和确认结果。`AuthoringJob` 记录方案、路线图、生成、审阅、抽取、影响分析等可恢复任务，其状态为 `queued/running/paused/needs_confirmation/succeeded/failed/cancelled`，并带幂等键、输入快照和事件序列。

## 5. 持久化、检索与迁移

SQLite 使用 WAL、外键、事务和 FTS5。正文与 revision 以块化或文件引用方式存储时，元数据必须仍在数据库内可检索；全文索引由 canonical 正文、章节摘要、资产名称和证据片段构建。所有关联引用 revision ID、章节 ID 与证据区间，以支持审阅解释和导入溯源。

数据库演进继续使用嵌入式、版本化 Goose SQL 迁移（必要时配合受控 Go data migration），并记录 schema version 与迁移校验。**不改为 GORM 自动迁移。** 原因是本产品的项目数据库必须可审查、可测试、可回滚地随应用升级，且 FTS、索引、触发器、数据回填与桌面端旧项目兼容都需要显式控制。GORM 自动迁移对这些需求帮助有限，并可能掩盖破坏性 schema 变更；性能不是采用它的理由。运行时性能主要受索引、查询设计、批量事务、上下文检索和模型 IO 影响，而不是 ORM 与否。

迁移原则：迁移只前进；破坏性变更走 expand → backfill → switch → contract；升级前备份项目；迁移在临时副本验证后原子替换；必须有旧项目 fixture、升级测试和失败恢复测试。

## 6. AI 写作工作流状态机

```text
idea/imported
  → directions_pending_confirmation
  → proposal_pending_confirmation
  → roadmap_pending_confirmation
  → ready_to_write
  → batch_generating
  → batch_review
  → awaiting_finalization
  → ready_to_write | completed
```

作者可在任意节点暂停、编辑或提出意见。方向、完整作品方案、章节路线图、影响方案和每章定稿是硬确认点。正文批次大小为 1–50 章；批次中的章节完成“草稿 → 自动审阅/修订 → 作者审阅 → 定稿/替换/丢弃”后，才允许创建下一批。作业层按单章和单阶段拆分，便于断点续跑、取消和重试。

每个 AI 行为都先构造 `ContextManifest`，记录本次使用的 canonical 资产、章节摘要、人物/事实状态、时间窗口、相关伏笔、检索证据、作者指令、模型、提示词模板和预算。Manifest 固化进 job/revision，确保生成可解释、可复现到相近上下文，而不依赖隐式“聊天历史”。

## 7. 百万字长篇 Harness

模型不能将整本小说放进上下文。连续性由以下五层记忆共同保障：

| 层级 | 内容 | 用途 |
|---|---|---|
| 宪法层 | 已确认作品方案、世界规则、主线、结局边界、不可违背设定 | 每次规划与写作的稳定约束 |
| 结构层 | 卷/阶段目标、章节路线图、冲突推进、节奏与信息揭示 | 当前章在全书中的位置 |
| 状态层 | 人物关系、伤病/物品/能力、地点、时间线、未解问题 | 防止事实漂移 |
| 摘要层 | 每章/每卷 canonical 摘要、近期章节详细摘要 | 保持局部叙事连贯 |
| 证据层 | 原文片段、导入证据、审阅发现、伏笔登记 | 回答“依据是什么” |

生成前，检索器以当前章节目标、出场实体、冲突、未兑现伏笔和作者指令建立最小充分上下文，而非按时间无限拼接。生成后，独立的抽取/审阅模型更新候选事实与摘要；只有作者定稿后才合入 canonical memory。长篇越过设定阈值（例如百万字）后，影响分析默认只建议修改后续未定稿内容；对既有定稿内容仅报告冲突与风险，除非作者明确选择回溯修改。

审阅至少包括：设定与事实一致性、人物动机/弧光、时间线、伏笔、情节推进、节奏、重复与文风、敏感内容与引用风险。不同步骤可选择不同模型，防止同一模型自我审阅产生盲点。

## 8. 影响分析与修订

作者意见、手工替换和 AI 建议先创建 ChangeSet。影响分析器将目标分为：立即重写的未定稿候选、需要更新的后续路线图/记忆、仅提示风险的已定稿历史，以及不受影响项。它必须先给作者展示范围、理由、代价和建议方案；确认后才执行。

重写产生新候选/revision，不删除旧版本。任何失败、取消或模型中断只改变作业状态，不能破坏 canonical 内容。差异、被引用证据和每条审阅结论可从章节界面回看。

## 9. 导入续写与参考材料

导入服务支持 TXT、Markdown 和后续扩展格式，采用分段、章节识别、抽取任务和证据定位。它产出大纲、人物、世界观、关系、时间线、事实和未解线索的候选资产；作者确认、编辑或舍弃后，才能作为续写的 canonical memory。

参考材料服务只接受作者合法提供的短篇完整作品、少量摘录、简介或公开介绍链接。网页获取必须展示来源、抓取内容和版权提示。分析器输出题材、读者期待、叙事节奏、冲突模式和风格禁区等高层灵感约束；不得复用专名、角色关系、关键情节链、段落或可识别文风。相似性/引用风险审阅是生成前后都可启用的检查。

## 10. 统一模型运行时

应用层只依赖 `ModelRuntime`，以能力而非厂商名称选择模型：列出可用模型、检查能力、创建流式生成、取消、估算成本/预算、返回用量和标准化错误。每次执行记录 `runtime kind`、模型 ID、模型来源和计费语义。

| Runtime | 身份与凭据 | 初始适配方式 | 约束 |
|---|---|---|---|
| API | 应用在 OS 凭据库引用 Key | OpenAI、Anthropic；后续兼容 API | 显示预计与实际 API 成本 |
| Codex subscription | 官方 Codex 本地 App Server/SDK 已登录会话 | 受支持的 JSON-RPC/流式接口 | 只暴露该订阅授权的 OpenAI/Codex 能力 |
| Cursor subscription | 用户安装且登录的官方 Cursor CLI/SDK | 官方 headless/stream-json 合约 | 只暴露 Cursor 授权并实际返回的模型 |

订阅连接的发现、健康检查、模型列表、权限失败和版本兼容均经本地适配器处理。不得模仿第三方工具的内部 OAuth、私有端点或令牌读取；不得把“已绑定 Cursor/Codex”宣传为可使用所有厂商模型。模型名必须连同来源显示，例如“Claude（经 Cursor 订阅）”或“Claude（Anthropic API）”。

无论写作、规划或审阅，用户都可在任务启动处选择连接/模型，也可通过对话说明意图；精确预算、批次数、温度等放入可展开的任务选项。连接配置仅在应用设置中维护，不放入小说工作区。默认不静默回退到另一个 API 或付费连接；需要回退时向作者说明目标、费用与原因并征得确认。

## 11. 作者人格、工作流模板与未来市场

V1 定义本地 `AuthorProfile` 与 `AuthorTeam`：题材偏好、叙事视角、节奏、禁区、提示词片段、推荐任务链和可用模型要求。它们是可版本化、可导入导出的本地模板，应用记录写入 ContextManifest。

未来“作者市场”以签名模板包、元数据、兼容性、版本、权限和推荐模型为边界扩展。市场、支付、云同步、账户和远程执行不属于当前交付；当前数据模型与 UI 只预留本地模板列表、来源和版本字段，避免锁死扩展路径。

## 12. 前端、绑定与事件

Wails binding 提供命令式用例：创建/打开项目、读取工作台状态、保存作者编辑、确认资产、启动/暂停/取消作业、提交反馈、确认 ChangeSet、配置连接、导入与导出。大文本流和作业状态通过事件或可恢复的 job event feed 推送；前端重新连接后按 job cursor 补拉。

前端维护三类状态：服务器权威状态（项目/资产/job）、局部未保存编辑状态、展示状态（筛选/面板/滚动）。保存冲突必须显示基于 revision 的差异，不能后写覆盖。表单仅服务于需要精确数值/范围的设置；叙事意图主要由对话输入和结构化候选卡协同表达。

## 13. 安全、隐私、成本与可靠性

- 不记录 API Key、订阅令牌、完整敏感提示词到普通日志；诊断日志可脱敏导出。
- 提供网络访问和外部运行时调用的清晰提示，链接抓取需单独确认。
- 每个 job 设置模型、最大输出、预算上限、取消令牌和超时；成本不可估算时明确标示。
- Provider/订阅运行时错误映射为可操作状态：未安装、未登录、版本不兼容、无权限、额度/限流、网络失败、内容拒绝和上下文超限。
- 长任务在应用退出、休眠、更新或崩溃后可恢复或安全终止；不得把半流式文本标为正式 revision。
- 对 project DB 执行完整性检查、备份与恢复；导出失败不能影响项目内容。

## 14. 测试与质量门槛

领域状态迁移、canonical 规则、影响分析、上下文 Manifest、迁移和导入解析必须有 Go 单元与集成测试。Provider runtime 使用契约测试和录制的流式 fixture，不能用真实用户订阅作为 CI 前提。桌面端使用 Wails binding 测试、React 组件/交互测试，以及 macOS/Windows 打包与关键流程冒烟测试。

验收至少覆盖：从一句创意到确认作品方案；分批生成被正确阻断；作者编辑/替换和 ChangeSet；导入抽取的证据可回看；百万字模拟数据下的检索与上下文上限；API/Codex/Cursor 三种连接的可用性与错误呈现；凭据不进入项目包、日志和导出；中断恢复、升级迁移和项目包可移植性。

性能指标以可测量目标管理：常规项目打开和章节切换应保持交互级响应；检索必须受候选数量和 token 预算限制；批量任务可串行恢复并避免长事务锁住编辑；对于测得的瓶颈先优化 SQL/索引、摘要和并发策略，再考虑引入独立 Rust sidecar。

## 15. 现有基线与交付顺序

当前仓库已有基础桌面项目和 P0–P4 的部分能力：正文/修订与恢复、结构化故事资产、变更集与快照、OpenAI/Anthropic Provider 与 Agent Run、以及确定性导入基础。它们是迁移基线，不等同于完整产品已经实现。

后续实现按可验收纵切推进：先统一项目状态机、canonical/candidate 与批次写作闭环；再接入 ContextManifest、长期记忆和审阅；随后完成导入确认与影响分析；最后接入官方订阅运行时、作者模板和导出/跨平台硬化。每一切片都必须更新测试和迁移，而不是依赖已经删除的旧实施计划。
