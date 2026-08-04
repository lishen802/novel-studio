# AI 主创作与百万字长篇运行时技术方案

- 日期：2026-08-04
- 状态：待用户技术评审
- 产品规格：`docs/superpowers/specs/2026-08-04-ai-first-long-form-authoring-design.md`
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

新增能力分为五个边界：

1. 应用级模型设置；
2. 作品方案与全书章节路线图；
3. 有界批量创作与自动审阅；
4. 带证据的长篇记忆 harness；
5. 导入小说结构化重建与续写接入。

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

### 2.4 导入只完成确定性阶段

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
├── Provider adapters
├── Keychain / Credential Manager
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
creative_profiles
creative_profile_roles
installed_author_profiles
installed_author_teams
legacy_connection_imports
app_schema_migrations
```

`model_connections` 只保存供应商、Base URL、显示名称和 credential reference。API Key 继续只存在系统凭据仓库。

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

### 4.3 兼容迁移

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
AppModelSettings / SaveModelConnection / TestModelConnection
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
- 模型响应失败只使当前步骤失败，不删除已完成 Artifact。
- 批次运行期间阻止分支切换、快照恢复、导入应用和项目关闭数据库替换；关闭项目先取消并等待运行释放读锁。
- 作者编辑章节导致基线变化时，候选应用使用 revision 和 canonical head 双重校验。
- 磁盘不足时停止新模型调用并保留可恢复状态。
- 任何安全错误摘要不得包含正文、Prompt、模型响应、SQL 或 API Key。

## 12. 测试与可证明性

### 12.1 领域和应用测试

- 状态机、批次 1–50 边界和单活动批次；
- 批次顺序执行、取消、恢复、独立重试；
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
- 长任务休眠、网络切换和应用重启恢复；
- 项目包跨平台复制后，缺失全局创作方案时的安全降级；
- production build 与人工长任务验收。

## 13. 实施拆分

本设计跨越多个独立子系统，不用一个巨型提交实现。拆为以下计划：

### 计划 A：应用级模型设置

- App SQLite、系统路径和迁移；
- 全局连接、创作方案和旧项目兼容导入；
- 设置 UI；
- 项目选择创作方案；
- 保留现有 P3 运行兼容。

退出门槛：打开任何项目都可使用全局 OpenAI/Anthropic 配置，项目包不保存 API Key。

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
