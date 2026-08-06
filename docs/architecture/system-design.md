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

项目包内部只使用固定短目录名和稳定 ID 作为文件名；卷名、章名、候选标题和作者输入只保存在数据库元数据中，不用于形成深层文件路径。Windows 验证必须覆盖未启用系统长路径选项的默认环境。

路径预算按“用户选择的项目根目录 + 最长内部相对路径”计算。短 ID 只降低内部增长，不能替代 Windows 长路径兼容、创建前预检和路径过长的可操作错误。附件、导入档案、快照、分支与调用工件使用固定短目录和 UUID/内容哈希文件名，不把卷名、章名、候选标题或 Prompt 片段写入物理路径。

可编辑项目在整个打开生命周期内持有项目级 OS 文件锁，锁目标为包根下固定短名的 lease 文件；元数据可记录 PID、主机、应用版本、项目 ID 和开始时间，但锁是否有效以操作系统锁为准，不能仅凭 PID 或时间戳删除。`activeWorkspace` 持有 lease，只有所有该项目作业 lease 已取消或结束、数据库已关闭后才释放。第二实例无法取得锁时只能重试、只读打开或创建恢复副本，不得并发写入。

打开可编辑项目时执行包目录可写、SQLite 短写入/回滚、WAL/SHM、checkpoint 和同目录原子创建/替换探针；探针不能修改 canonical 内容。同步盘和网络映射目录只触发风险提示，不按路径名称直接阻断，因为锁可能在打开后才发生。运行期仍必须分类处理 sharing violation、busy、readonly、disk full、checkpoint 和原子 rename 失败，并在可能时先保存恢复草稿。

只读打开不取得写入 lease，但必须使用不会触发迁移、恢复、checkpoint、索引重建或后台作业的只读 store；UI 禁用保存、定稿、抽取、模型生成、瘦身和版本切换。项目需要迁移或恢复时，只读模式仅报告原因，作者必须取得可编辑 lease 或先创建恢复副本。

## 4. 核心领域模型

### 4.1 作品与资产

`NovelProject` 持有标题、定位、阶段、目标篇幅、当前 canonical revision 和创建配置。`StoryAsset` 是带类型、状态、来源和 revision 的统一资产：作品方案、全书/卷/章大纲、人物、势力、地点、规则、关系、时间线、事实、伏笔、导入证据和作者人格应用记录。

资产状态严格区分：`candidate`、`proposed`、`confirmed`、`superseded`、`discarded`。AI 或导入不能直接写入 `confirmed`；作者确认或直接编辑生成的 revision 才能成为正式资产。

### 4.2 正文、候选与版本

`Chapter` 描述章节位置、路线图、批次与定稿状态。`ChapterRevision` 保存不可变正文 revision、作者/AI 来源、父 revision、生成上下文摘要、差异和审阅结论。`ChapterCandidate` 是尚未定稿的草稿，可并存多个候选。

“定稿为事实”由事务保证：定稿前的模型抽取先产生带证据的事实与记忆候选，数据库事务再原子提交指定 canonical revision、已经验证的派生记忆、审阅摘要和版本信息。模型调用不得运行在数据库事务中；若记忆抽取失败，正文仍可安全定稿，并记录 `memory_pending_rebuild` 供幂等任务重建。后续生成只能读取 canonical 内容及被确认资产，候选永远不能覆盖定稿正文。

### 4.3 变更与作业

`ChangeSet` 记录作者意见、建议修改或直接替换所引起的意图、影响分析、允许修改的目标和确认结果。`AuthoringJob` 记录方案、路线图、生成、审阅、抽取、影响分析等可恢复任务，其状态为 `queued/running/paused/needs_confirmation/succeeded/failed/cancelled`，并带幂等键、输入快照和事件序列。

`JobEvent` 是 append-only 的作业事件源，至少包含 `job_id`、单调递增的 `sequence_number`、事件类型、轻量 payload 或 artifact 引用、发生时间和关联步骤。`(job_id, sequence_number)` 唯一；作业状态/步骤变化与对应事件必须在同一短事务中提交。Wails 推送不是事件事实来源，只发送 `job_id + latest_sequence` 的唤醒通知；流式大文本进入可恢复 artifact，不在通知中复制完整正文。

`JobSnapshot` 是从作业、步骤和累计用量投影出的可重建权威快照，记录最大 sequence、最早可补拉 sequence、待确认动作和终态信息。它用于首次挂载、事件清理后的重建和 API 分页边界，不替代保留期内的 `JobEvent` 审计记录。

`Anchor` 绑定路线图节点、章节或章节范围，类型为 `hard/soft/prohibited`，并记录作者来源、锁定状态和版本。生成任务冻结相关锚点到 ContextManifest，审阅结果记录每项锚点的 `satisfied/deviated/not_covered` 状态。

`BatchCheckpoint` 记录批次内的安全边界、canonical head、已完成候选/revision、provisional 状态摘要、锚点评估、预算和检查结果。`GenerationAttempt` 记录从某个检查点或未定稿章节开始的一次生成尝试及其父 attempt；回退不会删除旧 attempt，而是将回退点之后的旧候选标记为 `superseded`，再创建使用新 Context Manifest 的子 attempt。

高层设定变更使用 `ConstraintChangeIntent` 区分 `future_only/in_story_change/retcon`，并记录生效章节或事件。影响分析必须以旧约束 revision、新候选 revision、变更意图和 canonical head 为输入，避免把故事内正常演变误判为追溯冲突。

## 5. 持久化、检索与迁移

SQLite 使用 WAL、外键、事务和 FTS5。正文与 revision 以块化或文件引用方式存储时，元数据必须仍在数据库内可检索；全文索引由 canonical 正文、章节摘要、资产名称和证据片段构建。所有关联引用 revision ID、章节 ID 与证据区间，以支持审阅解释和导入溯源。

SQLite 首版连接基线保持 `busy_timeout = 5000ms` 和受控连接数；当前单连接策略已同时串行化读写，不再额外强制叠加 Single-Writer Channel。写入事务必须短小、可取消并在需要抢占写锁的多步流程中使用 `BEGIN IMMEDIATE` 与有界重试。模型、网络、文件选择和等待作者确认不得位于数据库事务内。若基准测试证明单连接阻塞读取，再评估独立写入协调器与只读连接池；连接池扩大后，WAL、foreign key、busy timeout 等连接级 PRAGMA 必须对每条连接生效。

应用层互斥同样不得跨越 Model IO。Agent 启动时只在短锁内确认 active project、冻结 project/store generation 并取得 job lease，随后释放工作区锁再组装上下文和调用 Provider。每个步骤通过短事务保存状态、用量、工件和事件。切换、关闭或恢复项目时先拒绝新作业，再取消或等待已有 job lease，最后关闭 store；不得让长时间模型调用阻塞正文保存，也不得在 store 已关闭后继续回写。

Context 数据分为两层：

- `ContextManifest`：长期保存的轻量结构化索引，记录不可变来源 ID/revision、引用范围、装配顺序、权重和原因，锚点、模板与装配器版本，模型参数、预算、用量、耗时、遗漏原因、Provider request ID 和最终内容哈希；
- `InvocationArtifact`：可选的完整组装 Prompt、Provider 请求体、流式事件和调试输出，采用内容寻址、去重和压缩 blob 存储，并通过可配置的时间/数量/空间策略保留。

Manifest 不复制已由不可变 revision 保存的完整正文。调用工件清理后，系统仍应根据 Manifest、来源范围、模板/装配器版本和哈希解释并校验上下文组成；不承诺在 Provider 或模板运行环境变化后逐字重放远期请求。

项目瘦身先创建恢复记录，在独立事务中更新工件引用和保留状态，再回收无引用 blob 与可再生成索引。中断恢复根据已提交的引用表完成或撤销回收；不得触碰 canonical 正文、候选/revision、作者指令、正式资产、必要证据、快照或分支头。

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

作者可在任意节点暂停、编辑或提出意见。方向、完整作品方案、章节路线图、影响方案和每章定稿是硬确认点。正文批次大小为 1–50 章，默认 3–10 章；批次中的章节完成“草稿 → 自动审阅/修订 → 作者审阅 → 定稿/替换/丢弃”后，才允许创建下一批。作业层按单章和单阶段拆分，便于断点续跑、取消和重试。

批次编排器在默认每 3–5 章的边界插入 `BatchCheckpoint`。检查点只读取当前批次 provisional 候选和启动时冻结的 canonical head，输出主线偏离、硬锚点、锁定设定、人物状态、世界规则、时间线、严重审阅项和剩余预算。出现阻断条件时，作业转为 `needs_confirmation`，并在持久化事件中记录原因、已完成步骤和允许动作；在作者确认前不得调度后续模型调用。

熔断后的状态迁移为：

```text
needs_confirmation
  → intervene_and_resume → checkpoint_recheck → running
  → rewind_requested → impact_confirmed → new_attempt → running
  → accept_risk → running
  → stop_batch → batch_review
```

`rewind_requested` 只允许选择未定稿候选。确认后，应用在事务中标记受影响候选/审阅为 `superseded`、记录父子 attempt 和回退原因，再用更新后的路线图、锚点、作者修改与当前 canonical head 构造新 Manifest。若 canonical head 已改变或选择范围包含定稿章节，原回退方案失效并转入历史修订影响分析。

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

近场叙事装配优先保留上一章结束状态、当前场景、视角、情绪、未完成动作和必要正文片段；是否加入前 1–3 章全文由任务 Token 预算和相关性决定，不作为固定规则。远场证据必须指向 canonical revision，过期人物状态和已失效事实不得与当前状态等价排序。

长篇 Harness 使用 `fixtures/longform/novel_500ch` 下的确定性生成规格、固定种子和黄金事实/查询清单，在测试临时目录生成 500 章合成项目，不提交大体积或有版权风险的真实小说正文。黄金清单不少于 100 个查询，分别报告硬约束/当前状态/重要事实 `Recall@10`、未兑现伏笔/跨卷承诺 `Recall@10`、证据支持率和过期状态误召回率。阈值以产品需求 9.2 为发布标准；第 1 章承诺进入第 500 章 Context Manifest 是端到端哨兵用例，不替代总体指标。

Harness 分别记录冷启动、热缓存、Context Token/字符占用、查询候选数、数据库/索引体积、项目包体积和 Invocation Artifact 清理前后体积。检索与 Context 装配 Benchmark 不调用模型；fake/Recorded Provider 只用于完整 Agent 流水线、错误、取消、恢复和 JobEvent 协议测试。每个 PR 运行 50–100 章缩小正确性集和确定性预算检查，Nightly/Release 运行完整 500 章；墙钟性能硬门槛只在固定 Windows/macOS 参考 Runner 上判断，共享 Runner 仅用于正确性、分配量、Token 和体积回归。

审阅至少包括：设定与事实一致性、人物动机/弧光、时间线、伏笔、情节推进、节奏、重复与文风、敏感内容与引用风险。不同步骤可选择不同模型，防止同一模型自我审阅产生盲点。

## 8. 影响分析与修订

作者意见、手工替换和 AI 建议先创建 ChangeSet。影响分析器将目标分为：立即重写的未定稿候选、需要更新的后续路线图/记忆、仅提示风险的已定稿历史，以及不受影响项。它必须先给作者展示范围、理由、代价和建议方案；确认后才执行。

重写产生新候选/revision，不删除旧版本。任何失败、取消或模型中断只改变作业状态，不能破坏 canonical 内容。差异、被引用证据和每条审阅结论可从章节界面回看。

解锁硬约束或高层方案后，影响分析按以下顺序执行：

1. 根据 `ConstraintChangeIntent` 确定生效边界；`future_only` 不扫描边界前内容为冲突，`in_story_change` 验证触发事件和前后状态，`retcon` 才将新约束应用于全部历史范围；
2. 通过资产引用、事实生效区间、时间线、摘要、锚点、伏笔、人物弧和待执行作业建立受影响候选集；
3. 对候选集检索 canonical 原文证据，只有结构化信息无法判定时才调用模型；
4. 将结果分为历史定稿冲突、正式资产/派生记忆调整、待解决未定稿候选、路线图/锚点调整和未检查范围；
5. 为未定稿项提供重写、调整路线图、保留为例外、废弃或创建分支，不把“受影响”自动转换为重写；
6. 对 `retcon` 或跨多卷变更创建一致性快照并建议剧情分支。

影响方案冻结 canonical head、扫描覆盖、检索查询、模型调用和预计费用。任何相关 revision、资产或作业状态变化都会使方案失效；历史定稿始终只生成报告或显式修订候选。

## 9. 导入续写与参考材料

导入采用三阶段流水线：

1. `ingest`：本地安全归档、编码识别、重复检测、确定性切分卷章、原文行范围和基础全文索引，不调用模型；完成后正文即可阅读和编辑。
2. `continuation preparation`：按近期章节、当前卷和作者选择范围创建章节级抽取步骤，优先形成续写所需的人物当前状态、硬规则和未解线索候选。
3. `background enrichment`：其余授权章节在后台继续抽取，支持优先级调整、暂停、断点恢复和仅重试失败/未完成步骤。

导入分析状态为 `imported/partially_analyzed/continuation_ready/fully_analyzed/needs_review`。每个步骤冻结来源章节/revision 或导入哈希、行范围、模型、Schema、预算与 Context Manifest。已完成步骤立即持久化，后续步骤失败不能使其失效；重试默认复用输入未变化的完成步骤。

跨章合并不得使用简单的“首次值获胜”或“末次值获胜”。实体匹配先按稳定 ID、规范化名称和别名建立候选簇；字段一致时合并证据，字段冲突时保留多个带来源的值并生成 `needs_review` 候选。关系、事件、时间线和伏笔只有在引用目标可解析时才能进入可应用 ChangeSet。

抽取结果产出大纲、人物、世界观、关系、时间线、事实和未解线索的候选资产。证据必须包含精确原文片段及章节/revision 或导入行范围；作者确认、编辑或舍弃后，才能作为续写的 canonical memory。达到最低续写条件后可在覆盖范围提示下继续写作，其余后台结果仍只能进入候选队列。

参考材料服务只接受作者合法提供的短篇完整作品、少量摘录、简介或公开介绍链接。网页获取必须展示来源、抓取内容和版权提示。分析器输出题材、读者期待、叙事节奏、冲突模式和风格禁区等高层灵感约束；不得复用专名、角色关系、关键情节链、段落或可识别文风。相似性/引用风险审阅是生成前后都可启用的检查。

## 10. 统一模型运行时

应用层只依赖 `ModelRuntime`，以能力而非厂商名称选择模型：列出可用模型、检查能力、创建流式生成、取消、估算成本/预算、返回用量和标准化错误。每次执行记录 `runtime kind`、模型 ID、模型来源和计费语义。

| Runtime | 身份与凭据 | 初始适配方式 | 约束 |
|---|---|---|---|
| API | 应用在 OS 凭据库引用 Key | OpenAI、Anthropic；后续兼容 API | 商业发布基线，显示预计与实际 API 成本 |
| Codex subscription | 官方 Codex 本地 App Server/SDK 已登录会话 | 仅使用官方公开支持的本地协议 | 满足协议、许可、兼容性和契约测试门槛后发布 |
| Cursor subscription | 用户安装且登录的官方 Cursor CLI/SDK | 仅使用官方公开支持的 headless/stream-json 合约 | 满足协议、许可、兼容性和契约测试门槛后发布 |

订阅连接的发现、健康检查、模型列表、权限失败和版本兼容均经本地适配器处理。正式发布要求存在可引用的官方调用契约、明确允许的调用方式、受支持版本矩阵、契约 fixture 和安全禁用路径；任一门槛不满足时，连接器不得出现在稳定功能中。不得模仿第三方工具的内部 OAuth、私有端点或令牌读取；不得把“已绑定 Cursor/Codex”宣传为可使用所有厂商模型。模型名必须连同来源显示，例如“Claude（经 Cursor 订阅）”或“Claude（Anthropic API）”。

无论写作、规划或审阅，用户都可在任务启动处选择连接/模型，也可通过对话说明意图；精确预算、批次数、温度等放入可展开的任务选项。连接配置仅在应用设置中维护，不放入小说工作区。默认不静默回退到另一个 API 或付费连接；默认授权仅覆盖单次任务，项目级回退授权必须绑定来源连接、目标连接、金额/Token 上限和过期时间，不提供永久无限额授权。

## 11. 作者人格、工作流模板与未来市场

V1 定义本地 `AuthorProfile` 与 `AuthorTeam`：题材偏好、叙事视角、节奏、禁区、提示词片段、推荐任务链和可用模型要求。它们是可版本化、可导入导出的本地模板，应用记录写入 ContextManifest。

未来“作者市场”以签名模板包、元数据、兼容性、版本、权限和推荐模型为边界扩展。市场、支付、云同步、账户和远程执行不属于当前交付；当前数据模型与 UI 只预留本地模板列表、来源和版本字段，避免锁死扩展路径。

## 12. 前端、绑定与事件

Wails binding 提供命令式用例：创建/打开项目、读取工作台状态、保存作者编辑、确认资产、启动/暂停/取消作业、提交反馈、确认 ChangeSet、配置连接、导入与导出。启动长作业先持久化并返回 `job_id`，后台执行不依赖调用页面继续存活。

大文本流和作业状态使用可恢复的 job event feed。Go 端至少提供：

- `GetJobSnapshot(jobID)`：返回权威状态、步骤、累计用量、待确认动作和当前最大 sequence；
- `GetJobEvents(jobID, afterSequence, limit)`：按 sequence 升序增量返回事件、`next_cursor` 和 `has_more`；
- `ListRecoverableJobs(projectID)`：供应用打开或 Bridge 重挂载时发现非终态及待确认作业。

Wails Event 只通知“某 job 有更新”，允许丢失、合并或重复。React 收到通知、重新 Mount、HMR、窗口从休眠恢复或应用重新打开时，都从本地最后已应用 cursor 调用 `GetJobEvents`，直到追上 snapshot 的最大 sequence。前端 reducer 按 `(job_id, sequence_number)` 幂等应用，检测缺口时停止应用后续事件并补拉；不得根据事件到达次数累计 Token、费用或章节数。

cursor 只是前端缓存，不是事实来源。事件清理前先把终态、累计量、待确认动作和最新 sequence 折叠进 `JobSnapshot`；客户端 cursor 早于可用事件下界时，服务返回 `snapshot_required`，前端以 snapshot 重建后从新的 cursor 继续。终态 JobEvent 至少保留到恢复窗口结束，流式正文和大 Prompt 的保留遵循 Invocation Artifact 策略。

前端维护三类状态：服务器权威状态（项目/资产/job）、局部未保存编辑状态、展示状态（筛选/面板/滚动）。保存冲突必须显示基于 revision 的差异，不能后写覆盖。表单仅服务于需要精确数值/范围的设置；叙事意图主要由对话输入和结构化候选卡协同表达。

## 13. 安全、隐私、成本与可靠性

- 不记录 API Key、订阅令牌、完整敏感提示词到普通日志；诊断日志可脱敏导出。
- 提供网络访问和外部运行时调用的清晰提示，链接抓取需单独确认。
- 每个 job 设置模型、最大输出、预算上限、取消令牌和超时；成本不可估算时明确标示。
- Provider/订阅运行时错误映射为可操作状态：未安装、未登录、版本不兼容、无权限、额度/限流、网络失败、内容拒绝和上下文超限。
- 内容拒绝与网络、额度错误使用不同的稳定错误类型；失败不得改变作者输入、已有候选或 canonical 内容。切换到政策不同的 Provider 前必须重新确认来源、成本和内容边界。
- 长任务在应用退出、休眠、更新或崩溃后可恢复或安全终止；不得把半流式文本标为正式 revision。
- 对 project DB 执行完整性检查、备份与恢复；导出失败不能影响项目内容。
- 项目级 OS lease、SQLite 探针和运行期文件错误共同保护可编辑项目；启动探针成功不等于后续同步盘操作必然安全。

## 14. 测试与质量门槛

领域状态迁移、canonical 规则、影响分析、上下文 Manifest、Invocation Artifact 保留/瘦身、批次检查点、生成 attempt、锚点、迁移和渐进导入必须有 Go 单元与集成测试。Provider runtime 使用契约测试和录制的流式 fixture，不能用真实用户订阅作为 CI 前提。桌面端使用 Wails binding 测试、React 组件/交互测试，以及 macOS/Windows 打包与关键流程冒烟测试。

JobEvent 测试必须覆盖漏推后补拉、重复通知、sequence 缺口、Bridge 重挂载、应用重启、cursor 早于清理下界和 snapshot 重建。并发测试必须用可阻塞 fake Provider 证明 Model IO 期间正文仍可保存，并验证项目切换/关闭会取消或等待 job lease，不会向已关闭 store 回写。SQLite 测试覆盖 WAL、每连接 PRAGMA、有界 busy 重试、事务原子性和锁等待取消。

项目包测试在 Windows/macOS 原生 Runner 上覆盖第二实例争锁、锁元数据残留但 OS 锁已释放、只读打开、恢复副本、长根路径、保留名、同步目录模拟 sharing violation、WAL/SHM、checkpoint 和原子替换失败。不得通过删除有效锁或关闭完整性检查让测试通过。

验收至少覆盖：从一句创意到确认作品方案；硬锚点进入 Manifest；长批次在检查点被正确阻断；从未定稿位置回退后旧候选保留为 superseded、新 attempt 使用更新 Manifest、canonical 不变；三类硬约束变更意图得到不同影响范围且历史定稿不被改写；作者编辑/替换和 ChangeSet；快速导入不等待模型、部分抽取可审阅、冲突证据可回看、断点重试不重复完成步骤；Bridge 重挂载后 JobEvent 增量补拉与幂等恢复；Model IO 期间正文保存；第二实例项目写锁与恢复路径；分层合成 Harness 下的量化召回、上下文上限、体积和参考硬件性能；数千次调用后的工件去重、压缩、清理和中断恢复；API 连接和满足发布门槛的订阅连接之可用性与错误呈现；凭据不进入项目包、日志和导出；中断恢复、升级迁移、默认 Windows 路径环境和项目包可移植性。

性能指标以可测量目标管理：在 Windows/macOS 参考硬件上分别记录冷启动和热缓存；用固定 fixture 比较 FTS5、结构化索引和可选本地向量索引，外部向量服务不得成为发布前提。检索必须受候选数量和 token 预算限制；项目打开和正文编辑不能全量加载历史 Invocation Artifact；批量任务可串行恢复并避免长事务锁住编辑；对于测得的瓶颈先优化 SQL/索引、摘要和并发策略，再考虑引入独立 Rust sidecar。

自动化门槛之外，商业发布前进行不少于 8 名目标作者的封闭测试，其中至少 3 名有 50 章以上长篇经验。测试记录首次正文时间、候选接受/重写率、批次完成率、恢复成功率、费用理解和连续性问题；可能损坏作品、误导收费或阻断核心闭环的问题必须在发布前关闭。

## 15. 现有基线与交付顺序

当前仓库已有基础桌面项目和 P0–P4 的部分能力：正文/修订与恢复、结构化故事资产、变更集与快照、OpenAI/Anthropic Provider 与 Agent Run、以及确定性导入基础。SQLite 已启用 WAL、5 秒 busy timeout 和单连接基线；可恢复 JobEvent feed、Model IO 与工作区锁隔离、项目级 OS lease、真正的只读 store 和分层长篇 Harness 尚未完成。它们是迁移基线，不等同于完整产品已经实现。

后续实现按可验收纵切推进：

1. 商业发布底线：统一项目状态机、canonical/candidate、默认 3–10 章批次闭环、锚点、检查点、API 运行时、可恢复 JobEvent feed、Model IO 与编辑锁隔离、项目级写入 lease、基本项目恢复，以及现有快速导入与作者选择范围的抽取确认；
2. 连续性强化：ContextManifest、长期记忆、量化 Harness、影响分析和渐进导入的断点/冲突/覆盖管理；
3. 完整 V1：最多 50 章高级批次、全书后台深化、满足发布门槛的官方订阅运行时、作者模板、剧情分支和跨平台硬化。

每一切片都必须更新测试和迁移。已有导入与基础资产抽取作为实现基线保留，但不能把当前完成度等同于全书渐进抽取、冲突保留和续写准备已经满足。
